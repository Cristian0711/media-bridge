package download

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type QueuePayload struct {
	RequestEntryID uint   `json:"request_entry_id"`
	RequestID      string `json:"request_id"`
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
}

// RequestLinker lets the download processor write the media_id back onto
// the originating request row after the media is created, so a later remove
// flow can find the request via the media_id.
type RequestLinker interface {
	UpdateMediaID(ctx context.Context, requestID uint, mediaID uint) error
}

// RequestStatusUpdater updates download request rows when the download queue gives up.
type RequestStatusUpdater interface {
	MarkDownloadingIfQueued(ctx context.Context, requestID uint) (bool, error)
	MarkFailedIfInFlight(ctx context.Context, requestID uint) (bool, error)
}

// HardlinkEnqueuer enqueues and inspects hardlink_processing_queue jobs.
type HardlinkEnqueuer interface {
	Enqueue(ctx context.Context, payload hardlink.QueuePayload) error
	HasActiveJobForMediaID(ctx context.Context, mediaID uint) (bool, error)
}

// MediaForDownload is the subset of media.Service used by the download worker.
type MediaForDownload interface {
	CreateFromRequest(ctx context.Context, input media.CreateFromRequestInput) (uint, error)
	FindExistingDownloadMediaID(ctx context.Context, input media.CreateFromRequestInput) (uint, error)
	GetMediaByID(ctx context.Context, id uint) (*media.Media, error)
}

type Processor struct {
	queue         *processingqueue.Queue[QueuePayload]
	requestSource interface {
		FindByID(ctx context.Context, id uint) (*RequestDetails, error)
	}
	downloadService   Service
	mediaService      MediaForDownload
	hardlinkProcessor HardlinkEnqueuer
	requestLinker     RequestLinker
	requestStatus     RequestStatusUpdater
}

func NewProcessor(
	databaseURL string,
	requestSource interface {
		FindByID(ctx context.Context, id uint) (*RequestDetails, error)
	},
	downloadService Service,
	mediaService MediaForDownload,
	hardlinkProcessor HardlinkEnqueuer,
	requestLinker RequestLinker,
	requestStatus RequestStatusUpdater,
) (*Processor, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	q, err := processingqueue.New[QueuePayload](pool, "download_processing_queue",
		processingqueue.StandardQueueOptions()...,
	)
	if err != nil {
		return nil, err
	}
	if err := q.EnsureTable(context.Background()); err != nil {
		return nil, err
	}
	return &Processor{
		queue:             q,
		requestSource:     requestSource,
		downloadService:   downloadService,
		mediaService:      mediaService,
		hardlinkProcessor: hardlinkProcessor,
		requestLinker:     requestLinker,
		requestStatus:     requestStatus,
	}, nil
}

func (p *Processor) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	log := logger.Named("download.queue.worker")
	handler := func(ctx context.Context, job *processingqueue.Job[QueuePayload]) error {
		req, err := p.requestSource.FindByID(ctx, job.Payload.RequestEntryID)
		if err != nil {
			return err
		}
		switch req.Type {
		case "movie_download", "show_download":
			err := p.processDownload(ctx, log, job, req)
			if err == nil {
				return nil
			}
			permanent := errors.Is(err, processingqueue.ErrPermanentFailure)
			finalAttempt := job.Attempts >= job.MaxAttempts
			if permanent || finalAttempt {
				p.markRequestFailed(ctx, log, job.Payload.RequestEntryID)
			}
			return err
		default:
			return fmt.Errorf("unsupported request type for download queue: %s", req.Type)
		}
	}
	for i := 1; i <= workers; i++ {
		workerID := fmt.Sprintf("download-worker-%d", i)
		p.queue.StartWorker(ctx, workerID, handler)
		log.Info("download queue worker started", zap.String("worker_id", workerID))
	}
}

func (p *Processor) markRequestFailed(ctx context.Context, log *zap.Logger, requestEntryID uint) {
	if p.requestStatus == nil || requestEntryID == 0 {
		return
	}
	updated, err := p.requestStatus.MarkFailedIfInFlight(ctx, requestEntryID)
	if err != nil {
		log.Warn("failed to mark download request failed",
			zap.Uint("request_entry_id", requestEntryID),
			zap.Error(err),
		)
		return
	}
	if updated {
		log.Info("marked download request failed",
			zap.Uint("request_entry_id", requestEntryID),
		)
	}
}

// processDownload is idempotent: retries reuse request.media_id or an existing
// media row for the same scope, and only enqueue hardlink when none is active.
func (p *Processor) processDownload(
	ctx context.Context,
	log *zap.Logger,
	job *processingqueue.Job[QueuePayload],
	req *RequestDetails,
) error {
	input := media.CreateFromRequestInput{
		Type:        req.Type,
		Name:        req.Name,
		IMDBID:      req.IMDBID,
		TMDBID:      req.TMDBID,
		TVDBID:      req.TVDBID,
		Season:      req.Season,
		Episode:     req.Episode,
		PosterURL:   req.PosterURL,
		TorrentURL:  req.TorrentURL,
		TorrentName: req.TorrentName,
		Indexer:     req.Indexer,
		Quality:     req.Quality,
		UserID:      req.UserID,
		Username:    req.Username,
		RequestID:   req.RequestID,
	}

	mediaID := req.MediaID
	if mediaID == 0 {
		existing, err := p.mediaService.FindExistingDownloadMediaID(ctx, input)
		if err != nil {
			return err
		}
		if existing != 0 {
			mediaID = existing
			log.Info("reusing existing media row for download retry",
				zap.Uint("request_entry_id", job.Payload.RequestEntryID),
				zap.Uint("media_id", mediaID),
			)
		}
	}

	if mediaID == 0 {
		result, err := p.downloadService.Add(ctx, *req)
		if err != nil {
			return err
		}
		if result != nil {
			input.SavePath = result.SavePath
			input.TorrentHash = result.TorrentHash
			input.SizeBytes = result.SizeBytes
			input.StartedAt = result.StartedAt
			input.CompletedAt = result.CompletedAt
		}
		mediaID, err = p.mediaService.CreateFromRequest(ctx, input)
		if err != nil {
			return err
		}
		log.Info("created media row for download",
			zap.Uint("request_entry_id", job.Payload.RequestEntryID),
			zap.Uint("media_id", mediaID),
		)
	} else if _, err := p.mediaService.GetMediaByID(ctx, mediaID); err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			return fmt.Errorf("media %d not found for request %d: %w", mediaID, job.Payload.RequestEntryID, processingqueue.ErrPermanentFailure)
		}
		return err
	} else {
		log.Info("download job resuming with linked media (skipping torrent add and media create)",
			zap.Uint("request_entry_id", job.Payload.RequestEntryID),
			zap.Uint("media_id", mediaID),
		)
	}

	if p.requestLinker != nil {
		if err := p.requestLinker.UpdateMediaID(ctx, job.Payload.RequestEntryID, mediaID); err != nil {
			return fmt.Errorf("link request %d to media %d: %w", job.Payload.RequestEntryID, mediaID, err)
		}
	}

	active, err := p.hardlinkProcessor.HasActiveJobForMediaID(ctx, mediaID)
	if err != nil {
		return err
	}
	if active {
		log.Info("hardlink job already active for media; skipping enqueue",
			zap.Uint("media_id", mediaID),
			zap.Uint("request_entry_id", job.Payload.RequestEntryID),
		)
	} else if err := p.hardlinkProcessor.Enqueue(ctx, hardlink.QueuePayload{
		MediaID:        mediaID,
		RequestEntryID: job.Payload.RequestEntryID,
		UserID:         req.UserID,
		Username:       req.Username,
	}); err != nil {
		return err
	}

	p.markDownloading(ctx, log, job.Payload.RequestEntryID)

	log.Info("download job processed",
		zap.String("job_id", job.ID.String()),
		zap.Uint("request_entry_id", job.Payload.RequestEntryID),
		zap.String("request_id", job.Payload.RequestID),
		zap.String("type", req.Type),
		zap.Uint("media_id", mediaID),
		zap.Uint("user_id", job.Payload.UserID),
		zap.String("username", job.Payload.Username),
	)
	return nil
}

func (p *Processor) markDownloading(ctx context.Context, log *zap.Logger, requestEntryID uint) {
	if p.requestStatus == nil || requestEntryID == 0 {
		return
	}
	updated, err := p.requestStatus.MarkDownloadingIfQueued(ctx, requestEntryID)
	if err != nil {
		log.Warn("failed to mark download request downloading",
			zap.Uint("request_entry_id", requestEntryID),
			zap.Error(err),
		)
		return
	}
	if updated {
		log.Info("marked download request downloading",
			zap.Uint("request_entry_id", requestEntryID),
		)
	}
}

func (p *Processor) Enqueue(ctx context.Context, payload QueuePayload) error {
	return p.queue.Enqueue(ctx, payload)
}

// HasForwardJobForRequest reports whether a download job was already enqueued for
// this request (pending, processing, or completed — failed jobs do not count).
func (p *Processor) HasForwardJobForRequest(ctx context.Context, requestEntryID uint) (bool, error) {
	if requestEntryID == 0 {
		return false, nil
	}
	return p.queue.HasForwardJobByPayloadField(ctx, "request_entry_id", uint64(requestEntryID))
}

func (p *Processor) ListEntries(ctx context.Context, page, pageSize int) ([]QueuePayload, int64, error) {
	result, err := p.queue.ListPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]QueuePayload, 0, len(result.Entries))
	for _, row := range result.Entries {
		_ = row.CreatedAt.Format(time.RFC3339)
		entries = append(entries, row.Payload)
	}
	return entries, result.TotalCount, nil
}

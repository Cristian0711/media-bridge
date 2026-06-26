package download

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/pipeline"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/Cristian0711/media-bridge/backend/shared/queueutil"
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
	q, err := queueutil.NewQueue[QueuePayload](databaseURL, pipeline.QueueDownload, processingqueue.StandardQueueOptions()...)
	if err != nil {
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

// Shutdown waits for in-flight download workers to exit (bounded by ctx) and
// then releases the queue's database pool.
func (p *Processor) Shutdown(ctx context.Context) {
	p.queue.Wait(ctx)
	p.queue.Close()
}

func (p *Processor) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	log := logger.Component("download.queue.worker")
	handler := func(ctx context.Context, job *processingqueue.Job[QueuePayload]) error {
		// Attribute this job's logs to the user who triggered it (executed by a
		// worker, not a live request).
		ctx = logger.WithActor(ctx, logger.UserActor(job.Payload.UserID, job.Payload.Username, "").WithExecutor("queue.download"))
		req, err := p.requestSource.FindByID(ctx, job.Payload.RequestEntryID)
		if err != nil {
			return err
		}
		if !pipeline.IsDownloadType(req.Type) {
			return fmt.Errorf("unsupported request type for download queue: %s", req.Type)
		}
		err = p.processDownload(ctx, log, job, req)
		if err == nil {
			return nil
		}
		permanent := errors.Is(err, processingqueue.ErrPermanentFailure)
		finalAttempt := job.Attempts >= job.MaxAttempts
		if permanent || finalAttempt {
			p.markRequestFailed(ctx, log, job.Payload.RequestEntryID)
		}
		return err
	}
	for i := 1; i <= workers; i++ {
		workerID := fmt.Sprintf("download-worker-%d", i)
		p.queue.StartWorker(ctx, workerID, handler)
		log.InfoContext(ctx, "download queue worker started", "worker_id", workerID)
	}
}

func (p *Processor) markRequestFailed(ctx context.Context, log *slog.Logger, requestEntryID uint) {
	if p.requestStatus == nil {
		return
	}
	queueutil.MarkRequest(ctx, log, requestEntryID, "download request failed", p.requestStatus.MarkFailedIfInFlight)
}

// processDownload is idempotent: retries reuse request.media_id or an existing
// media row for the same scope, and only enqueue hardlink when none is active.
func (p *Processor) processDownload(
	ctx context.Context,
	log *slog.Logger,
	job *processingqueue.Job[QueuePayload],
	req *RequestDetails,
) error {
	requestEntryID := job.Payload.RequestEntryID

	mediaID, err := p.resolveOrCreateMedia(ctx, log, requestEntryID, req)
	if err != nil {
		return err
	}
	if err := p.linkRequestToMedia(ctx, requestEntryID, mediaID); err != nil {
		return err
	}
	if err := p.ensureHardlinkEnqueued(ctx, log, mediaID, requestEntryID, req); err != nil {
		return err
	}

	p.markDownloading(ctx, log, requestEntryID)

	log.InfoContext(ctx, "download job processed",
		"job_id", job.ID.String(),
		"request_entry_id", requestEntryID,
		"request_id", job.Payload.RequestID,
		"type", req.Type,
		"media_id", mediaID,
		"user_id", job.Payload.UserID,
		"username", job.Payload.Username,
	)
	return nil
}

// resolveOrCreateMedia returns the media row id this download should link to,
// reusing request.media_id or an existing row for the same scope, otherwise
// adding the torrent and creating a new media row. A linked-but-missing media
// row is a permanent failure.
func (p *Processor) resolveOrCreateMedia(
	ctx context.Context,
	log *slog.Logger,
	requestEntryID uint,
	req *RequestDetails,
) (uint, error) {
	input := requestToMediaInput(req)

	mediaID := req.MediaID
	if mediaID == 0 {
		existing, err := p.mediaService.FindExistingDownloadMediaID(ctx, input)
		if err != nil {
			return 0, err
		}
		if existing != 0 {
			mediaID = existing
			log.InfoContext(ctx, "reusing existing media row for download retry",
				"request_entry_id", requestEntryID,
				"media_id", mediaID,
			)
		}
	}

	if mediaID == 0 {
		result, err := p.downloadService.Add(ctx, *req)
		if err != nil {
			return 0, err
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
			return 0, err
		}
		log.InfoContext(ctx, "created media row for download",
			"request_entry_id", requestEntryID,
			"media_id", mediaID,
		)
		return mediaID, nil
	}

	if _, err := p.mediaService.GetMediaByID(ctx, mediaID); err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			return 0, fmt.Errorf("media %d not found for request %d: %w", mediaID, requestEntryID, processingqueue.ErrPermanentFailure)
		}
		return 0, err
	}
	log.InfoContext(ctx, "download job resuming with linked media (skipping torrent add and media create)",
		"request_entry_id", requestEntryID,
		"media_id", mediaID,
	)
	return mediaID, nil
}

// linkRequestToMedia writes the media id back onto the request row so a later
// remove flow can find the request via the media id.
func (p *Processor) linkRequestToMedia(ctx context.Context, requestEntryID, mediaID uint) error {
	if p.requestLinker == nil {
		return nil
	}
	if err := p.requestLinker.UpdateMediaID(ctx, requestEntryID, mediaID); err != nil {
		return fmt.Errorf("link request %d to media %d: %w", requestEntryID, mediaID, err)
	}
	return nil
}

// ensureHardlinkEnqueued enqueues a hardlink job for the media row unless one is
// already in flight (idempotent across download retries).
func (p *Processor) ensureHardlinkEnqueued(
	ctx context.Context,
	log *slog.Logger,
	mediaID, requestEntryID uint,
	req *RequestDetails,
) error {
	active, err := p.hardlinkProcessor.HasActiveJobForMediaID(ctx, mediaID)
	if err != nil {
		return err
	}
	if active {
		log.InfoContext(ctx, "hardlink job already active for media; skipping enqueue",
			"media_id", mediaID,
			"request_entry_id", requestEntryID,
		)
		return nil
	}
	return p.hardlinkProcessor.Enqueue(ctx, hardlink.QueuePayload{
		MediaID:        mediaID,
		RequestEntryID: requestEntryID,
		UserID:         req.UserID,
		Username:       req.Username,
	})
}

func requestToMediaInput(req *RequestDetails) media.CreateFromRequestInput {
	return media.CreateFromRequestInput{
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
}

func (p *Processor) markDownloading(ctx context.Context, log *slog.Logger, requestEntryID uint) {
	if p.requestStatus == nil {
		return
	}
	queueutil.MarkRequest(ctx, log, requestEntryID, "download request downloading", p.requestStatus.MarkDownloadingIfQueued)
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
	return queueutil.ListPayloads(ctx, p.queue, page, pageSize)
}

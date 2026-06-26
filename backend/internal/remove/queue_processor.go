package remove

import (
	"context"
	"errors"
	"fmt"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/pipeline"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/Cristian0711/media-bridge/backend/shared/queueutil"
	"go.uber.org/zap"
)

type QueuePayload struct {
	RequestEntryID uint   `json:"request_entry_id"`
	RequestID      string `json:"request_id"`
	MediaID        uint   `json:"media_id"`
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
}

// DownloadCanceller marks in-flight download requests for a media row as
// cancelled before removal begins (stops the completion watcher from racing).
type DownloadCanceller interface {
	CancelDownloadsByMediaID(ctx context.Context, mediaID uint) (int64, error)
}

// RemoveRequestStatusUpdater updates the remove request row when removal finishes.
type RemoveRequestStatusUpdater interface {
	MarkRemovedIfRemoving(ctx context.Context, requestID uint) (bool, error)
	MarkFailedIfRemoving(ctx context.Context, requestID uint) (bool, error)
}

type Processor struct {
	queue         *processingqueue.Queue[QueuePayload]
	requestSource interface {
		FindByID(ctx context.Context, id uint) (*RequestDetails, error)
	}
	removeService     Service
	mediaService      media.Service
	downloadCanceller DownloadCanceller
	requestStatus     RemoveRequestStatusUpdater
}

func NewProcessor(
	databaseURL string,
	requestSource interface {
		FindByID(ctx context.Context, id uint) (*RequestDetails, error)
	},
	removeService Service,
	mediaService media.Service,
	downloadCanceller DownloadCanceller,
	requestStatus RemoveRequestStatusUpdater,
) (*Processor, error) {
	q, err := queueutil.NewQueue[QueuePayload](databaseURL, pipeline.QueueRemove, processingqueue.LongRunningQueueOptions()...)
	if err != nil {
		return nil, err
	}
	return &Processor{
		queue:             q,
		requestSource:     requestSource,
		removeService:     removeService,
		mediaService:      mediaService,
		downloadCanceller: downloadCanceller,
		requestStatus:     requestStatus,
	}, nil
}

// Shutdown waits for in-flight remove workers to exit (bounded by ctx) and then
// releases the queue's database pool.
func (p *Processor) Shutdown(ctx context.Context) {
	p.queue.Wait(ctx)
	p.queue.Close()
}

func (p *Processor) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	log := logger.Named("remove.queue.worker")
	handler := func(ctx context.Context, job *processingqueue.Job[QueuePayload]) error {
		req, err := p.requestSource.FindByID(ctx, job.Payload.RequestEntryID)
		if err != nil {
			return err
		}
		if !pipeline.IsRemoveType(req.Type) {
			return fmt.Errorf("unsupported request type for remove queue: %s", req.Type)
		}
		err = p.processRemoveJob(ctx, log, job, req)
		if err == nil {
			return nil
		}
		permanent := errors.Is(err, processingqueue.ErrPermanentFailure)
		finalAttempt := job.Attempts >= job.MaxAttempts
		if permanent || finalAttempt {
			p.markRemoveRequestFailed(ctx, log, job.Payload.RequestEntryID)
		}
		return err
	}
	for i := 1; i <= workers; i++ {
		workerID := fmt.Sprintf("remove-worker-%d", i)
		p.queue.StartWorker(ctx, workerID, handler)
		log.Info("remove queue worker started", zap.String("worker_id", workerID))
	}
}

func (p *Processor) processRemoveJob(
	ctx context.Context,
	log *zap.Logger,
	job *processingqueue.Job[QueuePayload],
	req *RequestDetails,
) error {
	// Cancel download requests first so the completion watcher cannot mark
	// them 'downloaded' while we are deleting files (C3).
	if p.downloadCanceller != nil && req.MediaID != 0 {
		n, cancelErr := p.downloadCanceller.CancelDownloadsByMediaID(ctx, req.MediaID)
		if cancelErr != nil {
			log.Warn("cancel download requests before remove failed (continuing)",
				zap.Uint("media_id", req.MediaID),
				zap.Error(cancelErr),
			)
		} else if n > 0 {
			log.Info("cancelled download requests before remove",
				zap.Uint("media_id", req.MediaID),
				zap.Int64("count", n),
			)
		}
	}

	if err := p.removeService.Process(ctx, *req); err != nil {
		return err
	}
	if err := p.mediaService.RemoveFromRequest(ctx, media.CreateFromRequestInput{
		MediaID: req.MediaID,
		Type:    req.Type,
	}); err != nil {
		return err
	}

	if p.requestStatus != nil {
		updated, err := p.requestStatus.MarkRemovedIfRemoving(ctx, job.Payload.RequestEntryID)
		if err != nil {
			return fmt.Errorf("mark remove request removed: %w", err)
		}
		if !updated {
			log.Warn("remove finished but request was not in removing state",
				zap.Uint("request_entry_id", job.Payload.RequestEntryID),
			)
		}
	}

	log.Info("remove job processed",
		zap.String("job_id", job.ID.String()),
		zap.Uint("request_entry_id", job.Payload.RequestEntryID),
		zap.String("request_id", job.Payload.RequestID),
		zap.String("type", req.Type),
		zap.Uint("user_id", job.Payload.UserID),
		zap.String("username", job.Payload.Username),
	)
	return nil
}

func (p *Processor) markRemoveRequestFailed(ctx context.Context, log *zap.Logger, requestEntryID uint) {
	if p.requestStatus == nil {
		return
	}
	queueutil.MarkRequest(ctx, log, requestEntryID, "remove request failed", p.requestStatus.MarkFailedIfRemoving)
}

func (p *Processor) Enqueue(ctx context.Context, payload QueuePayload) error {
	return p.queue.Enqueue(ctx, payload)
}

// HasForwardJobForRequest reports whether a remove job was already enqueued for
// this request (pending, processing, or completed — failed jobs do not count).
func (p *Processor) HasForwardJobForRequest(ctx context.Context, requestEntryID uint) (bool, error) {
	if requestEntryID == 0 {
		return false, nil
	}
	return p.queue.HasForwardJobByPayloadField(ctx, "request_entry_id", uint64(requestEntryID))
}

// HasActiveRemoveForMedia reports pending/processing remove jobs for a media row (H5).
func (p *Processor) HasActiveRemoveForMedia(ctx context.Context, mediaID uint) (bool, error) {
	if mediaID == 0 {
		return false, nil
	}
	return p.queue.HasActiveJobByPayloadField(ctx, "media_id", uint64(mediaID))
}

func (p *Processor) ListEntries(ctx context.Context, page, pageSize int) ([]QueuePayload, int64, error) {
	return queueutil.ListPayloads(ctx, p.queue, page, pageSize)
}

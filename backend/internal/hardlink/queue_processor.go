package hardlink

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueuePayload struct {
	MediaID        uint   `json:"media_id"`
	RequestEntryID uint   `json:"request_entry_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
}

// CorrelationID lets the queue tag every job log with the originating request id.
func (p QueuePayload) CorrelationID() string { return p.RequestID }

// RequestStatusUpdater lets the hardlink processor mark failed requests when
// hardlinking permanently fails. Successful hardlinks leave the request in
// 'downloading' until the download completion watcher confirms the torrent too.
type RequestStatusUpdater interface {
	UpdateStatus(ctx context.Context, requestID uint, status string) error
}

// RemoveInProgressGuard reports active remove_processing_queue jobs for a media row.
type RemoveInProgressGuard interface {
	HasActiveRemoveForMedia(ctx context.Context, mediaID uint) (bool, error)
}

type Processor struct {
	queue         *processingqueue.Queue[QueuePayload]
	service       Service
	requestStatus RequestStatusUpdater
	removeGuard   RemoveInProgressGuard
}

func NewProcessor(databaseURL string, service Service, requestStatus RequestStatusUpdater) (*Processor, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	q, err := processingqueue.New[QueuePayload](
		pool,
		"hardlink_processing_queue",
		processingqueue.LongRunningQueueOptions()...,
	)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := q.EnsureTable(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}
	return &Processor{queue: q, service: service, requestStatus: requestStatus}, nil
}

// Shutdown waits for in-flight hardlink workers to exit (bounded by ctx) and
// then releases the queue's database pool. Call from the server's shutdown path
// after the root worker context has been cancelled.
func (p *Processor) Shutdown(ctx context.Context) {
	p.queue.Wait(ctx)
	p.queue.Close()
}

// SetRemoveGuard wires the remove queue so hardlink jobs defer while removal runs (H5).
func (p *Processor) SetRemoveGuard(guard RemoveInProgressGuard) {
	p.removeGuard = guard
}

// checkRemoveInProgress returns a retryable error when a remove job is active for this media (H5).
func (p *Processor) checkRemoveInProgress(ctx context.Context, log *slog.Logger, mediaID uint, jobID string) error {
	if p.removeGuard == nil || mediaID == 0 {
		return nil
	}
	active, err := p.removeGuard.HasActiveRemoveForMedia(ctx, mediaID)
	if err != nil {
		return err
	}
	if active {
		log.InfoContext(ctx, "deferring hardlink while remove is in progress",
			"media_id", mediaID,
			"job_id", jobID,
		)
		return fmt.Errorf("remove in progress for media %d", mediaID)
	}
	return nil
}

func (p *Processor) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	log := logger.Component("hardlink.queue.worker")
	handler := func(ctx context.Context, job *processingqueue.Job[QueuePayload]) error {
		// Attribute this job's logs to the user who triggered it (executed by a
		// worker, not a live request). The request_id is seeded by the queue
		// worker (Correlatable), so it's already on ctx.
		ctx = logger.WithActor(ctx, logger.UserActor(job.Payload.UserID, job.Payload.Username, "").WithExecutor("queue.hardlink"))
		if err := p.checkRemoveInProgress(ctx, log, job.Payload.MediaID, job.ID.String()); err != nil {
			return err
		}

		err := p.service.Hardlink(ctx, job.Payload.MediaID)
		if err == nil {
			// Guard: if the queue row was cancelled (typically by the remove
			// flow) while we were running, the hardlinks we just created are
			// about to be torn down. Don't flip the request to 'downloaded'.
			if status, gerr := p.queue.GetStatus(ctx, job.ID); gerr == nil && status != "processing" {
				log.InfoContext(ctx, "hardlink succeeded but job was cancelled mid-flight; skipping request status update",
					"job_id", job.ID.String(),
					"queue_status", status,
					"media_id", job.Payload.MediaID,
				)
				return nil
			}
			log.InfoContext(ctx, "hardlink job processed; awaiting torrent completion before marking downloaded",
				"job_id", job.ID.String(),
				"media_id", job.Payload.MediaID,
				"request_entry_id", job.Payload.RequestEntryID,
				"user_id", job.Payload.UserID,
				"username", job.Payload.Username,
			)
			return nil
		}

		// Mark the request failed when this attempt won't be retried:
		//  - permanent failure (sentinel), or
		//  - we just consumed the final attempt and the queue will set 'failed'.
		// Defer retries (torrent still downloading) do not count as final.
		permanent := errors.Is(err, processingqueue.ErrPermanentFailure)
		deferRetry := errors.Is(err, processingqueue.ErrDeferRetry)
		finalAttempt := !deferRetry && job.Attempts >= job.MaxAttempts
		if permanent || finalAttempt {
			p.markRequest(ctx, log, job.Payload.RequestEntryID, "failed")
		}
		if deferRetry {
			log.DebugContext(ctx, "hardlink deferred until torrent finishes downloading",
				"job_id", job.ID.String(),
				"media_id", job.Payload.MediaID,
				logger.Err(err),
			)
		}
		return err
	}
	for i := 1; i <= workers; i++ {
		workerID := fmt.Sprintf("hardlink-worker-%d", i)
		p.queue.StartWorker(ctx, workerID, handler)
		log.InfoContext(ctx, "hardlink queue worker started", "worker_id", workerID)
	}
}

func (p *Processor) markRequest(ctx context.Context, log *slog.Logger, requestID uint, status string) {
	if p.requestStatus == nil || requestID == 0 {
		return
	}
	if err := p.requestStatus.UpdateStatus(ctx, requestID, status); err != nil {
		log.WarnContext(ctx, "failed to update request status",
			"request_entry_id", requestID,
			"status", status,
			logger.Err(err),
		)
	}
}

func (p *Processor) Enqueue(ctx context.Context, payload QueuePayload) error {
	return p.queue.Enqueue(ctx, payload)
}

// HasActiveJobForMediaID reports whether a hardlink job is already queued or running.
func (p *Processor) HasActiveJobForMediaID(ctx context.Context, mediaID uint) (bool, error) {
	if mediaID == 0 {
		return false, nil
	}
	return p.queue.HasActiveJobByPayloadField(ctx, "media_id", uint64(mediaID))
}

// CancelByMediaID marks every pending/processing hardlink job for the given
// media row as 'failed' so that the remove flow can delete files without a
// concurrent hardlink job racing to recreate them.
func (p *Processor) CancelByMediaID(ctx context.Context, mediaID uint) error {
	_, err := p.queue.CancelByPayloadField(ctx, "media_id", uint64(mediaID))
	return err
}

func (p *Processor) ListEntries(ctx context.Context, page, pageSize int) ([]QueuePayload, int64, error) {
	result, err := p.queue.ListPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]QueuePayload, 0, len(result.Entries))
	for _, row := range result.Entries {
		entries = append(entries, row.Payload)
	}
	return entries, result.TotalCount, nil
}

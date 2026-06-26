package requests

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/download"
	"github.com/Cristian0711/media-bridge/backend/internal/remove"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

type QueuePayload struct {
	RequestEntryID uint   `json:"request_entry_id"`
	RequestID      string `json:"request_id"`
	Type           string `json:"type"`
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
}

// DownloadForwarder enqueues download_processing_queue jobs idempotently.
type DownloadForwarder interface {
	Enqueue(ctx context.Context, payload download.QueuePayload) error
	HasForwardJobForRequest(ctx context.Context, requestEntryID uint) (bool, error)
}

// RemoveForwarder enqueues remove_processing_queue jobs idempotently.
type RemoveForwarder interface {
	Enqueue(ctx context.Context, payload remove.QueuePayload) error
	HasForwardJobForRequest(ctx context.Context, requestEntryID uint) (bool, error)
}

type QueueProcessor struct {
	queue         *processingqueue.Queue[QueuePayload]
	repo          Repository
	downloadQueue DownloadForwarder
	removeQueue   RemoveForwarder
}

func NewQueueProcessor(
	databaseURL string,
	repo Repository,
	downloadProcess DownloadForwarder,
	removeProcess RemoveForwarder,
) (*QueueProcessor, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create queue pool: %w", err)
	}

	q, err := processingqueue.New[QueuePayload](pool, "requests_processing_queue",
		processingqueue.RoutingQueueOptions()...,
	)
	if err != nil {
		return nil, fmt.Errorf("create queue: %w", err)
	}
	if err := q.EnsureTable(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure queue table: %w", err)
	}

	return &QueueProcessor{
		queue:         q,
		repo:          repo,
		downloadQueue: downloadProcess,
		removeQueue:   removeProcess,
	}, nil
}

// Shutdown waits for in-flight requests-queue workers to exit (bounded by ctx)
// and then releases the queue's database pool.
func (p *QueueProcessor) Shutdown(ctx context.Context) {
	p.queue.Wait(ctx)
	p.queue.Close()
}

func (p *QueueProcessor) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	log := logger.Component("requests.queue.worker")
	handler := func(ctx context.Context, job *processingqueue.Job[QueuePayload]) error {
		// Attribute this job's logs to the user who triggered it (executed by a
		// worker, not a live request).
		ctx = logger.WithActor(ctx, logger.UserActor(job.Payload.UserID, job.Payload.Username, "").WithExecutor("queue.requests"))
		req, err := p.repo.FindByID(ctx, job.Payload.RequestEntryID)
		if err != nil {
			return err
		}
		return p.processRequest(ctx, log, job, req)
	}
	for i := 1; i <= workers; i++ {
		workerID := fmt.Sprintf("requests-worker-%d", i)
		p.queue.StartWorker(ctx, workerID, handler)
		log.InfoContext(ctx, "requests queue worker started", "worker_id", workerID)
	}
}

// processRequest forwards a request to the download or remove queue once, then
// advances status from pending. Retries after a successful child enqueue do not
// create duplicate downstream jobs (C2).
func (p *QueueProcessor) processRequest(
	ctx context.Context,
	log *slog.Logger,
	job *processingqueue.Job[QueuePayload],
	req *Request,
) error {
	if req.Status != StatusPending {
		log.DebugContext(ctx, "request already forwarded; skipping child enqueue",
			"request_entry_id", req.ID,
			"status", req.Status,
		)
		return nil
	}

	var targetQueue string
	switch req.Type {
	case TypeMovieDownload, TypeShowDownload:
		targetQueue = "download_processing_queue"
		if err := p.forwardDownload(ctx, log, req); err != nil {
			return err
		}
	case TypeMovieRemove, TypeShowRemove:
		targetQueue = "remove_processing_queue"
		if err := p.forwardRemove(ctx, log, req); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported request type: %s", req.Type)
	}

	log.InfoContext(ctx, "request job forwarded",
		"job_id", job.ID.String(),
		"queue_name", job.QueueName,
		"job_status", job.Status,
		"attempts", job.Attempts,
		"request_entry_id", req.ID,
		"request_id", req.RequestID,
		"request_type", req.Type,
		"request_name", req.Name,
		"user_id", req.UserID,
		"username", req.Username,
		"indexer", req.Indexer,
		"quality", req.Quality,
		"target_queue", targetQueue,
	)
	return nil
}

func (p *QueueProcessor) forwardDownload(ctx context.Context, log *slog.Logger, req *Request) error {
	hasJob, err := p.downloadQueue.HasForwardJobForRequest(ctx, req.ID)
	if err != nil {
		return err
	}
	if hasJob {
		log.InfoContext(ctx, "download job already exists for request; skipping enqueue",
			"request_entry_id", req.ID,
		)
	} else if err := p.downloadQueue.Enqueue(ctx, download.QueuePayload{
		RequestEntryID: req.ID,
		RequestID:      req.RequestID,
		UserID:         req.UserID,
		Username:       req.Username,
	}); err != nil {
		return err
	}

	return p.markForwarded(ctx, log, req, p.repo.MarkQueuedIfPending)
}

func (p *QueueProcessor) forwardRemove(ctx context.Context, log *slog.Logger, req *Request) error {
	hasJob, err := p.removeQueue.HasForwardJobForRequest(ctx, req.ID)
	if err != nil {
		return err
	}
	if hasJob {
		log.InfoContext(ctx, "remove job already exists for request; skipping enqueue",
			"request_entry_id", req.ID,
		)
	} else if err := p.removeQueue.Enqueue(ctx, remove.QueuePayload{
		RequestEntryID: req.ID,
		RequestID:      req.RequestID,
		MediaID:        req.MediaID,
		UserID:         req.UserID,
		Username:       req.Username,
	}); err != nil {
		return err
	}

	return p.markForwarded(ctx, log, req, p.repo.MarkRemovingIfPending)
}

// markForwarded applies the guarded pending → queued/removing transition (R7),
// tolerating the case where the request already advanced past pending.
func (p *QueueProcessor) markForwarded(
	ctx context.Context,
	log *slog.Logger,
	req *Request,
	mark func(ctx context.Context, requestID uint) (bool, error),
) error {
	updated, err := mark(ctx, req.ID)
	if err != nil {
		return err
	}
	if !updated {
		log.DebugContext(ctx, "skip status update: request no longer pending",
			"request_entry_id", req.ID,
		)
	}
	return nil
}

func (p *QueueProcessor) Enqueue(ctx context.Context, payload QueuePayload) error {
	return p.queue.Enqueue(ctx, payload)
}

// EnqueueInGormTx inserts a routing job in the same Postgres transaction as tx (R2).
func (p *QueueProcessor) EnqueueInGormTx(ctx context.Context, tx *gorm.DB, payload QueuePayload) error {
	return p.queue.EnqueueInGormTx(ctx, tx, payload)
}

// HasJobForRequest reports whether a requests_processing_queue job exists for this request.
func (p *QueueProcessor) HasJobForRequest(ctx context.Context, requestEntryID uint) (bool, error) {
	if requestEntryID == 0 {
		return false, nil
	}
	return p.queue.HasJobForPayloadField(ctx, "request_entry_id", uint64(requestEntryID))
}

// PurgeCompletedOlderThan deletes completed jobs older than retention (R9).
func (p *QueueProcessor) PurgeCompletedOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	return p.queue.PurgeCompletedOlderThan(ctx, retention)
}

func (p *QueueProcessor) ListEntries(ctx context.Context, page, pageSize int) ([]QueueEntryResponse, int64, error) {
	result, err := p.queue.ListPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	entries := make([]QueueEntryResponse, 0, len(result.Entries))
	for _, row := range result.Entries {
		entries = append(entries, QueueEntryResponse{
			ID:             row.ID,
			QueueName:      row.QueueName,
			Status:         row.Status,
			Attempts:       row.Attempts,
			RequestEntryID: row.Payload.RequestEntryID,
			RequestID:      row.Payload.RequestID,
			UserID:         row.Payload.UserID,
			Username:       row.Payload.Username,
			Type:           row.Payload.Type,
			CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		})
	}
	return entries, result.TotalCount, nil
}

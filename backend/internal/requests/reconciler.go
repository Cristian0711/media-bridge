package requests

import (
	"context"
	"log/slog"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/download"
	"github.com/Cristian0711/media-bridge/backend/internal/remove"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
)

const (
	reconcilerInterval = 2 * time.Minute
	reconcilerMinAge   = 30 * time.Second
	requestRetention   = 90 * 24 * time.Hour
	queueRetention     = 30 * 24 * time.Hour
)

// Reconciler re-enqueues orphaned request rows and purges old terminal history (R2, R9).
type Reconciler struct {
	repo             Repository
	requestsQueue    *QueueProcessor
	downloadForward  DownloadForwarder
	removeForward    RemoveForwarder
	requestRetention time.Duration
	queueRetention   time.Duration
}

func NewReconciler(
	repo Repository,
	requestsQueue *QueueProcessor,
	downloadForward DownloadForwarder,
	removeForward RemoveForwarder,
) *Reconciler {
	return &Reconciler{
		repo:             repo,
		requestsQueue:    requestsQueue,
		downloadForward:  downloadForward,
		removeForward:    removeForward,
		requestRetention: requestRetention,
		queueRetention:   queueRetention,
	}
}

func (r *Reconciler) Start(ctx context.Context) {
	go r.run(logger.WithSystem(ctx, "requests.reconciler"))
}

func (r *Reconciler) run(ctx context.Context) {
	log := logger.Component("requests.reconciler")
	log.InfoContext(ctx, "request reconciler started",
		"interval", reconcilerInterval,
		"min_age", reconcilerMinAge,
	)
	r.tick(ctx, log)
	ticker := time.NewTicker(reconcilerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "request reconciler stopped")
			return
		case <-ticker.C:
			r.tick(ctx, log)
		}
	}
}

func (r *Reconciler) tick(ctx context.Context, log *slog.Logger) {
	r.reconcilePending(ctx, log)
	r.reconcileQueued(ctx, log)
	r.reconcileRemoving(ctx, log)
	r.purge(ctx, log)
}

func (r *Reconciler) reconcilePending(ctx context.Context, log *slog.Logger) {
	rows, err := r.repo.ListOrphanedPending(ctx, reconcilerMinAge)
	if err != nil {
		log.WarnContext(ctx, "list orphaned pending requests failed", logger.Err(err))
		return
	}
	for i := range rows {
		req := &rows[i]
		hasJob, err := r.requestsQueue.HasJobForRequest(ctx, req.ID)
		if err != nil {
			log.WarnContext(ctx, "check requests queue job failed", "request_entry_id", req.ID, logger.Err(err))
			continue
		}
		if hasJob {
			continue
		}
		if err := r.requestsQueue.Enqueue(ctx, QueuePayload{
			RequestEntryID: req.ID,
			RequestID:      req.RequestID,
			Type:           req.Type,
			UserID:         req.UserID,
			Username:       req.Username,
		}); err != nil {
			log.WarnContext(ctx, "re-enqueue orphaned pending request failed",
				"request_entry_id", req.ID,
				logger.Err(err),
			)
			continue
		}
		log.InfoContext(ctx, "re-enqueued orphaned pending request", "request_entry_id", req.ID)
	}
}

func (r *Reconciler) reconcileQueued(ctx context.Context, log *slog.Logger) {
	rows, err := r.repo.ListStuckQueued(ctx, reconcilerMinAge, r.downloadForward.HasForwardJobForRequest)
	if err != nil {
		log.WarnContext(ctx, "list stuck queued requests failed", logger.Err(err))
		return
	}
	for i := range rows {
		req := &rows[i]
		if err := r.downloadForward.Enqueue(ctx, download.QueuePayload{
			RequestEntryID: req.ID,
			RequestID:      req.RequestID,
			UserID:         req.UserID,
			Username:       req.Username,
		}); err != nil {
			log.WarnContext(ctx, "re-enqueue stuck queued download failed",
				"request_entry_id", req.ID,
				logger.Err(err),
			)
			continue
		}
		log.InfoContext(ctx, "re-enqueued stuck queued download", "request_entry_id", req.ID)
	}
}

func (r *Reconciler) reconcileRemoving(ctx context.Context, log *slog.Logger) {
	rows, err := r.repo.ListStuckRemoving(ctx, reconcilerMinAge, r.removeForward.HasForwardJobForRequest)
	if err != nil {
		log.WarnContext(ctx, "list stuck removing requests failed", logger.Err(err))
		return
	}
	for i := range rows {
		req := &rows[i]
		if err := r.removeForward.Enqueue(ctx, remove.QueuePayload{
			RequestEntryID: req.ID,
			RequestID:      req.RequestID,
			MediaID:        req.MediaID,
			UserID:         req.UserID,
			Username:       req.Username,
		}); err != nil {
			log.WarnContext(ctx, "re-enqueue stuck removing request failed",
				"request_entry_id", req.ID,
				logger.Err(err),
			)
			continue
		}
		log.InfoContext(ctx, "re-enqueued stuck removing request", "request_entry_id", req.ID)
	}
}

func (r *Reconciler) purge(ctx context.Context, log *slog.Logger) {
	n, err := r.repo.PurgeTerminalOlderThan(ctx, r.requestRetention)
	if err != nil {
		log.WarnContext(ctx, "purge terminal requests failed", logger.Err(err))
		return
	}
	if n > 0 {
		log.InfoContext(ctx, "purged old terminal request rows", "count", n)
	}
	if r.requestsQueue != nil {
		qn, qerr := r.requestsQueue.PurgeCompletedOlderThan(ctx, r.queueRetention)
		if qerr != nil {
			log.WarnContext(ctx, "purge completed queue jobs failed", logger.Err(qerr))
		} else if qn > 0 {
			log.InfoContext(ctx, "purged old completed queue jobs", "count", qn)
		}
	}
}

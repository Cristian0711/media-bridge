package requests

import (
	"context"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/download"
	"github.com/Cristian0711/media-bridge/backend/internal/remove"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
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
	go r.run(ctx)
}

func (r *Reconciler) run(ctx context.Context) {
	log := logger.Named("requests.reconciler")
	log.Info("request reconciler started",
		zap.Duration("interval", reconcilerInterval),
		zap.Duration("min_age", reconcilerMinAge),
	)
	r.tick(ctx, log)
	ticker := time.NewTicker(reconcilerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("request reconciler stopped")
			return
		case <-ticker.C:
			r.tick(ctx, log)
		}
	}
}

func (r *Reconciler) tick(ctx context.Context, log *zap.Logger) {
	r.reconcilePending(ctx, log)
	r.reconcileQueued(ctx, log)
	r.reconcileRemoving(ctx, log)
	r.purge(ctx, log)
}

func (r *Reconciler) reconcilePending(ctx context.Context, log *zap.Logger) {
	rows, err := r.repo.ListOrphanedPending(ctx, reconcilerMinAge)
	if err != nil {
		log.Warn("list orphaned pending requests failed", zap.Error(err))
		return
	}
	for i := range rows {
		req := &rows[i]
		hasJob, err := r.requestsQueue.HasJobForRequest(ctx, req.ID)
		if err != nil {
			log.Warn("check requests queue job failed", zap.Uint("request_entry_id", req.ID), zap.Error(err))
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
			log.Warn("re-enqueue orphaned pending request failed",
				zap.Uint("request_entry_id", req.ID),
				zap.Error(err),
			)
			continue
		}
		log.Info("re-enqueued orphaned pending request", zap.Uint("request_entry_id", req.ID))
	}
}

func (r *Reconciler) reconcileQueued(ctx context.Context, log *zap.Logger) {
	rows, err := r.repo.ListStuckQueued(ctx, reconcilerMinAge, r.downloadForward.HasForwardJobForRequest)
	if err != nil {
		log.Warn("list stuck queued requests failed", zap.Error(err))
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
			log.Warn("re-enqueue stuck queued download failed",
				zap.Uint("request_entry_id", req.ID),
				zap.Error(err),
			)
			continue
		}
		log.Info("re-enqueued stuck queued download", zap.Uint("request_entry_id", req.ID))
	}
}

func (r *Reconciler) reconcileRemoving(ctx context.Context, log *zap.Logger) {
	rows, err := r.repo.ListStuckRemoving(ctx, reconcilerMinAge, r.removeForward.HasForwardJobForRequest)
	if err != nil {
		log.Warn("list stuck removing requests failed", zap.Error(err))
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
			log.Warn("re-enqueue stuck removing request failed",
				zap.Uint("request_entry_id", req.ID),
				zap.Error(err),
			)
			continue
		}
		log.Info("re-enqueued stuck removing request", zap.Uint("request_entry_id", req.ID))
	}
}

func (r *Reconciler) purge(ctx context.Context, log *zap.Logger) {
	n, err := r.repo.PurgeTerminalOlderThan(ctx, r.requestRetention)
	if err != nil {
		log.Warn("purge terminal requests failed", zap.Error(err))
		return
	}
	if n > 0 {
		log.Info("purged old terminal request rows", zap.Int64("count", n))
	}
	if r.requestsQueue != nil {
		qn, qerr := r.requestsQueue.PurgeCompletedOlderThan(ctx, r.queueRetention)
		if qerr != nil {
			log.Warn("purge completed queue jobs failed", zap.Error(qerr))
		} else if qn > 0 {
			log.Info("purged old completed queue jobs", zap.Int64("count", qn))
		}
	}
}

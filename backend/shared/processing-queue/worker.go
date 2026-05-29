package processingqueue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HandlerFunc is the function you provide to process a job.
// Return nil to mark the job completed; return an error to fail it (and
// requeue it if retries remain).
type HandlerFunc[T any] func(ctx context.Context, job *Job[T]) error

// StartWorker starts a background goroutine that polls for jobs and calls
// handler for each one. It also starts the stale-job recovery loop the first
// time it is called on this Queue.
//
// The goroutines stop when ctx is cancelled. Call StartWorker multiple times
// to run concurrent workers on the same queue.
func (q *Queue[T]) StartWorker(ctx context.Context, workerID string, handler HandlerFunc[T]) {
	q.startRecoveryOnce(ctx)
	go q.runWorker(ctx, workerID, handler)
}

// Process is the blocking version of StartWorker. It runs in the calling
// goroutine and returns when ctx is cancelled.
func (q *Queue[T]) Process(ctx context.Context, workerID string, handler HandlerFunc[T]) {
	q.startRecoveryOnce(ctx)
	q.runWorker(ctx, workerID, handler)
}

// ---- internals ---------------------------------------------------------------

// recoveryOnce ensures the recovery goroutine starts exactly once per Queue,
// regardless of how many workers are started.
var recoveryMu sync.Mutex
var recoveryStarted = map[string]bool{}

func (q *Queue[T]) startRecoveryOnce(ctx context.Context) {
	key := q.opts.Table + "|" + q.name
	recoveryMu.Lock()
	defer recoveryMu.Unlock()
	if recoveryStarted[key] {
		return
	}
	recoveryStarted[key] = true
	go q.runRecovery(ctx)
}

func (q *Queue[T]) runWorker(ctx context.Context, workerID string, handler HandlerFunc[T]) {
	for {
		// Check for cancellation before each iteration.
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := q.Dequeue(ctx, workerID)
		if err != nil {
			slog.ErrorContext(ctx, "processingqueue: dequeue error",
				"queue", q.name,
				"worker", workerID,
				"error", err,
			)
			q.wait(ctx)
			continue
		}

		if job == nil {
			// Queue is empty — back off before polling again.
			q.wait(ctx)
			continue
		}

		handlerCtx, cancelHandler := context.WithCancel(ctx)
		stopHeartbeat := q.startLeaseHeartbeat(handlerCtx, job.ID)
		err = handler(handlerCtx, job)
		stopHeartbeat()
		cancelHandler()

		if err != nil {
			permanent := errors.Is(err, ErrPermanentFailure)
			slog.WarnContext(ctx, "processingqueue: job failed",
				"queue", q.name,
				"job_id", job.ID,
				"attempts", job.Attempts,
				"max_attempts", job.MaxAttempts,
				"permanent", permanent,
				"error", err,
			)
			var (
				rows    int64
				failErr error
			)
			if permanent {
				rows, failErr = q.FailPermanent(ctx, job.ID, err)
			} else {
				rows, failErr = q.Fail(ctx, job.ID, err)
			}
			if failErr != nil {
				slog.ErrorContext(ctx, "processingqueue: could not record failure",
					"queue", q.name,
					"job_id", job.ID,
					"error", failErr,
				)
			} else if rows == 0 {
				// The row left 'processing' before we could fail it —
				// typically a CancelByPayloadField from another flow.
				slog.InfoContext(ctx, "processingqueue: job state changed externally before fail",
					"queue", q.name,
					"job_id", job.ID,
				)
			}
		} else {
			rows, completeErr := q.Complete(ctx, job.ID)
			if completeErr != nil {
				slog.ErrorContext(ctx, "processingqueue: could not mark completed",
					"queue", q.name,
					"job_id", job.ID,
					"error", completeErr,
				)
			} else if rows == 0 {
				// Same as above — handler returned nil but the row was
				// flipped out from under us (cancel). Handlers can defend
				// against firing side effects on such success by checking
				// GetStatus before mutating related state.
				slog.InfoContext(ctx, "processingqueue: job state changed externally before complete",
					"queue", q.name,
					"job_id", job.ID,
				)
			}
		}
		// No sleep here — immediately try to grab the next job.
	}
}

// runRecovery periodically requeues jobs whose worker stopped responding.
func (q *Queue[T]) runRecovery(ctx context.Context) {
	ticker := time.NewTicker(q.opts.RecoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := q.recoverStale(ctx); err != nil {
				slog.ErrorContext(ctx, "processingqueue: recovery error",
					"queue", q.name,
					"error", err,
				)
			}
		}
	}
}

func (q *Queue[T]) recoverStale(ctx context.Context) error {
	// Use last_processed_at as the lease clock — workers bump it via TouchLease (R6).
	sql := fmt.Sprintf(`
		UPDATE %s
		SET
			status    = 'pending',
			queued_at = NOW(),
			worker_id = NULL,
			error     = 'worker timeout — requeued by recovery'
		WHERE queue_name = $1
		  AND status     = 'processing'
		  AND COALESCE(last_processed_at, started_at) < NOW() - $2::INTERVAL
	`, q.opts.Table)

	tag, err := q.db.Exec(ctx, sql, q.name, q.opts.WorkerTimeout.String())
	if err != nil {
		return fmt.Errorf("recover stale jobs: %w", err)
	}
	if tag.RowsAffected() > 0 {
		slog.InfoContext(ctx, "processingqueue: requeued stale jobs",
			"queue", q.name,
			"count", tag.RowsAffected(),
		)
	}
	return nil
}

const leaseHeartbeatInterval = 30 * time.Second

func (q *Queue[T]) startLeaseHeartbeat(ctx context.Context, jobID uuid.UUID) context.CancelFunc {
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(leaseHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := q.TouchLease(heartbeatCtx, jobID); err != nil {
					slog.DebugContext(heartbeatCtx, "processingqueue: lease heartbeat failed",
						"queue", q.name,
						"job_id", jobID,
						"error", err,
					)
				}
			}
		}
	}()
	return cancel
}

// wait blocks for PollInterval or until ctx is done.
func (q *Queue[T]) wait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(q.opts.PollInterval):
	}
}

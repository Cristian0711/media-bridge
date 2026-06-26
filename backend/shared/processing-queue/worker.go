package processingqueue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// startJobSpan starts a span for processing one job. It CONTINUES the trace that
// enqueued the job (extracting the stored traceparent as the parent), so the
// whole pipeline — the originating request and every queue hop it fans out to —
// shares one trace id end-to-end. The job span is short-lived (one attempt);
// successive attempts/hops are sibling spans under the same trace, which is the
// natural shape for an async pipeline.
func (q *Queue[T]) startJobSpan(ctx context.Context, job *Job[T]) (context.Context, trace.Span) {
	if job.Traceparent != "" {
		ctx = otel.GetTextMapPropagator().Extract(
			ctx,
			propagation.MapCarrier{"traceparent": job.Traceparent},
		)
	}
	return otel.Tracer("processing-queue").Start(ctx, "queue.process "+q.name,
		trace.WithAttributes(
			attribute.String("queue.name", q.name),
			attribute.String("job.id", job.ID.String()),
			attribute.Int("job.attempts", job.Attempts),
		),
	)
}

// Correlatable lets a job payload expose a stable correlation id (the
// originating request id). When a payload implements it, the worker tags every
// log for that job with it, so an operation's whole pipeline is greppable by
// request_id across queue hops.
type Correlatable interface {
	CorrelationID() string
}

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
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		q.runWorker(ctx, workerID, handler)
	}()
}

// Process is the blocking version of StartWorker. It runs in the calling
// goroutine and returns when ctx is cancelled.
func (q *Queue[T]) Process(ctx context.Context, workerID string, handler HandlerFunc[T]) {
	q.startRecoveryOnce(ctx)
	q.runWorker(ctx, workerID, handler)
}

// ---- internals ---------------------------------------------------------------

// startRecoveryOnce launches the stale-job recovery goroutine exactly once for
// this Queue instance, regardless of how many workers are started. State lives
// on the Queue (sync.Once), not in a process-global map, so a freshly created
// queue always gets a recovery loop bound to its own context.
func (q *Queue[T]) startRecoveryOnce(ctx context.Context) {
	q.recoveryOnce.Do(func() {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			q.runRecovery(ctx)
		}()
	})
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
			logger.Error(ctx, "queue.dequeue_failed", "processingqueue: dequeue error", err,
				"queue", q.name,
				"worker", workerID,
			)
			q.wait(ctx)
			continue
		}

		if job == nil {
			// Queue is empty — back off before polling again.
			q.wait(ctx)
			continue
		}

		// Bound the handler by WorkerTimeout so a hung handler (e.g. a stuck
		// HTTP call) is actually cancelled instead of pinning the worker slot
		// forever while the DB-side recovery loop requeues the row underneath
		// it. Handlers must honour ctx for this to take effect.
		handlerCtx, cancelHandler := context.WithTimeout(ctx, q.opts.WorkerTimeout)
		// Continue the trace that enqueued this job, so the whole pipeline shares
		// one trace id (see startJobSpan).
		handlerCtx, span := q.startJobSpan(handlerCtx, job)
		// Tag every log of this job — the handler's domain logs AND the generic
		// queue logs below — with the originating request id, so the whole
		// pipeline is greppable by request_id (not just trace_id).
		if c, ok := any(job.Payload).(Correlatable); ok {
			if id := c.CorrelationID(); id != "" {
				handlerCtx = logger.WithRequestID(handlerCtx, id)
			}
		}
		stopHeartbeat := q.startLeaseHeartbeat(handlerCtx, job.ID)
		err = q.invokeHandler(handlerCtx, handler, job)
		stopHeartbeat()
		if err != nil && !errors.Is(err, ErrDeferRetry) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
		cancelHandler()

		// Record the terminal job state on a context detached from cancellation
		// (so a shutting-down worker still persists the final state) but keeping
		// the job's values — span/trace id, request id, actor — so these logs
		// correlate with the rest of the pipeline.
		writeCtx, cancelWrite := context.WithTimeout(context.WithoutCancel(handlerCtx), 5*time.Second)

		if err != nil {
			permanent := errors.Is(err, ErrPermanentFailure)
			deferRetry := errors.Is(err, ErrDeferRetry)
			slog.WarnContext(handlerCtx, "processingqueue: job failed",
				"queue", q.name,
				"job_id", job.ID,
				"attempts", job.Attempts,
				"max_attempts", job.MaxAttempts,
				"permanent", permanent,
				"defer", deferRetry,
				"error", err,
			)
			var (
				rows    int64
				failErr error
			)
			switch {
			case deferRetry:
				rows, failErr = q.Defer(writeCtx, job.ID, err)
			case permanent:
				rows, failErr = q.FailPermanent(writeCtx, job.ID, err)
			default:
				rows, failErr = q.Fail(writeCtx, job.ID, err)
			}
			if failErr != nil {
				logger.Error(writeCtx, "queue.fail_record_failed", "processingqueue: could not record failure", failErr,
					"queue", q.name,
					"job_id", job.ID,
				)
			} else if rows == 0 {
				// The row left 'processing' before we could fail it —
				// typically a CancelByPayloadField from another flow.
				slog.InfoContext(handlerCtx, "processingqueue: job state changed externally before fail",
					"queue", q.name,
					"job_id", job.ID,
				)
			}
		} else {
			rows, completeErr := q.Complete(writeCtx, job.ID)
			if completeErr != nil {
				logger.Error(writeCtx, "queue.complete_failed", "processingqueue: could not mark completed", completeErr,
					"queue", q.name,
					"job_id", job.ID,
				)
			} else if rows == 0 {
				// Same as above — handler returned nil but the row was
				// flipped out from under us (cancel). Handlers can defend
				// against firing side effects on such success by checking
				// GetStatus before mutating related state.
				slog.InfoContext(handlerCtx, "processingqueue: job state changed externally before complete",
					"queue", q.name,
					"job_id", job.ID,
				)
			}
		}
		cancelWrite()
		// No sleep here — immediately try to grab the next job.
	}
}

// invokeHandler runs handler and converts a panic into an error. Without this a
// panicking handler would kill the worker goroutine and leave the row stuck in
// 'processing' until the recovery loop requeues it (one full WorkerTimeout
// later). Recovering turns it into a normal job failure, so retries apply.
func (q *Queue[T]) invokeHandler(ctx context.Context, handler HandlerFunc[T], job *Job[T]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(ctx, "queue.handler_panic", "processingqueue: handler panic recovered", nil,
				"queue", q.name,
				"job_id", job.ID,
				"panic", fmt.Sprint(r),
				"stack", string(debug.Stack()),
			)
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return handler(ctx, job)
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
				logger.Error(ctx, "queue.recovery_failed", "processingqueue: recovery error", err,
					"queue", q.name,
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

// maxLeaseHeartbeatInterval caps how often a live worker bumps its lease. Long
// timeouts don't need sub-minute heartbeats.
const maxLeaseHeartbeatInterval = 30 * time.Second

// leaseHeartbeatInterval returns how often a worker should refresh its lease so
// the recovery loop never requeues a job a live worker is still processing. It
// is a fraction of WorkerTimeout (so short-timeout queues heartbeat sooner than
// the fixed 30s would allow), capped at 30s and floored at 1s to avoid
// hammering the DB.
func (q *Queue[T]) leaseHeartbeatInterval() time.Duration {
	interval := q.opts.WorkerTimeout / 3
	if interval > maxLeaseHeartbeatInterval {
		interval = maxLeaseHeartbeatInterval
	}
	if interval < time.Second {
		interval = time.Second
	}
	return interval
}

func (q *Queue[T]) startLeaseHeartbeat(ctx context.Context, jobID uuid.UUID) context.CancelFunc {
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(q.leaseHeartbeatInterval())
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

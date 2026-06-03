// Package processingqueue provides a generic, Postgres-backed processing queue.
//
// Each queue is typed to a payload struct, isolating job types while sharing a
// single Postgres table. Multiple services can safely poll concurrently — row
// locking via SELECT FOR UPDATE SKIP LOCKED ensures each job is claimed by
// exactly one worker.
//
// retry_after controls the minimum cooldown between processing attempts: a
// failed job will not be picked up again until at least that duration has
// elapsed since last_processed_at, regardless of where it sits in the queue.
//
// Typical usage:
//
//	type EmailPayload struct {
//	    To      string
//	    Subject string
//	}
//
//	q, err := processingqueue.New[EmailPayload](pool, "email_sender",
//	    processingqueue.WithRetryAfter(30*time.Second),
//	)
//
//	q.EnsureTable(ctx) // idempotent — call on every startup
//
//	// producer
//	q.Enqueue(ctx, EmailPayload{To: "alice@example.com", Subject: "Hi"})
//
//	// consumer
//	q.StartWorker(ctx, "worker-1", func(ctx context.Context, job *processingqueue.Job[EmailPayload]) error {
//	    return sendEmail(job.Payload)
//	})
package processingqueue

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

// ErrPermanentFailure marks a handler error as non-retryable. When a handler
// returns an error matching this sentinel (via errors.Is), the worker marks
// the job 'failed' immediately instead of requeueing it.
var ErrPermanentFailure = errors.New("permanent failure")

// ErrDeferRetry marks a transient condition where the job should be retried
// later without consuming an attempt (e.g. torrent still downloading while
// waiting to hardlink). Dequeue increments attempts; Defer rolls that back.
var ErrDeferRetry = errors.New("defer retry")

// Queue is a typed processing queue backed by a Postgres table.
// T is the payload type — it must be JSON-serialisable.
type Queue[T any] struct {
	db   *pgxpool.Pool
	name string // queue_name column — unique per service/job-type
	opts Options
}

// New creates a Queue for the given queue name.
//
// name identifies this queue's rows inside the shared table; use a stable,
// descriptive slug like "email_sender" or "pdf_renderer".
func New[T any](db *pgxpool.Pool, name string, opts ...Option) (*Queue[T], error) {
	if err := validateIdentifier(name); err != nil {
		return nil, fmt.Errorf("invalid queue name: %w", err)
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	if err := validateIdentifier(o.Table); err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	return &Queue[T]{db: db, name: name, opts: o}, nil
}

// EnsureTable creates the shared table and index if they do not already exist.
// Safe to call on every startup from every service — it is idempotent.
//
// If you are adding this to an existing table, run this migration manually:
//
//	ALTER TABLE processing_queue
//	    ADD COLUMN IF NOT EXISTS last_processed_at TIMESTAMPTZ,
//	    ADD COLUMN IF NOT EXISTS retry_after        BIGINT NOT NULL DEFAULT 0;
func (q *Queue[T]) EnsureTable(ctx context.Context) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			queue_name       TEXT        NOT NULL,
			payload          JSONB       NOT NULL,
			status           TEXT        NOT NULL DEFAULT 'pending'
			                 CHECK (status IN ('pending','processing','completed','failed')),

			attempts         INT         NOT NULL DEFAULT 0,
			max_attempts     INT         NOT NULL DEFAULT 3,

			-- Microseconds (matches Go's time.Duration). Stored per-row so
			-- different queues sharing the same table can have different cooldowns.
			retry_after      BIGINT      NOT NULL DEFAULT 0,

			queued_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_processed_at TIMESTAMPTZ,          -- set on every Dequeue
			started_at       TIMESTAMPTZ,
			completed_at     TIMESTAMPTZ,

			worker_id        TEXT,
			error            TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_%s_poll
			ON %s (queue_name, queued_at ASC)
			WHERE status = 'pending';
	`, q.opts.Table, q.opts.Table, q.opts.Table)

	if _, err := q.db.Exec(ctx, sql); err != nil {
		return fmt.Errorf("ensure table: %w", err)
	}
	return nil
}

// EnqueueInGormTx inserts a job in the same Postgres transaction as tx (R2).
func (q *Queue[T]) EnqueueInGormTx(ctx context.Context, tx *gorm.DB, payload T) error {
	return EnqueueGormTx(ctx, tx, q.name, q.opts, payload)
}

// HasJobForPayloadField reports pending/processing/completed jobs for a payload field.
func (q *Queue[T]) HasJobForPayloadField(ctx context.Context, field string, value uint64) (bool, error) {
	return q.HasForwardJobByPayloadField(ctx, field, value)
}

// Enqueue adds a new job to the back of the queue.
func (q *Queue[T]) Enqueue(ctx context.Context, payload T) error {
	args, err := enqueueArgs(q.name, q.opts, payload)
	if err != nil {
		return err
	}
	sql := buildEnqueueSQL(q.opts.Table, [4]string{"$1", "$2", "$3", "$4"})
	if _, err := q.db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("enqueue: %w", err)
	}
	return nil
}

// Dequeue atomically claims the oldest eligible pending job and marks it as
// processing. A job is eligible when:
//   - status = 'pending', AND
//   - last_processed_at IS NULL (never attempted), OR
//     last_processed_at + retry_after <= NOW() (cooldown has expired)
//
// Returns (nil, nil) when no eligible job exists — not an error.
func (q *Queue[T]) Dequeue(ctx context.Context, workerID string) (*Job[T], error) {
	sql := fmt.Sprintf(`
		UPDATE %s
		SET
			status            = 'processing',
			last_processed_at = NOW(),
			started_at        = NOW(),
			worker_id         = $1,
			attempts          = attempts + 1
		WHERE id = (
			SELECT id
			FROM   %s
			WHERE  queue_name = $2
			  AND  status     = 'pending'
			  AND  (
			    last_processed_at IS NULL
			    OR last_processed_at + (retry_after * INTERVAL '1 microsecond') <= NOW()
			  )
			ORDER BY queued_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING
			id, queue_name, payload, status,
			attempts, max_attempts, retry_after,
			queued_at, created_at, last_processed_at, started_at, completed_at,
			worker_id, error
	`, q.opts.Table, q.opts.Table)

	row := q.db.QueryRow(ctx, sql, workerID, q.name)
	return scanJob[T](row)
}

// Complete marks a job as successfully finished but only if the row is still
// in 'processing' state. Returns rowsAffected — 0 means the row was changed
// externally (typically cancelled) while the handler was running, in which
// case the worker should not fire any success-side effects (e.g. status
// updates on related rows).
func (q *Queue[T]) Complete(ctx context.Context, jobID uuid.UUID) (int64, error) {
	sql := fmt.Sprintf(`
		UPDATE %s
		SET status = 'completed', completed_at = NOW(), worker_id = NULL
		WHERE id = $1 AND status = 'processing'
	`, q.opts.Table)
	tag, err := q.db.Exec(ctx, sql, jobID)
	if err != nil {
		return 0, fmt.Errorf("complete job %s: %w", jobID, err)
	}
	return tag.RowsAffected(), nil
}

// FailPermanent marks a job as permanently failed regardless of remaining
// attempts but only if the row is still in 'processing' state. Returns
// rowsAffected — 0 means a cancellation or another writer already moved
// the row out of 'processing'.
func (q *Queue[T]) FailPermanent(ctx context.Context, jobID uuid.UUID, reason error) (int64, error) {
	errMsg := ""
	if reason != nil {
		errMsg = reason.Error()
	}
	sql := fmt.Sprintf(`
		UPDATE %s
		SET
			status    = 'failed',
			worker_id = NULL,
			error     = $2
		WHERE id = $1 AND status = 'processing'
	`, q.opts.Table)
	tag, err := q.db.Exec(ctx, sql, jobID, errMsg)
	if err != nil {
		return 0, fmt.Errorf("fail-permanent job %s: %w", jobID, err)
	}
	return tag.RowsAffected(), nil
}

// GetStatus returns the current status string for a job. Used by handlers
// that need to detect mid-flight cancellation before firing side effects.
// Returns an error if the row doesn't exist.
func (q *Queue[T]) GetStatus(ctx context.Context, jobID uuid.UUID) (string, error) {
	sql := fmt.Sprintf(`SELECT status FROM %s WHERE id = $1`, q.opts.Table)
	var status string
	if err := q.db.QueryRow(ctx, sql, jobID).Scan(&status); err != nil {
		return "", fmt.Errorf("get job status %s: %w", jobID, err)
	}
	return status, nil
}

// HasForwardJobByPayloadField reports whether a non-failed job exists for this payload
// field value (pending, processing, or completed). Used to avoid duplicate enqueues
// when a parent queue job retries after the child was already accepted.
func (q *Queue[T]) HasForwardJobByPayloadField(ctx context.Context, field string, value uint64) (bool, error) {
	if err := validateIdentifier(field); err != nil {
		return false, fmt.Errorf("invalid field name: %w", err)
	}
	sql := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s
			WHERE queue_name = $1
			  AND status IN ('pending', 'processing', 'completed')
			  AND (payload->>'%s')::BIGINT = $2
		)
	`, q.opts.Table, field)
	var exists bool
	if err := q.db.QueryRow(ctx, sql, q.name, value).Scan(&exists); err != nil {
		return false, fmt.Errorf("has forward job by %s: %w", field, err)
	}
	return exists, nil
}

// HasActiveJobByPayloadField reports whether a pending or processing job exists
// whose JSONB payload field equals value (numeric fields only).
func (q *Queue[T]) HasActiveJobByPayloadField(ctx context.Context, field string, value uint64) (bool, error) {
	if err := validateIdentifier(field); err != nil {
		return false, fmt.Errorf("invalid field name: %w", err)
	}
	sql := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s
			WHERE queue_name = $1
			  AND status IN ('pending', 'processing')
			  AND (payload->>'%s')::BIGINT = $2
		)
	`, q.opts.Table, field)
	var exists bool
	if err := q.db.QueryRow(ctx, sql, q.name, value).Scan(&exists); err != nil {
		return false, fmt.Errorf("has active job by %s: %w", field, err)
	}
	return exists, nil
}

// CancelByPayloadField marks every pending/processing job in this queue whose
// JSONB payload has `field` equal to `value` as 'failed' with an explanatory
// error message. Returns the number of rows updated.
//
// `field` must be a top-level JSON key on the payload object whose value is a
// JSON number (it is cast to BIGINT for comparison). Use this to cancel jobs
// keyed by a foreign id — e.g. cancelling all hardlink jobs for a media row
// that is being deleted.
//
// Note: marking a row 'failed' does NOT stop a worker that is already executing
// the handler; the worker will still call Complete/Fail on its own. That is OK
// for our cancellation flow because remove-then-requeue is idempotent.
func (q *Queue[T]) CancelByPayloadField(ctx context.Context, field string, value uint64) (int64, error) {
	if err := validateIdentifier(field); err != nil {
		return 0, fmt.Errorf("invalid field name: %w", err)
	}
	sql := fmt.Sprintf(`
		UPDATE %s
		SET
			status       = 'failed',
			worker_id    = NULL,
			completed_at = NOW(),
			error        = CASE
			                  WHEN error IS NULL OR error = '' THEN 'cancelled'
			                  ELSE error || '; cancelled'
			               END
		WHERE queue_name = $1
		  AND status IN ('pending', 'processing')
		  AND (payload->>'%s')::BIGINT = $2
	`, q.opts.Table, field)
	tag, err := q.db.Exec(ctx, sql, q.name, value)
	if err != nil {
		return 0, fmt.Errorf("cancel by payload field %s: %w", field, err)
	}
	return tag.RowsAffected(), nil
}

// Defer requeues a processing job without counting the current run as a failed
// attempt. Use when the handler returns ErrDeferRetry (unwrap via errors.Is).
func (q *Queue[T]) Defer(ctx context.Context, jobID uuid.UUID, reason error) (int64, error) {
	errMsg := ""
	if reason != nil {
		errMsg = reason.Error()
	}
	sql := fmt.Sprintf(`
		UPDATE %s
		SET
			status      = 'pending',
			queued_at   = NOW(),
			retry_after = $3,
			worker_id   = NULL,
			error       = $2,
			attempts    = GREATEST(attempts - 1, 0)
		WHERE id = $1 AND status = 'processing'
	`, q.opts.Table)
	tag, err := q.db.Exec(ctx, sql, jobID, errMsg, q.opts.RetryAfter.Microseconds())
	if err != nil {
		return 0, fmt.Errorf("defer job %s: %w", jobID, err)
	}
	return tag.RowsAffected(), nil
}

// Fail records a failure but only if the row is still in 'processing' state.
// If attempts < max_attempts the job is put back at the end of the queue
// (queued_at = NOW()); the retry_after cooldown then controls when it next
// becomes eligible. Once max_attempts is reached the status becomes 'failed'
// permanently. Returns rowsAffected — 0 means a cancellation or another
// writer already moved the row out of 'processing'.
func (q *Queue[T]) Fail(ctx context.Context, jobID uuid.UUID, reason error) (int64, error) {
	errMsg := ""
	if reason != nil {
		errMsg = reason.Error()
	}
	sql := fmt.Sprintf(`
		UPDATE %s
		SET
			status    = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
			queued_at = NOW(),
			retry_after = $3,
			worker_id = NULL,
			error     = $2
		WHERE id = $1 AND status = 'processing'
	`, q.opts.Table)
	tag, err := q.db.Exec(ctx, sql, jobID, errMsg, q.opts.RetryAfter.Microseconds())
	if err != nil {
		return 0, fmt.Errorf("fail job %s: %w", jobID, err)
	}
	return tag.RowsAffected(), nil
}

// PurgeCompletedOlderThan deletes completed/failed jobs older than retention (R9).
func (q *Queue[T]) PurgeCompletedOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	sql := fmt.Sprintf(`
		DELETE FROM %s
		WHERE queue_name = $1
		  AND status IN ('completed', 'failed')
		  AND completed_at IS NOT NULL
		  AND completed_at < $2
	`, q.opts.Table)
	tag, err := q.db.Exec(ctx, sql, q.name, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge completed jobs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// TouchLease bumps last_processed_at for a processing job (R6 heartbeat).
func (q *Queue[T]) TouchLease(ctx context.Context, jobID uuid.UUID) error {
	sql := fmt.Sprintf(`
		UPDATE %s
		SET last_processed_at = NOW()
		WHERE id = $1 AND status = 'processing'
	`, q.opts.Table)
	tag, err := q.db.Exec(ctx, sql, jobID)
	if err != nil {
		return fmt.Errorf("touch lease job %s: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("touch lease job %s: not processing", jobID)
	}
	return nil
}

// validateIdentifier ensures a string is safe to interpolate as a Postgres
// identifier. Only letters, digits, and underscores are allowed.
func validateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("must not be empty")
	}
	for _, c := range s {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			return fmt.Errorf("character %q not allowed (use letters, digits, underscores)", c)
		}
	}
	return nil
}

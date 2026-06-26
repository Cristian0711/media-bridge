package processingqueue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Job is a single item in the queue. T is the payload type you registered
// the queue with — you get it back already unmarshalled.
type Job[T any] struct {
	ID          uuid.UUID
	QueueName   string
	Payload     T
	Status      string
	Attempts    int
	MaxAttempts int

	// RetryAfter is the cooldown stored with this job (copied from queue options
	// at enqueue time). Available for logging/debugging.
	RetryAfter time.Duration

	QueuedAt        time.Time
	CreatedAt       time.Time
	LastProcessedAt *time.Time // nil until first dequeue
	StartedAt       *time.Time
	CompletedAt     *time.Time

	WorkerID *string
	Error    *string

	// Traceparent is the W3C trace context captured at enqueue time. The worker
	// uses it to link the job's span back to the request/job that produced it.
	Traceparent string
}

// scanJob reads a single row returned by a Dequeue UPDATE ... RETURNING.
// Returns (nil, nil) when there is no pending job (pgx.ErrNoRows).
func scanJob[T any](row pgx.Row) (*Job[T], error) {
	var (
		j               Job[T]
		rawPayload      []byte
		retryAfterMicro int64
		traceparent     *string
	)

	err := row.Scan(
		&j.ID,
		&j.QueueName,
		&rawPayload,
		&j.Status,
		&j.Attempts,
		&j.MaxAttempts,
		&retryAfterMicro,
		&j.QueuedAt,
		&j.CreatedAt,
		&j.LastProcessedAt,
		&j.StartedAt,
		&j.CompletedAt,
		&j.WorkerID,
		&j.Error,
		&traceparent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // queue is empty — not an error
		}
		return nil, fmt.Errorf("scan job row: %w", err)
	}

	j.RetryAfter = time.Duration(retryAfterMicro) * time.Microsecond
	if traceparent != nil {
		j.Traceparent = *traceparent
	}

	if err := json.Unmarshal(rawPayload, &j.Payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &j, nil
}

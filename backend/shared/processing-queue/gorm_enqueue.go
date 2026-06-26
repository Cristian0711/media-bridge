package processingqueue

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/gorm"
)

// buildEnqueueSQL builds the INSERT shared by the pgx (Enqueue) and GORM
// (EnqueueGormTx) paths. The two differ only in placeholder syntax: pgx uses
// $1..$5, GORM uses ?.
func buildEnqueueSQL(table string, placeholders [5]string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (queue_name, payload, max_attempts, retry_after, traceparent) VALUES (%s, %s, %s, %s, %s)",
		table, placeholders[0], placeholders[1], placeholders[2], placeholders[3], placeholders[4],
	)
}

// traceparentFromContext extracts the W3C traceparent of the active span so it
// can be stored with the job and later linked from the worker's span. Returns
// "" when there is no active span.
func traceparentFromContext(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"]
}

// enqueueArgs marshals payload and returns the ordered INSERT arguments shared
// by both enqueue paths. retry_after is stored as a microsecond bigint; the
// active span's traceparent (if any) is stored for trace linkage.
func enqueueArgs(ctx context.Context, queueName string, opts Options, payload any) ([]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return []any{queueName, data, opts.MaxAttempts, opts.RetryAfter.Microseconds(), traceparentFromContext(ctx)}, nil
}

// EnqueueGormTx inserts a queue job using the same Postgres transaction as tx.
// Use this to atomically commit a request row and its routing job (R2).
func EnqueueGormTx(ctx context.Context, tx *gorm.DB, queueName string, opts Options, payload any) error {
	if err := validateIdentifier(queueName); err != nil {
		return fmt.Errorf("invalid queue name: %w", err)
	}
	if err := validateIdentifier(opts.Table); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	args, err := enqueueArgs(ctx, queueName, opts, payload)
	if err != nil {
		return err
	}
	sql := buildEnqueueSQL(opts.Table, [5]string{"?", "?", "?", "?", "?"})
	if err := tx.WithContext(ctx).Exec(sql, args...).Error; err != nil {
		return fmt.Errorf("enqueue in tx: %w", err)
	}
	return nil
}

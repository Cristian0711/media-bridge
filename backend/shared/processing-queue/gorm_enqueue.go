package processingqueue

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// buildEnqueueSQL builds the INSERT shared by the pgx (Enqueue) and GORM
// (EnqueueGormTx) paths. The two differ only in placeholder syntax: pgx uses
// $1..$4, GORM uses ?.
func buildEnqueueSQL(table string, placeholders [4]string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (queue_name, payload, max_attempts, retry_after) VALUES (%s, %s, %s, %s)",
		table, placeholders[0], placeholders[1], placeholders[2], placeholders[3],
	)
}

// enqueueArgs marshals payload and returns the ordered INSERT arguments shared
// by both enqueue paths. retry_after is stored as a microsecond bigint.
func enqueueArgs(queueName string, opts Options, payload any) ([]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return []any{queueName, data, opts.MaxAttempts, opts.RetryAfter.Microseconds()}, nil
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
	args, err := enqueueArgs(queueName, opts, payload)
	if err != nil {
		return err
	}
	sql := buildEnqueueSQL(opts.Table, [4]string{"?", "?", "?", "?"})
	if err := tx.WithContext(ctx).Exec(sql, args...).Error; err != nil {
		return fmt.Errorf("enqueue in tx: %w", err)
	}
	return nil
}

package processingqueue

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// EnqueueGormTx inserts a queue job using the same Postgres transaction as tx.
// Use this to atomically commit a request row and its routing job (R2).
func EnqueueGormTx(ctx context.Context, tx *gorm.DB, queueName string, opts Options, payload any) error {
	if err := validateIdentifier(queueName); err != nil {
		return fmt.Errorf("invalid queue name: %w", err)
	}
	if err := validateIdentifier(opts.Table); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	sql := fmt.Sprintf(`
		INSERT INTO %s (queue_name, payload, max_attempts, retry_after)
		VALUES (?, ?, ?, ?)
	`, opts.Table)
	if err := tx.WithContext(ctx).Exec(
		sql,
		queueName,
		data,
		opts.MaxAttempts,
		opts.RetryAfter.Microseconds(),
	).Error; err != nil {
		return fmt.Errorf("enqueue in tx: %w", err)
	}
	return nil
}

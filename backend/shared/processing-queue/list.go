package processingqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type ListEntry[T any] struct {
	ID          string
	QueueName   string
	Status      string
	Attempts    int
	MaxAttempts int
	WorkerID    *string
	Error       *string
	CreatedAt   time.Time
	Payload     T
}

type PaginatedListResult[T any] struct {
	Entries    []ListEntry[T]
	Page       int
	PageSize   int
	TotalCount int64
	TotalPages int
}

func (q *Queue[T]) ListPaginated(ctx context.Context, page, pageSize int) (*PaginatedListResult[T], error) {
	page, pageSize = normalizePagination(page, pageSize)

	var total int64
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE queue_name = $1`, q.opts.Table)
	if err := q.db.QueryRow(ctx, countSQL, q.name).Scan(&total); err != nil {
		return nil, fmt.Errorf("count queue entries: %w", err)
	}

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT id, queue_name, status, attempts, max_attempts, worker_id, error, payload, created_at
		FROM %s
		WHERE queue_name = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, q.opts.Table)
	rows, err := q.db.Query(ctx, listSQL, q.name, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list queue entries: %w", err)
	}
	defer rows.Close()

	entries := make([]ListEntry[T], 0, pageSize)
	for rows.Next() {
		var (
			entry      ListEntry[T]
			payloadRaw []byte
		)
		if err := rows.Scan(
			&entry.ID,
			&entry.QueueName,
			&entry.Status,
			&entry.Attempts,
			&entry.MaxAttempts,
			&entry.WorkerID,
			&entry.Error,
			&payloadRaw,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan queue entry: %w", err)
		}
		if err := json.Unmarshal(payloadRaw, &entry.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal queue payload: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue entries: %w", err)
	}

	return &PaginatedListResult[T]{
		Entries:    entries,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		TotalPages: calcTotalPages(total, pageSize),
	}, nil
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func calcTotalPages(total int64, pageSize int) int {
	if total <= 0 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

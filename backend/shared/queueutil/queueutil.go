// Package queueutil holds the boilerplate shared by the download and remove
// queue processors: pool+queue construction, payload listing, and the
// conditional request-status update log pattern. Queue payload types differ per
// package, so every helper is generic over the payload T.
package queueutil

import (
	"context"

	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// NewQueue opens a pgx pool for databaseURL, builds a processing queue on the
// given table with opts, and ensures the table exists.
func NewQueue[T any](databaseURL, table string, opts ...processingqueue.Option) (*processingqueue.Queue[T], error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	q, err := processingqueue.New[T](pool, table, opts...)
	if err != nil {
		return nil, err
	}
	if err := q.EnsureTable(context.Background()); err != nil {
		return nil, err
	}
	return q, nil
}

// ListPayloads returns the decoded payloads for one page of queue entries,
// along with the total count.
func ListPayloads[T any](ctx context.Context, q *processingqueue.Queue[T], page, pageSize int) ([]T, int64, error) {
	result, err := q.ListPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]T, 0, len(result.Entries))
	for _, row := range result.Entries {
		entries = append(entries, row.Payload)
	}
	return entries, result.TotalCount, nil
}

// MarkRequest runs a conditional request-status update and logs the outcome.
// noun describes the transition (e.g. "download request failed"); the messages
// are "failed to mark <noun>" on error and "marked <noun>" when a row changed.
// It is a no-op when mark is nil or requestEntryID is zero.
func MarkRequest(
	ctx context.Context,
	log *zap.Logger,
	requestEntryID uint,
	noun string,
	mark func(context.Context, uint) (bool, error),
) {
	if mark == nil || requestEntryID == 0 {
		return
	}
	updated, err := mark(ctx, requestEntryID)
	if err != nil {
		log.Warn("failed to mark "+noun, zap.Uint("request_entry_id", requestEntryID), zap.Error(err))
		return
	}
	if updated {
		log.Info("marked "+noun, zap.Uint("request_entry_id", requestEntryID))
	}
}

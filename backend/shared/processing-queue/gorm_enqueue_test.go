package processingqueue_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/testhelpers"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"gorm.io/gorm"
)

func TestEnqueueGormTx_InsertsJobInSameTransaction(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.CreateProcessingQueueTable(t, db)

	opts := testhelpers.QueueOptions()
	err := db.Transaction(func(tx *gorm.DB) error {
		return processingqueue.EnqueueGormTx(context.Background(), tx, "requests_processing_queue", opts, map[string]any{
			"request_entry_id": 42,
			"request_id":       "req-1",
			"type":             "movie_remove",
		})
	})
	if err != nil {
		t.Fatalf("enqueue in tx: %v", err)
	}

	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM processing_queue`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 queue row, got %d", count)
	}
}

func TestEnqueueGormTx_RollsBackWithTransaction(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.CreateProcessingQueueTable(t, db)

	opts := testhelpers.QueueOptions()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := processingqueue.EnqueueGormTx(context.Background(), tx, "requests_processing_queue", opts, map[string]any{
			"request_entry_id": 1,
		}); err != nil {
			return err
		}
		return fmt.Errorf("simulated failure")
	})
	if err == nil {
		t.Fatal("expected transaction error")
	}

	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM processing_queue`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("queue row must roll back with failed transaction, count=%d", count)
	}
}

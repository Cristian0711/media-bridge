package testhelpers

import (
	"context"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func OpenSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func MigrateRequests(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&requests.Request{}); err != nil {
		t.Fatalf("migrate requests: %v", err)
	}
}

func MigrateMedia(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&media.Media{}, &media.Movie{}, &media.Show{}, &media.ShowEntry{}); err != nil {
		t.Fatalf("migrate media: %v", err)
	}
}

func CreateProcessingQueueTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE processing_queue (
			id TEXT PRIMARY KEY,
			queue_name TEXT NOT NULL,
			payload TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 100,
			retry_after INTEGER NOT NULL DEFAULT 0,
			queued_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_processed_at DATETIME,
			started_at DATETIME,
			completed_at DATETIME,
			worker_id TEXT,
			error TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create processing_queue: %v", err)
	}
}

func queueOptions(opts ...func(*processingqueue.Options)) processingqueue.Options {
	o := processingqueue.Options{Table: "processing_queue"}
	for _, opt := range processingqueue.RoutingQueueOptions() {
		opt(&o)
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// QueueOptions returns routing queue options with the default table name set.
func QueueOptions() processingqueue.Options {
	return queueOptions()
}

func RequestsEnqueueInTx(t *testing.T) func(*gorm.DB, *requests.Request) error {
	t.Helper()
	o := queueOptions()
	return func(tx *gorm.DB, e *requests.Request) error {
		return processingqueue.EnqueueGormTx(context.Background(), tx, "requests_processing_queue", o, map[string]any{
			"request_entry_id": e.ID,
			"request_id":       e.RequestID,
			"type":             e.Type,
			"user_id":          e.UserID,
			"username":         e.Username,
		})
	}
}

func CountQueueJobsForRequest(t *testing.T, db *gorm.DB, requestEntryID uint) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM processing_queue WHERE json_extract(payload, '$.request_entry_id') = ?`,
		requestEntryID,
	).Scan(&n).Error; err != nil {
		t.Fatalf("count queue jobs: %v", err)
	}
	return n
}

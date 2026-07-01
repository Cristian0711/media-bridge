package health

import (
	"context"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/pipeline"
	"gorm.io/gorm"
)

type queueCounts struct {
	QueueName string `gorm:"column:queue_name"`
	Status    string `gorm:"column:status"`
	Count     int64  `gorm:"column:count"`
}

func checkQueues(ctx context.Context, db *gorm.DB) Check {
	start := time.Now()
	var rows []queueCounts
	err := db.WithContext(ctx).Raw(`
		SELECT queue_name, status, COUNT(*)::bigint AS count
		FROM processing_queue
		GROUP BY queue_name, status
		ORDER BY queue_name, status
	`).Scan(&rows).Error
	if err != nil {
		return Check{
			ID: "processing_queues", Name: "Processing queues", Status: CheckFail,
			Message: err.Error(), DurationMS: time.Since(start).Milliseconds(),
		}
	}

	byQueue := make(map[string]map[string]int64)
	var staleProcessing int64
	for _, r := range rows {
		if byQueue[r.QueueName] == nil {
			byQueue[r.QueueName] = make(map[string]int64)
		}
		byQueue[r.QueueName][r.Status] = r.Count
	}

	// Stale: processing longer than 3h (hardlink/remove timeout).
	if err := db.WithContext(ctx).Raw(`
		SELECT COUNT(*)::bigint FROM processing_queue
		WHERE status = 'processing'
		  AND started_at < NOW() - INTERVAL '3 hours'
	`).Scan(&staleProcessing).Error; err != nil {
		staleProcessing = 0
	}

	status := CheckOK
	msg := "queue table reachable"
	if staleProcessing > 0 {
		status = CheckWarn
		msg = fmt.Sprintf("%d jobs processing > 3h (may need attention)", staleProcessing)
	}

	return Check{
		ID: "processing_queues", Name: "Processing queues", Status: status, Message: msg,
		DurationMS: time.Since(start).Milliseconds(),
		Details: map[string]any{
			"by_queue":         byQueue,
			"stale_processing": staleProcessing,
		},
	}
}

func checkPipeline(ctx context.Context, db *gorm.DB) Check {
	start := time.Now()
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	err := db.WithContext(ctx).Raw(`
		SELECT status, COUNT(*)::bigint AS count
		FROM requests
		WHERE type IN ?
		GROUP BY status
	`, pipeline.AllRequestTypes).Scan(&rows).Error
	if err != nil {
		return Check{
			ID: "request_pipeline", Name: "Request pipeline", Status: CheckFail,
			Message: err.Error(), DurationMS: time.Since(start).Milliseconds(),
		}
	}
	byStatus := make(map[string]int64)
	for _, r := range rows {
		byStatus[r.Status] = r.Count
	}

	status := CheckOK
	msg := "request counts loaded"
	inFlight := byStatus["downloading"] + byStatus["queued"] + byStatus["removing"]
	if inFlight > 0 {
		msg = fmt.Sprintf("%d in-flight download/remove requests", inFlight)
	}

	return Check{
		ID: "request_pipeline", Name: "Request pipeline", Status: status, Message: msg,
		DurationMS: time.Since(start).Milliseconds(),
		Details:    map[string]any{"by_status": byStatus},
	}
}

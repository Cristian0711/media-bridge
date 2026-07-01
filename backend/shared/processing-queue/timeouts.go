package processingqueue

import "time"

// Worker timeout guidance (see docs/requests-hardlink-remove-findings.md):
//   - requests/download: indexer + qBittorrent + DB — tens of minutes
//   - hardlink/remove: large torrent file walks and os.Link — up to hours
const (
	// DefaultWorkerTimeout is used for short routing queues (requests).
	DefaultWorkerTimeout = 10 * time.Minute

	// StandardWorkerTimeout covers download jobs (torrent add + media row).
	StandardWorkerTimeout = 30 * time.Minute

	// LongRunningWorkerTimeout covers hardlink and remove handlers on big libraries.
	LongRunningWorkerTimeout = 3 * time.Hour
)

// LongRunningQueueOptions is for hardlink_processing_queue and remove_processing_queue.
func LongRunningQueueOptions() []Option {
	return []Option{
		WithRetryAfter(60 * time.Second),
		WithMaxAttempts(100),
		WithWorkerTimeout(LongRunningWorkerTimeout),
		WithRecoveryInterval(5 * time.Minute),
	}
}

// StandardQueueOptions is for download_processing_queue.
func StandardQueueOptions() []Option {
	return []Option{
		WithRetryAfter(60 * time.Second),
		WithMaxAttempts(100),
		WithWorkerTimeout(StandardWorkerTimeout),
		WithRecoveryInterval(2 * time.Minute),
	}
}

// RoutingQueueOptions is for requests_processing_queue (enqueue only).
func RoutingQueueOptions() []Option {
	return []Option{
		WithRetryAfter(60 * time.Second),
		WithMaxAttempts(100),
		WithWorkerTimeout(DefaultWorkerTimeout),
	}
}

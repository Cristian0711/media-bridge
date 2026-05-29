package processingqueue

import (
	"fmt"
	"time"
)

// Options controls queue behaviour. All fields have sensible defaults.
type Options struct {
	// MaxAttempts is how many times a job will be retried before being
	// permanently marked as failed. Default: 3.
	MaxAttempts int

	// RetryAfter is the minimum time that must pass since last_processed_at
	// before a failed job becomes eligible for pickup again.
	// 0 means it is eligible immediately after being requeued. Default: 0.
	RetryAfter time.Duration

	// PollInterval is how long a worker waits before polling again when the
	// queue is empty. Default: 2s.
	PollInterval time.Duration

	// WorkerTimeout is the maximum time a job may stay in 'processing'
	// before the recovery loop considers the worker dead and requeues it.
	// Default: 10m.
	WorkerTimeout time.Duration

	// RecoveryInterval is how often the recovery loop runs.
	// Default: 1m.
	RecoveryInterval time.Duration

	// Table is the Postgres table name. Shared across all Queue instances,
	// so every service writes into the same physical table and is isolated
	// only by queue_name. Default: "processing_queue".
	Table string
}

func defaultOptions() Options {
	return Options{
		MaxAttempts:      3,
		RetryAfter:       0,
		PollInterval:     2 * time.Second,
		WorkerTimeout:    10 * time.Minute,
		RecoveryInterval: time.Minute,
		Table:            "processing_queue",
	}
}

type Option func(*Options)

func WithMaxAttempts(n int) Option {
	return func(o *Options) { o.MaxAttempts = n }
}

// WithRetryAfter sets the minimum cooldown between processing attempts.
// A failed job will not be picked up again until at least this duration has
// elapsed since last_processed_at.
// Example: WithRetryAfter(30 * time.Second)
func WithRetryAfter(d time.Duration) Option {
	return func(o *Options) { o.RetryAfter = d }
}

func WithPollInterval(d time.Duration) Option {
	return func(o *Options) { o.PollInterval = d }
}

func WithWorkerTimeout(d time.Duration) Option {
	return func(o *Options) { o.WorkerTimeout = d }
}

func WithRecoveryInterval(d time.Duration) Option {
	return func(o *Options) { o.RecoveryInterval = d }
}

// WithTable overrides the Postgres table name. All Queue instances in your
// application must agree on the same table name.
func WithTable(name string) Option {
	return func(o *Options) { o.Table = name }
}

// fmtInterval converts a Go duration to a Postgres interval literal.
func fmtInterval(d time.Duration) string {
	return fmt.Sprintf("%d microseconds", d.Microseconds())
}

package processingqueue_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

type wPayload struct {
	N int `json:"n"`
}

// pgPool dials TEST_DATABASE_URL or skips the test when it is unset, so the
// default `go test ./...` stays DB-free while CI/local-with-PG runs them.
func pgPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres-backed queue test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("postgres not reachable at TEST_DATABASE_URL: %v", err)
	}
	return pool
}

// newQueue builds an isolated queue on a unique table that is dropped on cleanup.
// It returns the queue and the generated table name.
func newQueue(t *testing.T, pool *pgxpool.Pool, opts ...processingqueue.Option) (*processingqueue.Queue[wPayload], string) {
	t.Helper()
	table := fmt.Sprintf("pq_test_%d", time.Now().UnixNano())
	base := []processingqueue.Option{
		processingqueue.WithTable(table),
		processingqueue.WithPollInterval(25 * time.Millisecond),
	}
	q, err := processingqueue.New[wPayload](pool, "test_queue", append(base, opts...)...)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}
	if err := q.EnsureTable(context.Background()); err != nil {
		t.Fatalf("ensure table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
	})
	return q, table
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

func TestWorker_ProcessesAndCompletes(t *testing.T) {
	pool := pgPool(t)
	defer pool.Close()
	q, _ := newQueue(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processed := make(chan int, 1)
	q.StartWorker(ctx, "w1", func(_ context.Context, job *processingqueue.Job[wPayload]) error {
		processed <- job.Payload.N
		return nil
	})

	if err := q.Enqueue(ctx, wPayload{N: 7}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case got := <-processed:
		if got != 7 {
			t.Fatalf("processed payload %d, want 7", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler was never called")
	}
}

func TestWorker_RetriesThenSucceeds(t *testing.T) {
	pool := pgPool(t)
	defer pool.Close()
	q, _ := newQueue(t, pool, processingqueue.WithMaxAttempts(5))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	attempts := 0
	q.StartWorker(ctx, "w1", func(_ context.Context, _ *processingqueue.Job[wPayload]) error {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < 3 {
			return fmt.Errorf("transient failure %d", n)
		}
		return nil
	})

	if err := q.Enqueue(ctx, wPayload{N: 1}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return attempts >= 3
	}, "job to be retried until success")
}

func TestWorker_PermanentFailureStopsRetrying(t *testing.T) {
	pool := pgPool(t)
	defer pool.Close()
	q, _ := newQueue(t, pool, processingqueue.WithMaxAttempts(5))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	attempts := 0
	q.StartWorker(ctx, "w1", func(_ context.Context, _ *processingqueue.Job[wPayload]) error {
		mu.Lock()
		attempts++
		mu.Unlock()
		return fmt.Errorf("fatal: %w", processingqueue.ErrPermanentFailure)
	})

	if err := q.Enqueue(ctx, wPayload{N: 1}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Give the worker time to (not) retry, then confirm it ran exactly once.
	time.Sleep(1 * time.Second)
	mu.Lock()
	got := attempts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("permanent failure should not retry, attempts = %d", got)
	}
}

// TestWorker_RecoversFromHandlerPanic verifies a panicking handler is turned
// into a normal retryable failure: the worker goroutine survives and the job is
// reprocessed on a later attempt instead of being stuck in 'processing'.
func TestWorker_RecoversFromHandlerPanic(t *testing.T) {
	pool := pgPool(t)
	defer pool.Close()
	q, _ := newQueue(t, pool,
		processingqueue.WithMaxAttempts(5),
		processingqueue.WithRetryAfter(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	attempts := 0
	done := make(chan struct{}, 1)
	q.StartWorker(ctx, "w1", func(_ context.Context, _ *processingqueue.Job[wPayload]) error {
		mu.Lock()
		n := attempts + 1
		attempts = n
		mu.Unlock()
		if n == 1 {
			panic("boom")
		}
		done <- struct{}{}
		return nil
	})

	if err := q.Enqueue(ctx, wPayload{N: 1}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not survive handler panic and reprocess the job")
	}
}

// TestWorker_RecoversStaleJob inserts a job stuck in 'processing' with an old
// lease, then relies on the recovery loop to requeue it so a worker finishes it.
func TestWorker_RecoversStaleJob(t *testing.T) {
	pool := pgPool(t)
	defer pool.Close()
	q, table := newQueue(t, pool,
		processingqueue.WithWorkerTimeout(500*time.Millisecond),
		processingqueue.WithRecoveryInterval(100*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := q.Enqueue(ctx, wPayload{N: 42}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Simulate a dead worker: claim the row and backdate its lease.
	if _, err := pool.Exec(ctx, fmt.Sprintf(
		`UPDATE %s SET status='processing', worker_id='dead', last_processed_at = NOW() - INTERVAL '1 hour', started_at = NOW() - INTERVAL '1 hour'`,
		table,
	)); err != nil {
		t.Fatalf("backdate lease: %v", err)
	}

	done := make(chan struct{}, 1)
	q.StartWorker(ctx, "w1", func(_ context.Context, _ *processingqueue.Job[wPayload]) error {
		done <- struct{}{}
		return nil
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stale job was not recovered and reprocessed")
	}
}

func TestWorker_ConcurrentWorkersProcessEachJobOnce(t *testing.T) {
	pool := pgPool(t)
	defer pool.Close()
	q, _ := newQueue(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const jobs = 30
	var mu sync.Mutex
	seen := map[int]int{}
	handler := func(_ context.Context, job *processingqueue.Job[wPayload]) error {
		mu.Lock()
		seen[job.Payload.N]++
		mu.Unlock()
		return nil
	}
	for i := 0; i < 4; i++ {
		q.StartWorker(ctx, fmt.Sprintf("w%d", i), handler)
	}

	for i := 0; i < jobs; i++ {
		if err := q.Enqueue(ctx, wPayload{N: i}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == jobs
	}, "all jobs to be processed")

	mu.Lock()
	defer mu.Unlock()
	for n, c := range seen {
		if c != 1 {
			t.Errorf("job %d processed %d times, want exactly 1", n, c)
		}
	}
}

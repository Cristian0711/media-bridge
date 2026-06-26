package health

import (
	"context"
	"sync"
	"time"

	"log/slog"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	scheduledQuickInterval = 10 * time.Minute
	scheduledFullInterval  = 1 * time.Hour
)

// Scheduler runs background health scans and persists results.
type Scheduler struct {
	svc        *Service
	repo       Repository
	quickEvery time.Duration
	fullEvery  time.Duration

	mu          sync.Mutex
	running     bool
	lastFullRun time.Time
}

func NewScheduler(svc *Service, repo Repository) *Scheduler {
	return &Scheduler{
		svc:        svc,
		repo:       repo,
		quickEvery: scheduledQuickInterval,
		fullEvery:  scheduledFullInterval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	// Unlike the high-frequency pollers, the health scan is infrequent and worth
	// a coherent trace, so its context is NOT suppressed; runOnce starts a span.
	go s.loop(logger.WithSystem(ctx, "health.scheduler"))
}

func (s *Scheduler) loop(ctx context.Context) {
	log := logger.Component("health.scheduler")
	log.InfoContext(ctx, "health scan scheduler started",
		"quick_interval", s.quickEvery,
		"full_interval", s.fullEvery,
	)

	s.runOnce(ctx, log)

	ticker := time.NewTicker(s.quickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "health scan scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx, log)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context, log *slog.Logger) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		log.DebugContext(ctx, "skip scheduled scan: previous still running")
		return
	}
	s.running = true
	full := time.Since(s.lastFullRun) >= s.fullEvery
	if full {
		s.lastFullRun = time.Now().UTC()
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	// One coherent trace per scan: the report + SaveScan DB queries nest here.
	ctx, span := otel.Tracer("health.scheduler").Start(ctx, "health.scan",
		trace.WithAttributes(attribute.Bool("scan.full", full)))
	defer span.End()

	start := time.Now()
	var report Report
	if full {
		report = s.svc.FullReport(ctx)
	} else {
		report = s.svc.QuickReport(ctx)
	}
	durationMS := time.Since(start).Milliseconds()

	row, err := s.repo.SaveScan(ctx, report, full, durationMS)
	if err != nil {
		log.WarnContext(ctx, "failed to persist health scan", logger.Err(err))
		return
	}
	log.InfoContext(ctx, "scheduled health scan completed",
		"scan_id", row.ID,
		"status", report.Status,
		"full_scan", full,
		"duration_ms", durationMS,
	)
}

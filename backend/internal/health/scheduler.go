package health

import (
	"context"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
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
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	log := logger.Named("health.scheduler")
	log.Info("health scan scheduler started",
		zap.Duration("quick_interval", s.quickEvery),
		zap.Duration("full_interval", s.fullEvery),
	)

	s.runOnce(ctx, log)

	ticker := time.NewTicker(s.quickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("health scan scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx, log)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context, log *zap.Logger) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		log.Debug("skip scheduled scan: previous still running")
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
		log.Warn("failed to persist health scan", zap.Error(err))
		return
	}
	log.Info("scheduled health scan completed",
		zap.Uint("scan_id", row.ID),
		zap.String("status", report.Status),
		zap.Bool("full_scan", full),
		zap.Int64("duration_ms", durationMS),
	)
}

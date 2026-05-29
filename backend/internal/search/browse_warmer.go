package search

import (
	"context"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

const (
	browseWarmInterval   = browseCacheTTL
	browseWarmConcurrent = 4
)

// BrowseWarmer pre-fetches discover data so API requests hit a warm cache.
type BrowseWarmer struct {
	svc *Service
}

func NewBrowseWarmer(svc *Service) *BrowseWarmer {
	return &BrowseWarmer{svc: svc}
}

// Start runs an immediate warm-up, then refreshes on browseWarmInterval.
func (w *BrowseWarmer) Start(ctx context.Context) {
	go func() {
		log := logger.Named("search.browse_warmer")
		log.Info("browse cache warmer started", zap.Duration("interval", browseWarmInterval))

		w.run(ctx, log)

		ticker := time.NewTicker(browseWarmInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("browse cache warmer stopped")
				return
			case <-ticker.C:
				w.run(ctx, log)
			}
		}
	}()
}

func (w *BrowseWarmer) run(ctx context.Context, log *zap.Logger) {
	start := time.Now()

	if _, err := w.svc.warmBrowseServices(ctx); err != nil {
		log.Warn("browse warm: services failed", zap.Error(err))
	}

	tasks := 1 + len(browseServices)
	sem := make(chan struct{}, browseWarmConcurrent)
	var wg sync.WaitGroup
	var okCount, failCount int
	var countMu sync.Mutex

	warm := func(label string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := fn(); err != nil {
				countMu.Lock()
				failCount++
				countMu.Unlock()
				log.Warn("browse warm: catalog failed", zap.String("scope", label), zap.Error(err))
				return
			}
			countMu.Lock()
			okCount++
			countMu.Unlock()
		}()
	}

	warm("global", func() error { return w.svc.warmGlobalCatalog(ctx) })
	for _, svc := range browseServices {
		svc := svc
		warm(svc.ID, func() error { return w.svc.warmServiceCatalog(ctx, svc.ID) })
	}

	wg.Wait()
	log.Info("browse cache warm completed",
		zap.Int("catalogs_ok", okCount),
		zap.Int("catalogs_failed", failCount),
		zap.Int("catalogs_total", tasks),
		zap.Int64("duration_ms", time.Since(start).Milliseconds()),
	)
}

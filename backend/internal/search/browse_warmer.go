package search

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
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
	ctx = logger.WithSystem(ctx, "search.browse_warmer")
	go func() {
		log := logger.Component("search.browse_warmer")
		log.InfoContext(ctx, "browse cache warmer started", "interval", browseWarmInterval)

		w.run(ctx, log)

		ticker := time.NewTicker(browseWarmInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.InfoContext(ctx, "browse cache warmer stopped")
				return
			case <-ticker.C:
				w.run(ctx, log)
			}
		}
	}()
}

func (w *BrowseWarmer) run(ctx context.Context, log *slog.Logger) {
	start := time.Now()

	if _, err := w.svc.warmBrowseServices(ctx); err != nil {
		log.WarnContext(ctx, "browse warm: services failed", logger.Err(err))
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
				log.WarnContext(ctx, "browse warm: catalog failed", "scope", label, logger.Err(err))
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
	log.InfoContext(ctx, "browse cache warm completed",
		"catalogs_ok", okCount,
		"catalogs_failed", failCount,
		"catalogs_total", tasks,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

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
	listIDs := allBrowseWarmListIDs()

	if _, err := w.svc.warmBrowseServices(ctx); err != nil {
		log.Warn("browse warm: services failed", zap.Error(err))
	}

	sem := make(chan struct{}, browseWarmConcurrent)
	var wg sync.WaitGroup
	var okCount, failCount int
	var countMu sync.Mutex

	for _, listID := range listIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := w.svc.warmBrowseListPage(ctx, id, 1); err != nil {
				countMu.Lock()
				failCount++
				countMu.Unlock()
				log.Warn("browse warm: list failed", zap.String("list_id", id), zap.Error(err))
				return
			}
			countMu.Lock()
			okCount++
			countMu.Unlock()
		}(listID)
	}

	wg.Wait()
	log.Info("browse cache warm completed",
		zap.Int("lists_ok", okCount),
		zap.Int("lists_failed", failCount),
		zap.Int("lists_total", len(listIDs)),
		zap.Int64("duration_ms", time.Since(start).Milliseconds()),
	)
}

// allBrowseWarmListIDs returns every discover row loaded on the home page (page 1).
func allBrowseWarmListIDs() []string {
	ids := make([]string, 0, len(browseServices)*len(serviceListKinds)+1)
	ids = append(ids, "trending")
	for _, svc := range browseServices {
		for _, kind := range serviceListKinds {
			ids = append(ids, svc.ID+":"+kind.Suffix)
		}
	}
	return ids
}

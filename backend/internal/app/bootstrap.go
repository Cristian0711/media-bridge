package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/Cristian0711/media-bridge/backend/internal/config"
	"github.com/Cristian0711/media-bridge/backend/internal/download"
	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/health"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer"
	indexersetup "github.com/Cristian0711/media-bridge/backend/internal/indexer/setup"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/internal/remove"
	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	requestssource "github.com/Cristian0711/media-bridge/backend/internal/requests/source"
	"github.com/Cristian0711/media-bridge/backend/internal/search"
	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// searchAvailabilityAdapter lets the search service flag library availability on
// results without depending on the media domain directly.
type searchAvailabilityAdapter struct {
	svc media.Service
}

func (a searchAvailabilityAdapter) Available(ctx context.Context, queries []search.AvailabilityQuery) ([]bool, error) {
	items := make([]media.AvailabilityItem, len(queries))
	for i, q := range queries {
		items[i] = media.AvailabilityItem{
			Type:   q.Type,
			IMDBID: q.IMDB,
			TMDBID: q.TMDB,
			TVDBID: q.TVDB,
		}
	}
	results, err := a.svc.CheckAvailability(ctx, items)
	if err != nil {
		return nil, err
	}
	out := make([]bool, len(results))
	for i := range results {
		out[i] = results[i].Available
	}
	return out, nil
}

func Bootstrap() (*Server, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	db, err := connectDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := users.SeedRuntime(db); err != nil {
		return nil, fmt.Errorf("seed roles: %w", err)
	}

	userRepo := users.NewRepository(db)
	authRepo := auth.NewRepository(db)

	// Root context for all long-lived background workers. Cancelled by
	// Server.Shutdown so queues, watchers, schedulers and the torrent monitor
	// stop cleanly on process exit. On a failed bootstrap the deferred cancel
	// avoids leaking the context; on success ownership passes to the Server.
	ctx, cancel := context.WithCancel(context.Background())
	bootstrapped := false
	defer func() {
		if !bootstrapped {
			cancel()
		}
	}()

	// Single broker for app-wide media + request events (see internal/sse).
	appSSEBroker := sse.NewBroker()
	eventPublisher := sse.NewPublisher(appSSEBroker)

	requestsRepo := requests.NewRepository(db, eventPublisher)
	mediaRepo := media.NewRepository(db)
	mediaSvc := media.NewService(mediaRepo, eventPublisher)

	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	userSvc := users.NewService(userRepo)
	authSvc := auth.NewService(authRepo, userSvc, jwtManager)
	indexerSvc := indexersetup.NewService(cfg.Indexer)
	qbitSvc, err := qbittorrent.NewService(
		cfg.QBittorrent.URL,
		cfg.QBittorrent.Username,
		cfg.QBittorrent.Password,
	)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent: %w", err)
	}
	hardlinkSvc := hardlink.NewService(mediaSvc, qbitSvc, cfg.MoviesPath, cfg.ShowsPath)
	hardlinkProcessor, err := hardlink.NewProcessor(cfg.DatabaseURL, hardlinkSvc, requestsRepo)
	if err != nil {
		return nil, fmt.Errorf("hardlink queue: %w", err)
	}

	downloadProcessor, err := download.NewProcessor(
		cfg.DatabaseURL,
		requestssource.NewDownloadSource(requestsRepo),
		download.NewService(indexerSvc, qbitSvc, cfg.DownloadsPath),
		mediaSvc,
		hardlinkProcessor,
		requestsRepo,
		requestsRepo,
	)

	if err != nil {
		return nil, fmt.Errorf("download queue: %w", err)
	}
	downloadProcessor.Start(ctx, cfg.QueueWorkers.Download)

	removeProcessor, err := remove.NewProcessor(
		cfg.DatabaseURL,
		requestssource.NewRemoveSource(requestsRepo),
		remove.NewService(mediaSvc, qbitSvc, hardlinkProcessor, cfg.MoviesPath, cfg.ShowsPath),
		mediaSvc,
		requestsRepo,
		requestsRepo,
	)
	if err != nil {
		return nil, fmt.Errorf("remove queue: %w", err)
	}
	hardlinkProcessor.SetRemoveGuard(removeProcessor)

	hardlinkProcessor.Start(ctx, cfg.QueueWorkers.Hardlink)
	removeProcessor.Start(ctx, cfg.QueueWorkers.Remove)

	requestsProcessor, err := requests.NewQueueProcessor(cfg.DatabaseURL, requestsRepo, downloadProcessor, removeProcessor)
	if err != nil {
		return nil, fmt.Errorf("requests queue: %w", err)
	}
	requestsProcessor.Start(ctx, cfg.QueueWorkers.Requests)
	requests.NewReconciler(requestsRepo, requestsProcessor, downloadProcessor, removeProcessor).Start(ctx)
	requests.NewDownloadCompletionWatcher(
		requestsRepo,
		hardlinkSvc,
		qbitSvc,
		5*time.Second,
	).Start(ctx)
	requestsSvc := requests.NewService(requestsRepo, mediaRepo, requestsProcessor, mediaSvc, qbitSvc, hardlinkSvc)
	qbitBroker := qbittorrent.NewBroker()
	qbittorrent.StartTorrentMonitor(
		ctx,
		qbitSvc,
		qbitBroker,
		2*time.Second,
	)

	searchSvc := search.NewService(search.TMDBConfig{
		BaseURL: cfg.TMDB.URL,
		APIKey:  cfg.TMDB.APIKey,
	})
	searchSvc.SetAvailabilityChecker(searchAvailabilityAdapter{svc: mediaSvc})
	search.NewBrowseWarmer(searchSvc).Start(ctx)

	healthRepo := health.NewRepository(db)
	healthCfg := health.Config{
		MoviesPath:    cfg.MoviesPath,
		ShowsPath:     cfg.ShowsPath,
		DownloadsPath: cfg.DownloadsPath,
	}
	healthSvc := health.NewService(db, healthRepo, mediaSvc, qbitSvc, healthCfg)
	health.NewScheduler(healthSvc, healthRepo).Start(ctx)

	router := newRouter(
		auth.NewHandler(authSvc),
		users.NewHandler(userSvc),
		qbittorrent.NewHandler(qbitSvc, qbitBroker),
		indexer.NewHandler(indexerSvc),
		media.NewHandler(mediaSvc),
		requests.NewHandler(requestsSvc),
		search.NewHandler(searchSvc),
		sse.NewHandler(appSSEBroker),
		health.NewHandler(healthSvc),
	)

	bootstrapped = true
	return newServer(cfg.Port, router, cancel, appSSEBroker.Shutdown, qbitBroker.Shutdown), nil
}

func connectDB(cfg *config.AppConfig) (*gorm.DB, error) {
	return gorm.Open(
		postgres.Open(cfg.DatabaseURL),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
}

func migrate(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		return err
	}

	if err := db.AutoMigrate(
		&users.RoleRecord{},
		&users.User{},
		&auth.Key{},
		&requests.Request{},
		&health.ScanLog{},
		&media.Movie{},
		&media.Show{},
		&media.ShowEntry{},
		&media.Media{},
	); err != nil {
		return err
	}

	if err := dropMediaSoftDelete(db); err != nil {
		return err
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_media_name_trgm ON media USING gin (name gin_trgm_ops)").Error
}

// dropMediaSoftDelete converts media and show_entries from soft-delete to hard
// delete. Any previously soft-deleted rows are purged first so that dropping the
// column cannot resurrect them, then the deleted_at column (and its index) is
// removed. Idempotent: a no-op once the column is gone.
func dropMediaSoftDelete(db *gorm.DB) error {
	targets := []struct {
		model any
		table string
	}{
		{&media.Media{}, "media"},
		{&media.ShowEntry{}, "show_entries"},
	}
	for _, t := range targets {
		if !db.Migrator().HasColumn(t.model, "deleted_at") {
			continue
		}
		if err := db.Exec("DELETE FROM " + t.table + " WHERE deleted_at IS NOT NULL").Error; err != nil {
			return err
		}
		if err := db.Migrator().DropColumn(t.model, "deleted_at"); err != nil {
			return err
		}
	}
	return nil
}

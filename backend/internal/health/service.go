package health

import (
	"context"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"gorm.io/gorm"
)

// Config holds paths and limits for diagnostics.
type Config struct {
	MoviesPath    string
	ShowsPath     string
	DownloadsPath string
}

// Service runs health and filesystem audits.
type Service struct {
	db       *gorm.DB
	repo     Repository
	mediaSvc media.Service
	qbitSvc  qbittorrent.Service
	cfg      Config
}

func NewService(db *gorm.DB, repo Repository, mediaSvc media.Service, qbitSvc qbittorrent.Service, cfg Config) *Service {
	return &Service{db: db, repo: repo, mediaSvc: mediaSvc, qbitSvc: qbitSvc, cfg: cfg}
}

func (s *Service) ListScans(ctx context.Context, page, pageSize int) (*PaginatedScanLogsResponse, error) {
	scans, total, err := s.repo.ListScans(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedScanLogsResponse{
		Scans:      scans,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		TotalPages: totalPages(total, pageSize),
	}, nil
}

func (s *Service) GetScan(ctx context.Context, id uint) (*ScanLog, error) {
	return s.repo.GetScan(ctx, id)
}

func (s *Service) LatestScan(ctx context.Context) (*ScanLogSummary, error) {
	return s.repo.LatestScan(ctx)
}

// RunAndPersist executes a scan, stores it in the log, and returns the report.
func (s *Service) RunAndPersist(ctx context.Context, full bool) (Report, *ScanLogSummary, error) {
	start := time.Now()
	var report Report
	if full {
		report = s.FullReport(ctx)
	} else {
		report = s.QuickReport(ctx)
	}
	durationMS := time.Since(start).Milliseconds()
	row, err := s.repo.SaveScan(ctx, report, full, durationMS)
	if err != nil {
		return report, nil, err
	}
	sum := scanLogToSummary(row)
	return report, &sum, nil
}

// QuickReport runs fast checks (no filesystem walk).
func (s *Service) QuickReport(ctx context.Context) Report {
	return s.run(ctx, false)
}

// FullReport includes filesystem hardlink audits (can take a while on large libraries).
func (s *Service) FullReport(ctx context.Context) Report {
	return s.run(ctx, true)
}

func (s *Service) run(ctx context.Context, includeFS bool) Report {
	checkedAt := time.Now().UTC()
	checks := []Check{
		s.checkAPI(),
		s.checkDatabase(ctx),
	}

	torrentByHash, qbitCheck := s.checkQBittorrentWithTorrents(ctx)
	checks = append(checks, qbitCheck)
	checks = append(checks,
		s.checkMediaTorrentRegistry(ctx, torrentByHash),
		checkQueues(ctx, s.db),
		checkPipeline(ctx, s.db),
	)

	if includeFS {
		fsCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		checks = append(checks, s.checkFilesystem(fsCtx)...)
	}

	status := aggregateStatus(checks)
	return Report{Status: status, CheckedAt: checkedAt, Checks: checks}
}

func (s *Service) checkAPI() Check {
	return Check{
		ID: "api", Name: "API", Status: CheckOK,
		Message: "handler responding",
	}
}

func (s *Service) checkDatabase(ctx context.Context) Check {
	start := time.Now()
	sqlDB, err := s.db.DB()
	if err != nil {
		return Check{
			ID: "database", Name: "Database", Status: CheckFail,
			Message: err.Error(), DurationMS: time.Since(start).Milliseconds(),
		}
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return Check{
			ID: "database", Name: "Database", Status: CheckFail,
			Message: err.Error(), DurationMS: time.Since(start).Milliseconds(),
		}
	}
	return Check{
		ID: "database", Name: "Database", Status: CheckOK,
		Message: "connected", DurationMS: time.Since(start).Milliseconds(),
	}
}

func (s *Service) checkQBittorrentWithTorrents(ctx context.Context) (map[string]qbittorrent.Torrent, Check) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	byHash, err := s.qbitSvc.TorrentsByHash(ctx)
	if err != nil {
		return nil, Check{
			ID: "qbittorrent", Name: "qBittorrent", Status: CheckFail,
			Message: err.Error(), DurationMS: time.Since(start).Milliseconds(),
		}
	}
	return byHash, Check{
		ID: "qbittorrent", Name: "qBittorrent", Status: CheckOK,
		Message: fmt.Sprintf("reachable (%d plexmedia torrents)", len(byHash)),
		DurationMS: time.Since(start).Milliseconds(),
		Details:    map[string]any{"torrent_count": len(byHash)},
	}
}

func (s *Service) checkFilesystem(ctx context.Context) []Check {
	start := time.Now()
	ex, err := collectExclusions(ctx, s.db, s.mediaSvc)
	if err != nil {
		return []Check{{
			ID: "fs_exclusions", Name: "Filesystem exclusions", Status: CheckFail,
			Message: err.Error(), DurationMS: time.Since(start).Milliseconds(),
		}}
	}

	libraryRoot := s.cfg.MoviesPath
	showsRoot := s.cfg.ShowsPath
	downloadsRoot := s.cfg.DownloadsPath

	moviesAudit := auditRoot(ctx, libraryRoot, "library_movies", ex)
	showsAudit := auditRoot(ctx, showsRoot, "library_shows", ex)
	downloadsAudit := auditRoot(ctx, downloadsRoot, "downloads", ex)

	libMovies := fsResultToCheck("fs_movies_hardlinks", "Movies library hardlinks", moviesAudit)
	libShows := fsResultToCheck("fs_shows_hardlinks", "Shows library hardlinks", showsAudit)
	dlCheck := fsResultToCheck("fs_download_hardlinks", "Downloads folder hardlinks", downloadsAudit)

	meta := Check{
		ID: "fs_exclusions", Name: "In-flight path exclusions", Status: CheckOK,
		Message: fmt.Sprintf("%d media rows excluded from nlink audit", ex.InFlightMedia),
		DurationMS: time.Since(start).Milliseconds(),
		Details: map[string]any{
			"in_flight_media":  ex.InFlightMedia,
			"prefix_count":     len(ex.Prefixes),
			"by_reason":        ex.ByReason,
			"removing_dest":    len(ex.RemovingDest),
		},
	}
	return []Check{meta, libMovies, libShows, dlCheck}
}

func aggregateStatus(checks []Check) string {
	hasFail := false
	hasWarn := false
	for _, c := range checks {
		switch c.Status {
		case CheckFail:
			hasFail = true
		case CheckWarn:
			hasWarn = true
		}
	}
	if hasFail {
		return StatusUnhealthy
	}
	if hasWarn {
		return StatusDegraded
	}
	return StatusHealthy
}

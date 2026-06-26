package requests

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
)

// DownloadCompletionWatcher polls in-flight download requests and marks them
// 'downloaded' only when both the library hardlinks exist and the torrent has
// finished downloading in qBittorrent.
type DownloadCompletionWatcher struct {
	repo        Repository
	hardlinkSvc hardlink.Service
	qbitSvc     qbittorrent.Service
	interval    time.Duration

	tickMu   sync.Mutex
	tickBusy bool
}

func NewDownloadCompletionWatcher(
	repo Repository,
	hardlinkSvc hardlink.Service,
	qbitSvc qbittorrent.Service,
	interval time.Duration,
) *DownloadCompletionWatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &DownloadCompletionWatcher{
		repo:        repo,
		hardlinkSvc: hardlinkSvc,
		qbitSvc:     qbitSvc,
		interval:    interval,
	}
}

func (w *DownloadCompletionWatcher) Start(ctx context.Context) {
	go w.run(logger.WithSystem(ctx, "requests.download_watcher"))
}

func (w *DownloadCompletionWatcher) run(ctx context.Context) {
	log := logger.Component("requests.download_watcher")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.InfoContext(ctx, "download completion watcher started", "interval", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "download completion watcher stopped")
			return
		case <-ticker.C:
			w.tick(ctx, log)
		}
	}
}

func (w *DownloadCompletionWatcher) tick(ctx context.Context, log *slog.Logger) {
	w.tickMu.Lock()
	if w.tickBusy {
		w.tickMu.Unlock()
		return
	}
	w.tickBusy = true
	w.tickMu.Unlock()

	defer func() {
		w.tickMu.Lock()
		w.tickBusy = false
		w.tickMu.Unlock()
	}()

	rows, err := w.repo.ListDownloading(ctx)
	if err != nil {
		log.WarnContext(ctx, "list downloading requests failed", logger.Err(err))
		return
	}
	if len(rows) == 0 {
		return
	}

	torrentByHash, err := w.qbitSvc.TorrentsByHash(ctx)
	if err != nil {
		log.WarnContext(ctx, "list qbittorrent torrents failed", logger.Err(err))
		return
	}

	type fileCheck struct {
		req  *Request
		hash string
	}

	var needFileCheck []string
	var checks []fileCheck

	for i := range rows {
		req := &rows[i]
		if req.MediaID == 0 {
			continue
		}

		progress, err := w.hardlinkSvc.Progress(ctx, req.MediaID)
		if err != nil {
			log.DebugContext(ctx, "hardlink progress check failed",
				"request_entry_id", req.ID,
				"media_id", req.MediaID,
				logger.Err(err),
			)
			continue
		}
		if !progress.Complete {
			continue
		}

		hash := qbittorrent.NormalizeHash(progress.TorrentHash)
		if hash == "" {
			w.finalizeIfStillLinked(ctx, log, req, hash)
			continue
		}

		torrent, ok := torrentByHash[hash]
		if !ok {
			// Torrent absent from qBittorrent is ambiguous: it can mean
			// "finished and cleaned up" or "a remove is tearing it down right
			// now". Re-verify the hardlinks before finalizing so we don't flip
			// a request being removed back to 'downloaded' (H1).
			w.finalizeIfStillLinked(ctx, log, req, hash)
			continue
		}

		if !qbittorrent.TorrentTransferComplete(torrent) {
			continue
		}

		needFileCheck = append(needFileCheck, hash)
		checks = append(checks, fileCheck{req: req, hash: hash})
	}

	filesComplete, err := w.qbitSvc.FilesCompleteByHash(ctx, needFileCheck, nil)
	if err != nil {
		log.WarnContext(ctx, "batch torrent file completion check failed", logger.Err(err))
		return
	}

	for _, c := range checks {
		if !filesComplete[c.hash] {
			continue
		}
		if err := w.finalizeDownloaded(ctx, log, c.req, c.hash); err != nil {
			log.DebugContext(ctx, "download finalize check failed",
				"request_entry_id", c.req.ID,
				"media_id", c.req.MediaID,
				logger.Err(err),
			)
		}
	}
}

// finalizeIfStillLinked finalizes a download only after re-confirming the
// library hardlinks are still complete on disk. It guards the "torrent absent
// from qBittorrent" paths, where absence alone is not a reliable completion
// signal: a concurrent remove may be deleting the files. Re-checking right
// before the state transition (together with MarkDownloadedIfDownloading's
// status guard) prevents resurrecting a download that is being removed (H1).
func (w *DownloadCompletionWatcher) finalizeIfStillLinked(
	ctx context.Context,
	log *slog.Logger,
	req *Request,
	hash string,
) {
	progress, err := w.hardlinkSvc.Progress(ctx, req.MediaID)
	if err != nil {
		log.DebugContext(ctx, "re-verify hardlinks before finalize failed",
			"request_entry_id", req.ID,
			"media_id", req.MediaID,
			logger.Err(err),
		)
		return
	}
	if progress.Total == 0 || !progress.Complete {
		log.DebugContext(ctx, "skip finalize: hardlinks no longer complete",
			"request_entry_id", req.ID,
			"media_id", req.MediaID,
		)
		return
	}
	if err := w.finalizeDownloaded(ctx, log, req, hash); err != nil {
		log.DebugContext(ctx, "download finalize failed", "request_entry_id", req.ID, logger.Err(err))
	}
}

func (w *DownloadCompletionWatcher) finalizeDownloaded(
	ctx context.Context,
	log *slog.Logger,
	req *Request,
	torrentHash string,
) error {
	updated, err := w.repo.MarkDownloadedIfDownloading(ctx, req.ID)
	if err != nil {
		return err
	}
	if !updated {
		log.DebugContext(ctx, "skip finalize: request no longer downloading",
			"request_entry_id", req.ID,
			"media_id", req.MediaID,
		)
		return nil
	}
	log.InfoContext(ctx, "download request finalized",
		"request_entry_id", req.ID,
		"media_id", req.MediaID,
		"torrent_hash", torrentHash,
	)
	return nil
}

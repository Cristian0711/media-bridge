package requests

import (
	"context"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
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
	go w.run(ctx)
}

func (w *DownloadCompletionWatcher) run(ctx context.Context) {
	log := logger.Named("requests.download_watcher")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Info("download completion watcher started", zap.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			log.Info("download completion watcher stopped")
			return
		case <-ticker.C:
			w.tick(ctx, log)
		}
	}
}

func (w *DownloadCompletionWatcher) tick(ctx context.Context, log *zap.Logger) {
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
		log.Warn("list downloading requests failed", zap.Error(err))
		return
	}
	if len(rows) == 0 {
		return
	}

	torrentByHash, err := w.qbitSvc.TorrentsByHash(ctx)
	if err != nil {
		log.Warn("list qbittorrent torrents failed", zap.Error(err))
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
			log.Debug("hardlink progress check failed",
				zap.Uint("request_entry_id", req.ID),
				zap.Uint("media_id", req.MediaID),
				zap.Error(err),
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
		log.Warn("batch torrent file completion check failed", zap.Error(err))
		return
	}

	for _, c := range checks {
		if !filesComplete[c.hash] {
			continue
		}
		if err := w.finalizeDownloaded(ctx, log, c.req, c.hash); err != nil {
			log.Debug("download finalize check failed",
				zap.Uint("request_entry_id", c.req.ID),
				zap.Uint("media_id", c.req.MediaID),
				zap.Error(err),
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
	log *zap.Logger,
	req *Request,
	hash string,
) {
	progress, err := w.hardlinkSvc.Progress(ctx, req.MediaID)
	if err != nil {
		log.Debug("re-verify hardlinks before finalize failed",
			zap.Uint("request_entry_id", req.ID),
			zap.Uint("media_id", req.MediaID),
			zap.Error(err),
		)
		return
	}
	if progress.Total == 0 || !progress.Complete {
		log.Debug("skip finalize: hardlinks no longer complete",
			zap.Uint("request_entry_id", req.ID),
			zap.Uint("media_id", req.MediaID),
		)
		return
	}
	if err := w.finalizeDownloaded(ctx, log, req, hash); err != nil {
		log.Debug("download finalize failed", zap.Uint("request_entry_id", req.ID), zap.Error(err))
	}
}

func (w *DownloadCompletionWatcher) finalizeDownloaded(
	ctx context.Context,
	log *zap.Logger,
	req *Request,
	torrentHash string,
) error {
	updated, err := w.repo.MarkDownloadedIfDownloading(ctx, req.ID)
	if err != nil {
		return err
	}
	if !updated {
		log.Debug("skip finalize: request no longer downloading",
			zap.Uint("request_entry_id", req.ID),
			zap.Uint("media_id", req.MediaID),
		)
		return nil
	}
	log.Info("download request finalized",
		zap.Uint("request_entry_id", req.ID),
		zap.Uint("media_id", req.MediaID),
		zap.String("torrent_hash", torrentHash),
	)
	return nil
}

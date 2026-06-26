package hardlink

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
	"go.uber.org/zap"
)

type Service interface {
	Hardlink(ctx context.Context, mediaID uint) error
	// IsComplete reports whether every torrent file is hardlinked into the library path.
	IsComplete(ctx context.Context, mediaID uint) (bool, error)
	// Progress returns per-file hardlink status (done vs remaining).
	Progress(ctx context.Context, mediaID uint) (*Progress, error)
	// ProgressForMedia is like Progress but uses an already-loaded media row.
	ProgressForMedia(ctx context.Context, row *media.Media) (*Progress, error)
}

type service struct {
	mediaService media.Service
	qbitService  qbittorrent.Service
	moviesPath   string
	showsPath    string
}

func NewService(mediaService media.Service, qbitService qbittorrent.Service, moviesPath, showsPath string) Service {
	return withProgressCache(&service{
		mediaService: mediaService,
		qbitService:  qbitService,
		moviesPath:   moviesPath,
		showsPath:    showsPath,
	}, defaultProgressCacheTTL)
}

func (s *service) IsComplete(ctx context.Context, mediaID uint) (bool, error) {
	p, err := s.Progress(ctx, mediaID)
	if err != nil {
		return false, err
	}
	return p.Complete, nil
}

func (s *service) Hardlink(ctx context.Context, mediaID uint) error {
	row, err := s.mediaService.GetMediaByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			return fmt.Errorf("media %d not found, cancelling hardlink: %w", mediaID, processingqueue.ErrPermanentFailure)
		}
		return fmt.Errorf("load media %d: %w", mediaID, err)
	}

	switch row.Type {
	case media.MediaTypeMovie:
		return s.hardlinkMovie(ctx, row)
	case media.MediaTypeShowFull, media.MediaTypeShowSeason, media.MediaTypeShowEpisode:
		return s.hardlinkShow(ctx, row)
	default:
		return fmt.Errorf("unsupported media type for hardlink: %s", row.Type)
	}
}

func (s *service) hardlinkMovie(ctx context.Context, row *media.Media) error {
	torrentHash, savePath, destinationPath, err := s.movieHardlinkPaths(row)
	if err != nil {
		return err
	}
	return s.runHardlink(ctx, row.ID, torrentHash, savePath, destinationPath, s.moviesPath)
}

func (s *service) hardlinkShow(ctx context.Context, row *media.Media) error {
	torrentHash, savePath, destinationPath, err := s.showHardlinkPaths(row)
	if err != nil {
		return err
	}
	return s.runHardlink(ctx, row.ID, torrentHash, savePath, destinationPath, s.showsPath)
}

func (s *service) movieHardlinkPaths(row *media.Media) (torrentHash, savePath, destinationPath string, err error) {
	if row.Movie == nil {
		return "", "", "", fmt.Errorf("media %d has no linked movie", row.ID)
	}
	savePath, torrentHash, err = requireSavePathAndHash(row.Movie.SavePath, row.Movie.TorrentHash, row.ID)
	if err != nil {
		return "", "", "", err
	}
	folderName := fmt.Sprintf("%s (%s) (%s)", row.Name, row.Movie.IMDBID, row.Quality)
	destinationPath = filepath.Join(s.moviesPath, folderName)
	return torrentHash, savePath, destinationPath, nil
}

func (s *service) showHardlinkPaths(row *media.Media) (torrentHash, savePath, destinationPath string, err error) {
	if row.ShowEntry == nil || row.ShowEntry.Show == nil {
		return "", "", "", fmt.Errorf("media %d has no linked show entry", row.ID)
	}
	savePath, torrentHash, err = requireSavePathAndHash(row.ShowEntry.SavePath, row.ShowEntry.TorrentHash, row.ID)
	if err != nil {
		return "", "", "", err
	}
	show := row.ShowEntry.Show
	showFolder := fmt.Sprintf("%s (%s) (%s)", show.Name, show.IMDBID, row.Quality)
	destinationPath = filepath.Join(s.showsPath, showFolder)
	if row.ShowEntry.Season != nil {
		destinationPath = filepath.Join(destinationPath, fmt.Sprintf("Season %d", *row.ShowEntry.Season))
	}
	return torrentHash, savePath, destinationPath, nil
}

func (s *service) runHardlink(ctx context.Context, mediaID uint, torrentHash, savePath, destinationPath, basePath string) error {
	log := logger.Named("hardlink").With(
		zap.Uint("media_id", mediaID),
		zap.String("torrent_hash", torrentHash),
	)

	files, err := s.qbitService.GetTorrentFiles(ctx, torrentHash)
	if err != nil {
		return fmt.Errorf("get torrent files: %w", err)
	}
	if len(files) == 0 {
		if err := s.errIfTorrentStillDownloading(ctx, torrentHash, "torrent files not ready yet"); err != nil {
			return err
		}
		return fmt.Errorf("no files reported by qbittorrent for hash %s yet", torrentHash)
	}

	if err := os.MkdirAll(destinationPath, 0755); err != nil {
		return fmt.Errorf("create destination dir %s: %w", destinationPath, err)
	}

	linked := 0
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		sourcePath := buildSourcePath(savePath, file.Name)
		destPath := filepath.Join(destinationPath, normalizeTorrentPath(file.Name))

		if hardlinkPresent(sourcePath, destPath) {
			linked++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			log.Warn("create dest dir failed", zap.String("dir", filepath.Dir(destPath)), zap.Error(err))
			continue
		}

		linkErr := os.Link(sourcePath, destPath)
		if linkErr == nil {
			linked++
			continue
		}
		if isCrossDeviceLinkErrno(linkErr) {
			return crossDeviceLinkError(sourcePath, destPath, linkErr)
		}
		if !os.IsExist(linkErr) {
			// Source likely not yet downloaded yet — retry via queue.
			log.Debug("os.Link failed", zap.String("source", sourcePath), zap.String("dest", destPath), zap.Error(linkErr))
			continue
		}

		// EEXIST: a file is already at destPath. Trust it only if it's
		// actually a hardlink to the same inode as our source — otherwise
		// it's stale (e.g. from a previous aborted run with different
		// content) and silently counting it would publish the wrong file
		// to Plex. Delete and relink in that case.
		if sameInode(sourcePath, destPath) {
			linked++
			continue
		}
		log.Warn("dest file exists with different inode, replacing",
			zap.String("source", sourcePath),
			zap.String("dest", destPath),
		)
		if rerr := os.Remove(destPath); rerr != nil && !os.IsNotExist(rerr) {
			log.Warn("could not remove stale dest before relink",
				zap.String("dest", destPath), zap.Error(rerr))
			continue
		}
		if rerr := os.Link(sourcePath, destPath); rerr != nil {
			if isCrossDeviceLinkErrno(rerr) {
				return crossDeviceLinkError(sourcePath, destPath, rerr)
			}
			log.Warn("relink after stale removal failed",
				zap.String("source", sourcePath),
				zap.String("dest", destPath),
				zap.Error(rerr))
			continue
		}
		linked++
	}

	total := len(files)
	remaining := total - linked
	if remaining > 0 {
		log.Info("hardlink progress",
			zap.Int("remaining", remaining),
			zap.Int("total", total),
		)
		if err := s.errIfTorrentStillDownloading(ctx, torrentHash,
			fmt.Sprintf("hardlinking in progress: %d remaining from %d", remaining, total)); err != nil {
			return err
		}
		return fmt.Errorf("hardlinking in progress: %d remaining from %d", remaining, total)
	}

	log.Info("hardlink completed",
		zap.Int("total", total),
	)
	if err := s.mediaService.UpdateLibraryPath(ctx, mediaID, destinationPath); err != nil {
		log.Warn("persist library path failed", zap.Error(err))
	}
	return nil
}

// errIfTorrentStillDownloading returns ErrDeferRetry when qBittorrent reports the
// torrent is not finished yet, so the queue can requeue without burning attempts.
func (s *service) errIfTorrentStillDownloading(ctx context.Context, torrentHash, msg string) error {
	t, err := s.qbitService.GetTorrent(ctx, torrentHash)
	if err != nil {
		// Transient qBittorrent failure (outage, timeout): defer rather than
		// returning nil. Returning nil lets the caller surface a plain error
		// that counts toward MaxAttempts, so a brief qBit blip could mark a
		// still-downloading request 'failed'. Deferring requeues the job
		// without consuming a real attempt.
		return fmt.Errorf("%s: checking torrent status: %w", msg, processingqueue.ErrDeferRetry)
	}
	if qbittorrent.TorrentTransferComplete(*t) {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, processingqueue.ErrDeferRetry)
}

func requireSavePathAndHash(savePath, torrentHash *string, mediaID uint) (string, string, error) {
	if savePath == nil || *savePath == "" {
		return "", "", fmt.Errorf("media %d has no save_path yet", mediaID)
	}
	if torrentHash == nil || *torrentHash == "" {
		return "", "", fmt.Errorf("media %d has no torrent_hash yet", mediaID)
	}
	return *savePath, qbittorrent.NormalizeHash(*torrentHash), nil
}

// normalizeTorrentPath strips the root folder from a torrent file path.
//
//	"TorrentFolder/Episode.mkv"      -> "Episode.mkv"
//	"TorrentFolder/Subs/English.srt" -> "Subs/English.srt"
//	"Episode.mkv"                    -> "Episode.mkv"
func normalizeTorrentPath(filePath string) string {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	if len(parts) > 1 {
		return filepath.Join(parts[1:]...)
	}
	return filePath
}

func buildSourcePath(savePath, filePath string) string {
	return filepath.Join(savePath, normalizeTorrentPath(filePath))
}

// hardlinkPresent is true when source exists and dest is the same inode (already
// linked). It stats the source once and reuses it — this runs per file on every
// progress poll, so the extra stat that delegating to sameInode would incur is
// worth avoiding.
func hardlinkPresent(sourcePath, destPath string) bool {
	si, err := os.Lstat(sourcePath)
	if err != nil {
		return false
	}
	di, err := os.Lstat(destPath)
	if err != nil {
		return false
	}
	return os.SameFile(si, di)
}

// sameInode reports whether the two paths reference the same underlying inode
// (i.e. they are hardlinks). Uses os.Lstat (not os.Stat) so a symlinked source
// is compared as the symlink itself rather than its target — otherwise a
// symlink pointing at an unrelated file could report a false inode match and
// the real hardlink would never be created. Compares (device, inode) via
// os.SameFile. Any stat error returns false — the caller treats that as
// "different" and rebuilds the link.
func sameInode(a, b string) bool {
	ai, err := os.Lstat(a)
	if err != nil {
		return false
	}
	bi, err := os.Lstat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

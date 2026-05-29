// Package remove deletes a media item from disk and from qBittorrent.
//
// A media row owns up to three things on disk:
//
//   - Source files in /downloads (managed by qBittorrent).
//   - One hardlink per source file under /movies or /shows (created by the
//     hardlink package — these are what Plex sees).
//   - A torrent entry in qBittorrent.
//
// Process() removes all three, in this order:
//
//  1. Cancel any pending/processing hardlink job for this media, so a racing
//     hardlink worker cannot recreate links the moment we delete them.
//  2. Load the media row. The Movie.SavePath / ShowEntry.SavePath column is
//     the source of truth for where files live in /downloads. The folder
//     layout under /movies or /shows is derived from the media metadata,
//     matching what the hardlink package built.
//  3. Tell qBittorrent to forget the torrent without deleting files — disk
//     cleanup is our job. We do this *first* so qBittorrent closes its
//     file descriptors before we unlink anything; otherwise an active
//     writer would keep the inode alive after we deleted the dirent and
//     could still write to disk.
//  4. Index savePath: collect each regular file's (device, inode). Hardlinks
//     share that pair with their source, so this set identifies every file
//     this torrent linked into /movies or /shows.
//  5. Walk destPath and delete every file whose (device, inode) is in the
//     set above (the hardlinks). ENOENT is silently ignored throughout.
//  6. Walk savePath and delete every regular file underneath it.
//  7. Walk destPath a second time using the same inode set. This catches
//     hardlinks created by a hardlink worker that was already past its
//     queue-status check when we cancelled it: it can keep calling
//     os.Link until the source files disappear in step 6. Once a source
//     is unlinked the inode is preserved by the surviving hardlink, so
//     the inode set is still the right key to find the orphan.
//  8. Verify savePath has no regular files left. If anything remains the
//     function returns an error and the queue retries — only when this
//     check passes does the outer worker proceed to delete the media row.
//  9. Best-effort cleanup of empty source and destination directories.
//
// Returning a non-nil error from Process() requeues the job (up to 100 attempts
// at 60s spacing). The media DB row is deleted by the queue's outer worker
// *after* Process returns nil, so retries keep observing the same state.
package remove

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

type RequestDetails struct {
	RequestEntryID uint
	RequestID      string
	MediaID        uint
	Type           string
	UserID         uint
	Username       string
}

// HardlinkCanceler cancels pending/processing hardlink jobs for a media row
// so they cannot run during deletion.
type HardlinkCanceler interface {
	CancelByMediaID(ctx context.Context, mediaID uint) error
}

type Service interface {
	Process(ctx context.Context, request RequestDetails) error
}

type service struct {
	mediaService     media.Service
	qbitService      qbittorrent.Service
	hardlinkCanceler HardlinkCanceler
	moviesPath       string
	showsPath        string
}

func NewService(
	mediaService media.Service,
	qbitService qbittorrent.Service,
	hardlinkCanceler HardlinkCanceler,
	moviesPath, showsPath string,
) Service {
	return &service{
		mediaService:     mediaService,
		qbitService:      qbitService,
		hardlinkCanceler: hardlinkCanceler,
		moviesPath:       moviesPath,
		showsPath:        showsPath,
	}
}

func (s *service) Process(ctx context.Context, request RequestDetails) error {
	log := logger.Named("remove").With(
		zap.Uint("media_id", request.MediaID),
		zap.String("request_id", request.RequestID),
		zap.String("type", request.Type),
	)

	if request.MediaID == 0 {
		log.Warn("remove called with media_id=0, nothing to do")
		return nil
	}

	// (1) Stop the hardlink processor from racing us. Pending jobs become
	// 'failed'; an already-running worker will finish but the upcoming
	// destination-file deletion will catch anything it linked, and any
	// further retries will see the media row gone (permanent failure).
	if s.hardlinkCanceler != nil {
		if err := s.hardlinkCanceler.CancelByMediaID(ctx, request.MediaID); err != nil {
			// Non-fatal — if we can't reach the queue we still try to delete.
			log.Warn("cancel hardlink jobs failed (continuing)", zap.Error(err))
		}
	}

	// (2) Load media. If it's already gone, the request has succeeded.
	row, err := s.mediaService.GetMediaByID(ctx, request.MediaID)
	if err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			log.Info("media row already gone, treating as success")
			return nil
		}
		return fmt.Errorf("load media %d: %w", request.MediaID, err)
	}

	paths, err := s.resolvePaths(row)
	if err != nil {
		return fmt.Errorf("resolve paths for media %d: %w", request.MediaID, err)
	}
	log = log.With(
		zap.String("save_path", paths.SavePath),
		zap.String("torrent_hash", paths.TorrentHash),
		zap.String("dest_path", paths.DestPath),
	)

	// (3) Forget the torrent in qBittorrent first, without touching files.
	// This closes qBittorrent's file descriptors so the upcoming unlinks
	// actually free disk space immediately instead of leaving ghost
	// inodes behind a still-open writer.
	if paths.TorrentHash != "" {
		if err := s.qbitService.RemoveTorrent(ctx, paths.TorrentHash); err != nil {
			if errors.Is(err, qbittorrent.ErrTorrentNotFound) {
				log.Info("torrent already absent from qbittorrent")
			} else {
				// Surface this as a retry — if qbit still has the torrent
				// open we don't want to start unlinking files underneath it.
				return fmt.Errorf("remove torrent from qbittorrent: %w", err)
			}
		}
	}

	// (4) Index savePath: (device, inode) for every regular file. The pair
	// identifies hardlinks regardless of name or location.
	var sources []sourceFile
	if paths.SavePath != "" {
		sources, err = indexSources(ctx, log, paths.SavePath)
		if err != nil {
			return fmt.Errorf("index save path: %w", err)
		}
	}

	// Build the inode lookup once — used for both destPath walks below.
	var keys map[fileKey]string
	if len(sources) > 0 {
		keys = make(map[fileKey]string, len(sources))
		for _, src := range sources {
			keys[src.key] = src.path
		}
	}

	// (5) Walk destPath and delete every file whose inode is in the set.
	hardlinksRemoved := 0
	if len(keys) > 0 && paths.DestPath != "" {
		hardlinksRemoved = removeHardlinksByInode(ctx, log, paths.DestPath, keys)
	}

	// (6) Walk savePath and delete every regular file underneath it.
	sourcesRemoved := 0
	if paths.SavePath != "" {
		sourcesRemoved = removeAllRegularFiles(ctx, log, paths.SavePath)
	}

	// (7) Second destPath walk to catch any hardlinks created by a racing
	// hardlink worker between step 5 and step 6 (its handler can keep
	// calling os.Link until the source files disappear). The inode is
	// kept alive by the orphan hardlink itself, so the same key map is
	// still the right lookup.
	hardlinksRemovedLate := 0
	if len(keys) > 0 && paths.DestPath != "" {
		hardlinksRemovedLate = removeHardlinksByInode(ctx, log, paths.DestPath, keys)
	}

	// (8) Gate: confirm savePath is drained and destPath holds no files
	// sharing source inodes (R4 — catches hardlinks created after step 7).
	if err := verifyRemovalDrained(ctx, log, paths, keys); err != nil {
		return err
	}

	// (9) Empty-dir cleanup. os.Remove on a non-empty dir fails with
	// ENOTEMPTY which we treat as "leave it alone" — that's exactly what
	// we want for show folders that still hold other episodes.
	if paths.SavePath != "" {
		tryRemoveEmptyTreeBottomUp(log, paths.SavePath)
	}
	if paths.DestPath != "" {
		tryRemoveEmptyTreeBottomUp(log, paths.DestPath)
	}
	if paths.ShowRoot != "" && paths.ShowRoot != paths.DestPath {
		tryRemoveEmpty(log, paths.ShowRoot)
	}

	log.Info("remove completed",
		zap.Int("sources_indexed", len(sources)),
		zap.Int("hardlinks_removed", hardlinksRemoved),
		zap.Int("hardlinks_removed_late", hardlinksRemovedLate),
		zap.Int("sources_removed", sourcesRemoved),
	)
	return nil
}

// fileKey uniquely identifies a Unix inode on a device. Two files share
// fileKey iff they are hardlinks (or the same file).
type fileKey struct {
	dev uint64
	ino uint64
}

// sourceFile is a regular file under savePath together with its (dev, ino).
type sourceFile struct {
	path string
	key  fileKey
}

// indexSources walks savePath and returns every regular file together with
// its (device, inode). The (dev, ino) pair is the key the destination walk
// uses to identify hardlinks regardless of name or location.
//
// A missing savePath is treated as "no sources to delete"; this can happen
// on a retry that finished partially, or when the download never started.
func indexSources(ctx context.Context, log *zap.Logger, savePath string) ([]sourceFile, error) {
	var out []sourceFile
	err := filepath.Walk(savePath, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			log.Warn("walk error (continuing)", zap.String("path", p), zap.Error(walkErr))
			return nil
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		key, ok := keyOf(info)
		if !ok {
			// Non-Unix FileInfo? Skip — without inode info we can't find a hardlink.
			log.Warn("no inode info for file, skipping", zap.String("path", p))
			return nil
		}
		out = append(out, sourceFile{path: p, key: key})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}
	return out, nil
}

// removeHardlinksByInode walks destPath and deletes every regular file whose
// (device, inode) appears in keys. That's how we locate the hardlink: the
// kernel exposes it under whatever name and folder it was created with, but
// the inode is shared with the source.
//
// destPath is scoped to the media's expected dest folder (movie folder, or
// show root / season folder), so we don't scan the whole library.
func removeHardlinksByInode(ctx context.Context, log *zap.Logger, destPath string, keys map[fileKey]string) int {
	removed := 0
	_ = filepath.Walk(destPath, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			log.Warn("dest walk error (continuing)", zap.String("path", p), zap.Error(walkErr))
			return nil
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		key, ok := keyOf(info)
		if !ok {
			return nil
		}
		src, match := keys[key]
		if !match {
			return nil
		}
		log.Debug("found hardlink by inode",
			zap.String("hardlink", p),
			zap.String("source", src),
			zap.Uint64("dev", key.dev),
			zap.Uint64("ino", key.ino),
		)
		removeFileIfExists(log, p, "hardlink")
		removed++
		return nil
	})
	return removed
}

// countRegularFiles walks root and returns the number of regular files it
// still contains. Used after a delete pass to confirm savePath is drained
// before we let the outer worker delete the media DB row. A missing root
// counts as zero.
func countRegularFiles(ctx context.Context, root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		count++
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return count, err
	}
	return count, nil
}

// removeAllRegularFiles walks root and deletes every regular file underneath
// it. ENOENT is treated as success, so retries are safe. Directories are
// left alone here — tryRemoveEmptyTreeBottomUp prunes empty ones afterwards.
//
// This walks instead of replaying the inode index because the index is a
// snapshot taken before qBittorrent was told to release the torrent; files
// that appeared between the snapshot and the release still need cleanup.
func removeAllRegularFiles(ctx context.Context, log *zap.Logger, root string) int {
	removed := 0
	_ = filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			log.Warn("walk error (continuing)", zap.String("path", p), zap.Error(walkErr))
			return nil
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		switch err := os.Remove(p); {
		case err == nil:
			removed++
			log.Debug("removed source file", zap.String("path", p))
		case os.IsNotExist(err):
			// concurrent removal — fine
		default:
			log.Warn("remove source file failed (continuing)",
				zap.String("path", p),
				zap.Error(err),
			)
		}
		return nil
	})
	return removed
}

// resolvedPaths captures everything Process needs to find files on disk.
type resolvedPaths struct {
	SavePath    string // source folder in /downloads (may be "")
	TorrentHash string // qbittorrent hash (may be "")
	DestPath    string // hardlink folder (movie folder, or season-or-show folder)
	ShowRoot    string // for shows: the top-level show folder, for movies: ""
}

func (s *service) resolvePaths(row *media.Media) (*resolvedPaths, error) {
	switch row.Type {
	case media.MediaTypeMovie:
		if row.Movie == nil {
			return nil, fmt.Errorf("media %d has type=movie but no movie row", row.ID)
		}
		destPath := row.LibraryPath
		if destPath == "" {
			folderName := fmt.Sprintf("%s (%s) (%s)", row.Name, row.Movie.IMDBID, row.Quality)
			destPath = filepath.Join(s.moviesPath, folderName)
		}
		return &resolvedPaths{
			SavePath:    deref(row.Movie.SavePath),
			TorrentHash: deref(row.Movie.TorrentHash),
			DestPath:    destPath,
		}, nil

	case media.MediaTypeShowFull, media.MediaTypeShowSeason, media.MediaTypeShowEpisode:
		if row.ShowEntry == nil || row.ShowEntry.Show == nil {
			return nil, fmt.Errorf("media %d has type=%s but missing show entry/show", row.ID, row.Type)
		}
		show := row.ShowEntry.Show
		showRoot := filepath.Join(s.showsPath, fmt.Sprintf("%s (%s) (%s)", show.Name, show.IMDBID, row.Quality))
		destPath := row.LibraryPath
		if destPath == "" {
			destPath = showRoot
			if row.ShowEntry.Season != nil {
				destPath = filepath.Join(showRoot, fmt.Sprintf("Season %d", *row.ShowEntry.Season))
			}
		}
		return &resolvedPaths{
			SavePath:    deref(row.ShowEntry.SavePath),
			TorrentHash: deref(row.ShowEntry.TorrentHash),
			DestPath:    destPath,
			ShowRoot:    showRoot,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported media type: %s", row.Type)
	}
}

const removalDrainMaxPasses = 3

func verifyRemovalDrained(ctx context.Context, log *zap.Logger, paths *resolvedPaths, keys map[fileKey]string) error {
	for pass := 1; pass <= removalDrainMaxPasses; pass++ {
		if len(keys) > 0 && paths.DestPath != "" {
			if removed := removeHardlinksByInode(ctx, log, paths.DestPath, keys); removed > 0 {
				log.Info("removed late hardlinks during drain gate",
					zap.Int("removed", removed),
					zap.Int("pass", pass),
				)
			}
		}
		if paths.SavePath != "" {
			remaining, err := countRegularFiles(ctx, paths.SavePath)
			if err != nil {
				return fmt.Errorf("verify save path drained: %w", err)
			}
			if remaining > 0 {
				if pass == removalDrainMaxPasses {
					return fmt.Errorf("save path %q still contains %d regular file(s) after delete pass", paths.SavePath, remaining)
				}
				continue
			}
		}
		if len(keys) > 0 && paths.DestPath != "" {
			destMatches, err := countDestInodesMatching(ctx, paths.DestPath, keys)
			if err != nil {
				return fmt.Errorf("verify dest path drained: %w", err)
			}
			if destMatches > 0 {
				if pass == removalDrainMaxPasses {
					return fmt.Errorf("dest path %q still contains %d hardlink(s) after delete pass", paths.DestPath, destMatches)
				}
				continue
			}
		}
		return nil
	}
	return nil
}

func countDestInodesMatching(ctx context.Context, destPath string, keys map[fileKey]string) (int, error) {
	if len(keys) == 0 || destPath == "" {
		return 0, nil
	}
	count := 0
	err := filepath.Walk(destPath, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		key, ok := keyOf(info)
		if !ok {
			return nil
		}
		if _, match := keys[key]; match {
			count++
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return count, err
	}
	return count, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// removeFileIfExists deletes a regular file, treating ENOENT as success.
// All other errors are logged at warn but do not propagate — the queue
// retries the whole job, so a transient I/O error gets another chance.
func removeFileIfExists(log *zap.Logger, path, kind string) {
	if path == "" {
		return
	}
	switch err := os.Remove(path); {
	case err == nil:
		log.Debug("removed file", zap.String("kind", kind), zap.String("path", path))
	case os.IsNotExist(err):
		// expected on retries / partial states
	default:
		log.Warn("remove file failed (continuing)",
			zap.String("kind", kind),
			zap.String("path", path),
			zap.Error(err),
		)
	}
}

// tryRemoveEmpty attempts to delete `dir`. Failures (non-empty, missing,
// permissions) are intentionally swallowed — this is best-effort tidying.
func tryRemoveEmpty(log *zap.Logger, dir string) {
	if dir == "" {
		return
	}
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		log.Debug("dir not removed (likely non-empty)", zap.String("path", dir), zap.Error(err))
		return
	}
	log.Debug("removed empty dir", zap.String("path", dir))
}

// tryRemoveEmptyTreeBottomUp walks `root` and removes every empty directory
// in deepest-first order, then `root` itself. It never deletes files — only
// empty directories — so it's safe to call on shared folders.
func tryRemoveEmptyTreeBottomUp(log *zap.Logger, root string) {
	type entry struct {
		path string
		info os.FileInfo
	}
	var dirs []entry
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			dirs = append(dirs, entry{path: p, info: info})
		}
		return nil
	})
	// Walk emits parents before children; iterate in reverse to delete deepest first.
	for i := len(dirs) - 1; i >= 0; i-- {
		tryRemoveEmpty(log, dirs[i].path)
	}
}


package qbittorrent

import (
	"context"
	"strings"
)

const progressCompleteThreshold = 0.999

// States where the torrent is still downloading or preparing — not safe to finalize.
var activeDownloadStates = map[string]struct{}{
	"downloading":  {},
	"metadl":       {},
	"stalleddl":    {},
	"queueddl":     {},
	"checkingdl":   {},
	"allocating":   {},
	"moving":       {},
	"forceddl":     {},
	"pauseddl":     {},
	"queuedfore":   {},
	"checkingresume": {},
}

// DownloadComplete reports whether torrent-level progress indicates a finished download.
// Deprecated: prefer TorrentTransferComplete for finalize decisions.
func DownloadComplete(progress float64) bool {
	return progress >= progressCompleteThreshold
}

// TorrentTransferComplete is true when qBittorrent reports full progress and the
// torrent is no longer in an active download/prepare state.
func TorrentTransferComplete(t Torrent) bool {
	if t.Progress < progressCompleteThreshold {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(t.State))
	if strings.HasPrefix(state, "checking") {
		return false
	}
	_, active := activeDownloadStates[state]
	return !active
}

// FileDownloadComplete reports whether an individual torrent file has finished downloading.
func FileDownloadComplete(progress float32) bool {
	return progress >= progressCompleteThreshold
}

// TorrentFilesComplete checks that every linkable file in the torrent has finished downloading.
func (s *service) TorrentFilesComplete(ctx context.Context, hash string, shouldCount func(name string, size int64) bool) (bool, error) {
	files, err := s.GetTorrentFiles(ctx, hash)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, nil
	}
	if shouldCount == nil {
		shouldCount = func(string, int64) bool { return true }
	}
	counted := 0
	for _, f := range files {
		if !shouldCount(f.Name, f.Size) {
			continue
		}
		counted++
		if !FileDownloadComplete(f.Progress) {
			return false, nil
		}
	}
	return counted > 0, nil
}

// ReadyForLibrary is true when transfer and per-file progress indicate the torrent
// is fully downloaded (used before marking a request as downloaded).
func (s *service) ReadyForLibrary(ctx context.Context, t Torrent, shouldCount func(name string, size int64) bool) (bool, error) {
	if !TorrentTransferComplete(t) {
		return false, nil
	}
	return s.TorrentFilesComplete(ctx, t.Hash, shouldCount)
}

// FilesCompleteByHash calls GetTorrentFiles at most once per distinct hash in hashes.
func (s *service) FilesCompleteByHash(ctx context.Context, hashes []string, shouldCount func(name string, size int64) bool) (map[string]bool, error) {
	seen := make(map[string]struct{}, len(hashes))
	out := make(map[string]bool, len(hashes))
	for _, raw := range hashes {
		hash := NormalizeHash(raw)
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		ok, err := s.TorrentFilesComplete(ctx, hash, shouldCount)
		if err != nil {
			return nil, err
		}
		out[hash] = ok
	}
	return out, nil
}

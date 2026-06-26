package qbittorrent

import (
	"context"
	"strings"
)

const progressCompleteThreshold = 0.999

// States where the torrent is still downloading or preparing — not safe to finalize.
var activeDownloadStates = map[string]struct{}{
	"downloading":    {},
	"metadl":         {},
	"stalleddl":      {},
	"queueddl":       {},
	"checkingdl":     {},
	"allocating":     {},
	"moving":         {},
	"forceddl":       {},
	"pauseddl":       {},
	"queuedfore":     {},
	"checkingresume": {},
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

// FilesComplete is true when at least one counted file exists and every counted
// file has finished downloading. shouldCount selects which files matter (nil
// counts all). An empty file list is never complete.
func FilesComplete(files []TorrentFile, shouldCount func(name string, size int64) bool) bool {
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
			return false
		}
	}
	return counted > 0
}

// TorrentFilesComplete checks that every linkable file in the torrent has finished downloading.
func (s *service) TorrentFilesComplete(ctx context.Context, hash string, shouldCount func(name string, size int64) bool) (bool, error) {
	files, err := s.GetTorrentFiles(ctx, hash)
	if err != nil {
		return false, err
	}
	return FilesComplete(files, shouldCount), nil
}

// ReadyForLibrary is true when transfer and per-file progress indicate the torrent
// is fully downloaded (used before marking a request as downloaded).
func (s *service) ReadyForLibrary(ctx context.Context, t Torrent, shouldCount func(name string, size int64) bool) (bool, error) {
	if !TorrentTransferComplete(t) {
		return false, nil
	}
	return s.TorrentFilesComplete(ctx, t.Hash, shouldCount)
}

// filesCompleteConcurrency bounds how many per-hash file lookups run at once.
// qBittorrent has no batch files endpoint, so each hash is its own HTTP call;
// the client pools up to 10 connections per host, so 8 is a safe ceiling.
const filesCompleteConcurrency = 8

// FilesCompleteByHash calls GetTorrentFiles at most once per distinct hash in
// hashes, fanning the per-hash lookups out with bounded concurrency so a watcher
// tick with many downloads doesn't serialize N network round-trips.
func (s *service) FilesCompleteByHash(ctx context.Context, hashes []string, shouldCount func(name string, size int64) bool) (map[string]bool, error) {
	seen := make(map[string]struct{}, len(hashes))
	unique := make([]string, 0, len(hashes))
	for _, raw := range hashes {
		hash := NormalizeHash(raw)
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		unique = append(unique, hash)
	}

	out := make(map[string]bool, len(unique))
	if len(unique) == 0 {
		return out, nil
	}

	type result struct {
		hash string
		ok   bool
		err  error
	}
	sem := make(chan struct{}, filesCompleteConcurrency)
	results := make(chan result, len(unique))
	for _, hash := range unique {
		sem <- struct{}{}
		go func(hash string) {
			defer func() { <-sem }()
			ok, err := s.TorrentFilesComplete(ctx, hash, shouldCount)
			results <- result{hash: hash, ok: ok, err: err}
		}(hash)
	}

	var firstErr error
	for range unique {
		r := <-results
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		out[r.hash] = r.ok
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

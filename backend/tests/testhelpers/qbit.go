package testhelpers

import (
	"context"

	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

// StubQbit is a no-op qbittorrent.Service for tests. Embed it and override only
// the methods a given test cares about.
type StubQbit struct {
	AddFunc            func(ctx context.Context, file []byte, savePath, torrentName string) (*qbittorrent.AddTorrentResponse, error)
	TorrentsByHashFunc func(ctx context.Context) (map[string]qbittorrent.Torrent, error)
}

func (s StubQbit) AddTorrent(ctx context.Context, file []byte, savePath, torrentName string) (*qbittorrent.AddTorrentResponse, error) {
	if s.AddFunc != nil {
		return s.AddFunc(ctx, file, savePath, torrentName)
	}
	return &qbittorrent.AddTorrentResponse{}, nil
}

func (s StubQbit) RemoveTorrent(context.Context, string) error { return nil }

func (s StubQbit) ListTorrents(context.Context) ([]qbittorrent.Torrent, error) { return nil, nil }

func (s StubQbit) ListTorrentsPaginated(context.Context, int, int) (*qbittorrent.PaginatedTorrentsResponse, error) {
	return &qbittorrent.PaginatedTorrentsResponse{}, nil
}

func (s StubQbit) TorrentsByHash(ctx context.Context) (map[string]qbittorrent.Torrent, error) {
	if s.TorrentsByHashFunc != nil {
		return s.TorrentsByHashFunc(ctx)
	}
	return map[string]qbittorrent.Torrent{}, nil
}

func (s StubQbit) GetTorrentFiles(context.Context, string) ([]qbittorrent.TorrentFile, error) {
	return nil, nil
}

func (s StubQbit) GetTorrentStatus(context.Context, string) (*qbittorrent.TorrentStatusResponse, error) {
	return nil, nil
}

func (s StubQbit) GetTorrent(context.Context, string) (*qbittorrent.Torrent, error) {
	return nil, nil
}

func (s StubQbit) ReadyForLibrary(context.Context, qbittorrent.Torrent, func(string, int64) bool) (bool, error) {
	return false, nil
}

func (s StubQbit) FilesCompleteByHash(context.Context, []string, func(string, int64) bool) (map[string]bool, error) {
	return nil, nil
}

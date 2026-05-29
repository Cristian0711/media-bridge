package download

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

type RequestDetails struct {
	RequestEntryID uint
	RequestID      string
	MediaID        uint // set on request row after first successful media create
	Type           string
	Name           string
	IMDBID         string
	TMDBID         string
	TVDBID         string
	Season         int
	Episode        int
	PosterURL      string
	TorrentURL     string
	TorrentName    string
	Indexer        string
	Quality        string
	UserID         uint
	Username       string
}

type AddResult struct {
	SavePath    *string
	TorrentHash *string
	SizeBytes   *int64
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type Service interface {
	Add(ctx context.Context, request RequestDetails) (*AddResult, error)
}

// TorrentDownloader fetches a base64-encoded .torrent payload from the indexer
// that owns the given download URL. Implemented by *indexer.Service.
type TorrentDownloader interface {
	DownloadTorrent(ctx context.Context, indexerID, downloadURL string) (string, error)
}

type service struct {
	indexerService TorrentDownloader
	qbitService    qbittorrent.Service
	downloadsPath  string
}

func NewService(indexerService TorrentDownloader, qbitService qbittorrent.Service, downloadsPath string) Service {
	return &service{
		indexerService: indexerService,
		qbitService:    qbitService,
		downloadsPath:  downloadsPath,
	}
}

func (s *service) Add(ctx context.Context, request RequestDetails) (*AddResult, error) {
	indexerID := detectIndexerFromURL(request.TorrentURL)
	torrentBase64, err := s.indexerService.DownloadTorrent(ctx, indexerID, request.TorrentURL)
	if err != nil {
		return nil, fmt.Errorf("download torrent from indexer: %w", err)
	}
	torrentBytes, err := base64.StdEncoding.DecodeString(torrentBase64)
	if err != nil {
		return nil, fmt.Errorf("decode torrent payload: %w", err)
	}

	savePath := s.downloadsPath
	addResp, err := s.qbitService.AddTorrent(ctx, torrentBytes, savePath, request.TorrentName)
	if err != nil && !errors.Is(err, qbittorrent.ErrTorrentExists) {
		return nil, fmt.Errorf("add torrent to qbittorrent: %w", err)
	}
	// On ErrTorrentExists addResp is still populated with the existing
	// torrent's hash and savePath, so we can treat it like a fresh add.
	if addResp == nil {
		return nil, fmt.Errorf("qbittorrent returned nil response for %s", request.TorrentName)
	}
	if addResp.Hash == "" {
		// Storing a placeholder hash means any later op keyed by hash
		// (hardlink lookup, remove flow) silently targets the wrong
		// torrent. Surface as a retry instead.
		return nil, fmt.Errorf("qbittorrent returned empty hash for %s", request.TorrentName)
	}

	now := time.Now().UTC()
	path := addResp.Path
	if path == "" {
		path = savePath
	}
	hash := qbittorrent.NormalizeHash(addResp.Hash)
	var sizeBytes *int64
	if addResp.Size > 0 {
		size := addResp.Size
		sizeBytes = &size
	}
	return &AddResult{
		SavePath:    &path,
		TorrentHash: &hash,
		SizeBytes:   sizeBytes,
		StartedAt:   &now,
		CompletedAt: &now,
	}, nil
}

func detectIndexerFromURL(url string) string {
	lower := strings.ToLower(url)
	if strings.Contains(lower, "torrentleech.org") {
		return "torrentleech"
	}
	if strings.Contains(lower, "filelist.io") {
		return "filelist"
	}
	if strings.Contains(lower, "blutopia.cc") {
		return "blutopia"
	}
	return "filelist"
}

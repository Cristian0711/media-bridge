package qbittorrent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/autobrr/go-qbittorrent"
)

var (
	ErrTorrentExists   = errors.New("torrent already exists")
	ErrTorrentNotFound = errors.New("torrent not found")
)

type Service interface {
	AddTorrent(ctx context.Context, file []byte, savePath, torrentName string) (*AddTorrentResponse, error)
	RemoveTorrent(ctx context.Context, hash string) error
	ListTorrents(ctx context.Context) ([]Torrent, error)
	ListTorrentsPaginated(ctx context.Context, page, pageSize int) (*PaginatedTorrentsResponse, error)
	// TorrentsByHash lists plexmedia torrents once and indexes them by hash.
	TorrentsByHash(ctx context.Context) (map[string]Torrent, error)
	GetTorrentFiles(ctx context.Context, hash string) ([]TorrentFile, error)
	GetTorrentStatus(ctx context.Context, hash string) (*TorrentStatusResponse, error)
	GetTorrent(ctx context.Context, hash string) (*Torrent, error)
	// ReadyForLibrary checks torrent + per-file completion before library finalize.
	ReadyForLibrary(ctx context.Context, t Torrent, shouldCount func(name string, size int64) bool) (bool, error)
	// FilesCompleteByHash fetches per-file completion once per hash (batched watcher lookups).
	FilesCompleteByHash(ctx context.Context, hashes []string, shouldCount func(name string, size int64) bool) (map[string]bool, error)
}

type service struct {
	client *qbittorrent.Client
}

func NewService(url, username, password string) (Service, error) {
	client := qbittorrent.NewClient(qbittorrent.Config{
		Host:     url,
		Username: username,
		Password: password,
	})
	if err := client.LoginCtx(context.Background()); err != nil {
		return nil, err
	}
	return &service{client: client}, nil
}

func (s *service) AddTorrent(ctx context.Context, file []byte, savePath, torrentName string) (*AddTorrentResponse, error) {
	options := map[string]string{"category": "plexmedia"}

	hasFolder, err := torrentHasRootFolder(file)
	if err == nil && !hasFolder && torrentName != "" {
		if savePath != "" {
			options["savepath"] = filepath.Join(savePath, torrentName)
		} else {
			options["savepath"] = torrentName
		}
	} else if savePath != "" {
		options["savepath"] = savePath
	}

	hash, err := torrentHash(file)
	if err != nil {
		return nil, err
	}

	all, err := s.client.GetTorrents(qbittorrent.TorrentFilterOptions{})
	if err != nil {
		return nil, err
	}
	hash = NormalizeHash(hash)
	for _, t := range all {
		if NormalizeHash(t.Hash) == hash {
			return &AddTorrentResponse{Hash: NormalizeHash(t.Hash), Path: t.SavePath, Size: t.Size}, ErrTorrentExists
		}
	}

	if err := s.client.AddTorrentFromMemoryCtx(ctx, file, options); err != nil {
		return nil, err
	}

	added, err := s.waitForTorrent(ctx, hash)
	if err != nil {
		return nil, err
	}

	finalPath, err := s.resolvePath(added, torrentName)
	if err != nil {
		finalPath = added.SavePath
	}

	return &AddTorrentResponse{Hash: NormalizeHash(added.Hash), Path: finalPath, Size: added.Size}, nil
}

// RemoveTorrent removes the torrent from qBittorrent but leaves its files on
// disk. Disk cleanup is the caller's responsibility (see internal/remove) —
// this lets the remove package own both the source-file and hardlink deletes
// using inode matching, instead of relying on qbit's best-effort cleanup.
func (s *service) RemoveTorrent(ctx context.Context, hash string) error {
	return s.client.DeleteTorrents([]string{NormalizeHash(hash)}, false)
}

func (s *service) ListTorrents(ctx context.Context) ([]Torrent, error) {
	torrents, err := s.client.GetTorrents(qbittorrent.TorrentFilterOptions{
		Category: "plexmedia",
		Sort:     "added_on",
		Reverse:  true,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Torrent, 0, len(torrents))
	for _, t := range torrents {
		out = append(out, mapTorrent(t))
	}
	return out, nil
}

func (s *service) ListTorrentsPaginated(ctx context.Context, page, pageSize int) (*PaginatedTorrentsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	all, err := s.client.GetTorrents(qbittorrent.TorrentFilterOptions{Category: "plexmedia"})
	if err != nil {
		return nil, err
	}
	totalCount := len(all)
	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}

	torrents, err := s.client.GetTorrents(qbittorrent.TorrentFilterOptions{
		Category: "plexmedia",
		Sort:     "added_on",
		Reverse:  true,
		Limit:    pageSize,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Torrent, 0, len(torrents))
	for _, t := range torrents {
		out = append(out, mapTorrent(t))
	}

	return &PaginatedTorrentsResponse{
		Torrents:   out,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	}, nil
}

func (s *service) GetTorrentFiles(ctx context.Context, hash string) ([]TorrentFile, error) {
	files, err := s.client.GetFilesInformation(NormalizeHash(hash))
	if err != nil {
		return nil, err
	}
	if files == nil {
		return []TorrentFile{}, nil
	}
	out := make([]TorrentFile, 0, len(*files))
	for _, f := range *files {
		out = append(out, TorrentFile{Name: f.Name, Size: f.Size, Progress: f.Progress})
	}
	return out, nil
}

func (s *service) GetTorrentStatus(ctx context.Context, hash string) (*TorrentStatusResponse, error) {
	t, err := s.GetTorrent(ctx, hash)
	if err != nil {
		return nil, err
	}
	return &TorrentStatusResponse{Hash: t.Hash, State: t.State, Progress: t.Progress}, nil
}

func (s *service) TorrentsByHash(ctx context.Context) (map[string]Torrent, error) {
	torrents, err := s.client.GetTorrentsCtx(ctx, qbittorrent.TorrentFilterOptions{
		Category: "plexmedia",
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]Torrent, len(torrents))
	for _, t := range torrents {
		out[t.Hash] = mapTorrent(t)
	}
	return out, nil
}

func (s *service) GetTorrent(ctx context.Context, hash string) (*Torrent, error) {
	hash = NormalizeHash(hash)
	torrents, err := s.client.GetTorrentsCtx(ctx, qbittorrent.TorrentFilterOptions{
		Category: "plexmedia",
		Hashes:   []string{hash},
	})
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return nil, ErrTorrentNotFound
	}
	out := mapTorrent(torrents[0])
	return &out, nil
}

func (s *service) waitForTorrent(ctx context.Context, hash string) (*qbittorrent.Torrent, error) {
	hash = NormalizeHash(hash)
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for torrent canceled")
		case <-timeout:
			return nil, fmt.Errorf("torrent added but not found")
		case <-ticker.C:
			torrents, err := s.client.GetTorrents(qbittorrent.TorrentFilterOptions{})
			if err != nil {
				return nil, err
			}
			for i := range torrents {
				if NormalizeHash(torrents[i].Hash) == hash {
					return &torrents[i], nil
				}
			}
		}
	}
}

func (s *service) resolvePath(t *qbittorrent.Torrent, createdFolder string) (string, error) {
	files, err := s.client.GetFilesInformation(t.Hash)
	if err != nil || files == nil || len(*files) == 0 {
		return "", fmt.Errorf("cannot determine files")
	}
	root, err := extractRootFolder((*files)[0].Name)
	if err != nil {
		if createdFolder != "" {
			return t.SavePath, nil
		}
		return "", err
	}
	return filepath.Join(t.SavePath, root), nil
}

func mapTorrent(t qbittorrent.Torrent) Torrent {
	return Torrent{
		Hash:       NormalizeHash(t.Hash),
		Name:       t.Name,
		Size:       t.Size,
		Progress:   t.Progress,
		State:      string(t.State),
		Seeders:    int(t.NumSeeds),
		Leechers:   int(t.NumLeechs),
		Downloaded: t.Downloaded,
		Uploaded:   t.Uploaded,
		DlSpeed:    t.DlSpeed,
		UpSpeed:    t.UpSpeed,
		ETA:        t.ETA,
	}
}

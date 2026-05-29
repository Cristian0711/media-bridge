package remove_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/internal/remove"
)

type stubMediaService struct {
	row *media.Media
}

func (s *stubMediaService) GetMediaByID(_ context.Context, id uint) (*media.Media, error) {
	if s.row == nil || s.row.ID != id {
		return nil, media.ErrMediaNotFound
	}
	return s.row, nil
}

func (s *stubMediaService) CreateFromRequest(context.Context, media.CreateFromRequestInput) (uint, error) {
	return 0, nil
}
func (s *stubMediaService) FindExistingDownloadMediaID(context.Context, media.CreateFromRequestInput) (uint, error) {
	return 0, nil
}
func (s *stubMediaService) RemoveFromRequest(context.Context, media.CreateFromRequestInput) error {
	return nil
}
func (s *stubMediaService) UpdateLibraryPath(context.Context, uint, string) error { return nil }
func (s *stubMediaService) GetAllMediaPaginated(context.Context, int, int) (*media.PaginatedMediaResponse, error) {
	return nil, nil
}
func (s *stubMediaService) GetMediaForUserPaginated(context.Context, uint, int, int) (*media.PaginatedMediaResponse, error) {
	return nil, nil
}
func (s *stubMediaService) SearchMedia(context.Context, string, int, int) (*media.PaginatedMediaResponse, error) {
	return nil, nil
}
func (s *stubMediaService) SearchMediaForUser(context.Context, uint, string, int, int) (*media.PaginatedMediaResponse, error) {
	return nil, nil
}

type stubQbitService struct {
	removedHash string
}

func (s *stubQbitService) RemoveTorrent(_ context.Context, hash string) error {
	s.removedHash = hash
	return nil
}
func (s *stubQbitService) AddTorrent(context.Context, []byte, string, string) (*qbittorrent.AddTorrentResponse, error) {
	return nil, nil
}
func (s *stubQbitService) ListTorrents(context.Context) ([]qbittorrent.Torrent, error) {
	return nil, nil
}
func (s *stubQbitService) ListTorrentsPaginated(context.Context, int, int) (*qbittorrent.PaginatedTorrentsResponse, error) {
	return nil, nil
}
func (s *stubQbitService) TorrentsByHash(context.Context) (map[string]qbittorrent.Torrent, error) {
	return nil, nil
}
func (s *stubQbitService) GetTorrentFiles(context.Context, string) ([]qbittorrent.TorrentFile, error) {
	return nil, nil
}
func (s *stubQbitService) GetTorrentStatus(context.Context, string) (*qbittorrent.TorrentStatusResponse, error) {
	return nil, nil
}
func (s *stubQbitService) GetTorrent(context.Context, string) (*qbittorrent.Torrent, error) {
	return nil, nil
}
func (s *stubQbitService) ReadyForLibrary(context.Context, qbittorrent.Torrent, func(string, int64) bool) (bool, error) {
	return false, nil
}
func (s *stubQbitService) FilesCompleteByHash(context.Context, []string, func(string, int64) bool) (map[string]bool, error) {
	return nil, nil
}

type stubHardlinkCanceler struct {
	cancelled uint
}

func (s *stubHardlinkCanceler) CancelByMediaID(_ context.Context, mediaID uint) error {
	s.cancelled = mediaID
	return nil
}

func TestProcess_RemovesSourceAndLibraryHardlinks(t *testing.T) {
	root := t.TempDir()
	moviesPath := filepath.Join(root, "movies")
	downloadsPath := filepath.Join(root, "downloads", "release")
	destPath := filepath.Join(moviesPath, "Film (tt1) (1080p)")
	if err := os.MkdirAll(downloadsPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destPath, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(downloadsPath, "movie.mkv")
	if err := os.WriteFile(srcFile, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	destFile := filepath.Join(destPath, "movie.mkv")
	if err := os.Link(srcFile, destFile); err != nil {
		t.Fatal(err)
	}

	hash := "abc123"
	savePath := downloadsPath
	mediaRow := &media.Media{
		ID:          1,
		Type:        media.MediaTypeMovie,
		Name:        "Film",
		LibraryPath: destPath,
		Quality:     "1080p",
		Movie: &media.Movie{
			IMDBID:      "tt1",
			SavePath:    &savePath,
			TorrentHash: &hash,
		},
	}

	qbit := &stubQbitService{}
	canceler := &stubHardlinkCanceler{}
	svc := remove.NewService(&stubMediaService{row: mediaRow}, qbit, canceler, moviesPath, filepath.Join(root, "shows"))

	err := svc.Process(context.Background(), remove.RequestDetails{
		MediaID: 1,
		Type:    "movie_remove",
	})
	if err != nil {
		t.Fatalf("process: %v", err)
	}

	if canceler.cancelled != 1 {
		t.Fatalf("expected hardlink cancel for media 1, got %d", canceler.cancelled)
	}
	if qbit.removedHash != hash {
		t.Fatalf("expected qbit remove hash %q, got %q", hash, qbit.removedHash)
	}
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Fatal("source file should be deleted")
	}
	if _, err := os.Stat(destFile); !os.IsNotExist(err) {
		t.Fatal("library hardlink should be deleted")
	}
}

func TestProcess_UsesStoredLibraryPath(t *testing.T) {
	root := t.TempDir()
	moviesPath := filepath.Join(root, "movies")
	customDest := filepath.Join(root, "custom-library", "Film")
	downloadsPath := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloadsPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(customDest, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(downloadsPath, "movie.mkv")
	if err := os.WriteFile(srcFile, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	destFile := filepath.Join(customDest, "movie.mkv")
	if err := os.Link(srcFile, destFile); err != nil {
		t.Fatal(err)
	}

	hash := "def456"
	savePath := downloadsPath
	mediaRow := &media.Media{
		ID:          2,
		Type:        media.MediaTypeMovie,
		Name:        "Renamed Later",
		LibraryPath: customDest,
		Quality:     "1080p",
		Movie: &media.Movie{
			IMDBID:      "tt1",
			SavePath:    &savePath,
			TorrentHash: &hash,
		},
	}

	svc := remove.NewService(
		&stubMediaService{row: mediaRow},
		&stubQbitService{},
		&stubHardlinkCanceler{},
		moviesPath,
		filepath.Join(root, "shows"),
	)

	if err := svc.Process(context.Background(), remove.RequestDetails{MediaID: 2, Type: "movie_remove"}); err != nil {
		t.Fatalf("process: %v", err)
	}
	if _, err := os.Stat(destFile); !os.IsNotExist(err) {
		t.Fatal("hardlink under stored library_path should be removed even when name changed")
	}
}

func TestProcess_MediaAlreadyGoneIsSuccess(t *testing.T) {
	svc := remove.NewService(
		&stubMediaService{row: nil},
		&stubQbitService{},
		&stubHardlinkCanceler{},
		t.TempDir(),
		t.TempDir(),
	)
	if err := svc.Process(context.Background(), remove.RequestDetails{MediaID: 99, Type: "movie_remove"}); err != nil {
		t.Fatalf("expected success when media gone: %v", err)
	}
}

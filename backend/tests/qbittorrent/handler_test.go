package qbittorrent_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	qbit "github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/gin-gonic/gin"
)

type qbitServiceStub struct {
	addFn            func(ctx context.Context, file []byte, savePath, torrentName string) (*qbit.AddTorrentResponse, error)
	removeFn         func(ctx context.Context, hash string) error
	listFn           func(ctx context.Context) ([]qbit.Torrent, error)
	listPaginatedFn  func(ctx context.Context, page, pageSize int) (*qbit.PaginatedTorrentsResponse, error)
	filesFn          func(ctx context.Context, hash string) ([]qbit.TorrentFile, error)
	statusFn         func(ctx context.Context, hash string) (*qbit.TorrentStatusResponse, error)
	torrentsByHashFn func(ctx context.Context) (map[string]qbit.Torrent, error)
	getTorrentFn       func(ctx context.Context, hash string) (*qbit.Torrent, error)
	readyForLibraryFn     func(ctx context.Context, t qbit.Torrent, shouldCount func(name string, size int64) bool) (bool, error)
	filesCompleteByHashFn func(ctx context.Context, hashes []string, shouldCount func(name string, size int64) bool) (map[string]bool, error)
}

func (s *qbitServiceStub) AddTorrent(ctx context.Context, file []byte, savePath, torrentName string) (*qbit.AddTorrentResponse, error) {
	return s.addFn(ctx, file, savePath, torrentName)
}
func (s *qbitServiceStub) RemoveTorrent(ctx context.Context, hash string) error {
	return s.removeFn(ctx, hash)
}
func (s *qbitServiceStub) ListTorrents(ctx context.Context) ([]qbit.Torrent, error) {
	return s.listFn(ctx)
}
func (s *qbitServiceStub) ListTorrentsPaginated(ctx context.Context, page, pageSize int) (*qbit.PaginatedTorrentsResponse, error) {
	return s.listPaginatedFn(ctx, page, pageSize)
}
func (s *qbitServiceStub) GetTorrentFiles(ctx context.Context, hash string) ([]qbit.TorrentFile, error) {
	return s.filesFn(ctx, hash)
}
func (s *qbitServiceStub) GetTorrentStatus(ctx context.Context, hash string) (*qbit.TorrentStatusResponse, error) {
	return s.statusFn(ctx, hash)
}
func (s *qbitServiceStub) TorrentsByHash(ctx context.Context) (map[string]qbit.Torrent, error) {
	if s.torrentsByHashFn != nil {
		return s.torrentsByHashFn(ctx)
	}
	return map[string]qbit.Torrent{}, nil
}
func (s *qbitServiceStub) GetTorrent(ctx context.Context, hash string) (*qbit.Torrent, error) {
	if s.getTorrentFn != nil {
		return s.getTorrentFn(ctx, hash)
	}
	return nil, qbit.ErrTorrentNotFound
}
func (s *qbitServiceStub) FilesCompleteByHash(ctx context.Context, hashes []string, shouldCount func(name string, size int64) bool) (map[string]bool, error) {
	if s.filesCompleteByHashFn != nil {
		return s.filesCompleteByHashFn(ctx, hashes, shouldCount)
	}
	return map[string]bool{}, nil
}
func (s *qbitServiceStub) ReadyForLibrary(ctx context.Context, t qbit.Torrent, shouldCount func(name string, size int64) bool) (bool, error) {
	if s.readyForLibraryFn != nil {
		return s.readyForLibraryFn(ctx, t, shouldCount)
	}
	return false, nil
}

func TestAddTorrentHandlerStatuses(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	encoded := base64.StdEncoding.EncodeToString([]byte("dummy-data"))
	requestBody := map[string]string{
		"torrent_base64": encoded,
		"save_path":      "/tmp",
		"torrent_name":   "my-torrent",
	}
	bodyBytes, _ := json.Marshal(requestBody)

	tests := []struct {
		name       string
		body       []byte
		svcErr     error
		wantStatus int
	}{
		{name: "invalid body", body: []byte("not-json"), wantStatus: http.StatusBadRequest},
		{name: "invalid base64", body: []byte(`{"torrent_base64":"*","torrent_name":"x"}`), wantStatus: http.StatusBadRequest},
		{name: "duplicate torrent", body: bodyBytes, svcErr: qbit.ErrTorrentExists, wantStatus: http.StatusConflict},
		{name: "service error", body: bodyBytes, svcErr: errors.New("boom"), wantStatus: http.StatusInternalServerError},
		{name: "success", body: bodyBytes, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := qbit.NewHandler(&qbitServiceStub{
				addFn: func(context.Context, []byte, string, string) (*qbit.AddTorrentResponse, error) {
					return &qbit.AddTorrentResponse{Hash: "h", Path: "/tmp/my-torrent"}, tt.svcErr
				},
			}, nil)
			r := gin.New()
			r.POST("/api/v1/qbittorrent/torrents", h.AddTorrent)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/qbittorrent/torrents", bytes.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestListStatusAndEventsHandlers(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := qbit.NewHandler(&qbitServiceStub{
		removeFn: func(context.Context, string) error { return nil },
		listFn: func(context.Context) ([]qbit.Torrent, error) {
			return []qbit.Torrent{{Hash: "h1", Name: "n1"}}, nil
		},
		listPaginatedFn: func(context.Context, int, int) (*qbit.PaginatedTorrentsResponse, error) {
			return &qbit.PaginatedTorrentsResponse{Page: 1, PageSize: 20, TotalCount: 1, TotalPages: 1}, nil
		},
		filesFn: func(context.Context, string) ([]qbit.TorrentFile, error) {
			return []qbit.TorrentFile{{Name: "file.mkv", Size: 10}}, nil
		},
		statusFn: func(context.Context, string) (*qbit.TorrentStatusResponse, error) {
			return nil, qbit.ErrTorrentNotFound
		},
	}, nil)

	r := gin.New()
	r.GET("/api/v1/qbittorrent/torrents", h.ListTorrents)
	r.GET("/api/v1/qbittorrent/torrents/:hash/files", h.GetTorrentFiles)
	r.GET("/api/v1/qbittorrent/torrents/:hash/status", h.GetTorrentStatus)
	r.GET("/api/v1/qbittorrent/torrents/events", h.TorrentEvents)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/qbittorrent/torrents?page=1&page_size=20", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected list paginated 200, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/qbittorrent/torrents/h1/files", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected files 200, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/qbittorrent/torrents/h1/status", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/qbittorrent/torrents/events", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected events 503 when broker disabled, got %d", w.Code)
	}
}

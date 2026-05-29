package requests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	"github.com/gin-gonic/gin"
)

type requestsSvcStub struct {
	movieDownloadFn func(ctx context.Context, req requests.MovieDownloadRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error)
	showDownloadFn  func(ctx context.Context, req requests.ShowDownloadRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error)
	movieRemoveFn   func(ctx context.Context, req requests.MovieRemoveRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error)
	showRemoveFn    func(ctx context.Context, req requests.ShowRemoveRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error)
	listFn          func(ctx context.Context, page, pageSize int) (*requests.PaginatedRequestsResponse, error)
	listQueueFn          func(ctx context.Context, page, pageSize int) (*requests.PaginatedQueueEntriesResponse, error)
	listForUserFn        func(ctx context.Context, userID uint, page, pageSize int) (*requests.PaginatedRequestsResponse, error)
	getRequestTorrentFn       func(ctx context.Context, requestID uint) (*requests.RequestTorrentInfo, error)
	getRequestTorrentFreshFn  func(ctx context.Context, requestID uint) (*requests.RequestTorrentInfo, error)
}

func (s *requestsSvcStub) RequestMovieDownload(ctx context.Context, req requests.MovieDownloadRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error) {
	return s.movieDownloadFn(ctx, req, userID, username, requestID)
}
func (s *requestsSvcStub) RequestShowDownload(ctx context.Context, req requests.ShowDownloadRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error) {
	return s.showDownloadFn(ctx, req, userID, username, requestID)
}
func (s *requestsSvcStub) RequestMovieRemove(ctx context.Context, req requests.MovieRemoveRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error) {
	return s.movieRemoveFn(ctx, req, userID, username, requestID)
}
func (s *requestsSvcStub) RequestShowRemove(ctx context.Context, req requests.ShowRemoveRequestBody, userID uint, username, requestID string) (*requests.RequestAck, error) {
	return s.showRemoveFn(ctx, req, userID, username, requestID)
}
func (s *requestsSvcStub) ListRequests(ctx context.Context, page, pageSize int) (*requests.PaginatedRequestsResponse, error) {
	return s.listFn(ctx, page, pageSize)
}
func (s *requestsSvcStub) ListQueueEntries(ctx context.Context, page, pageSize int) (*requests.PaginatedQueueEntriesResponse, error) {
	return s.listQueueFn(ctx, page, pageSize)
}
func (s *requestsSvcStub) ListRequestsForUser(ctx context.Context, userID uint, page, pageSize int) (*requests.PaginatedRequestsResponse, error) {
	if s.listForUserFn != nil {
		return s.listForUserFn(ctx, userID, page, pageSize)
	}
	return &requests.PaginatedRequestsResponse{}, nil
}
func (s *requestsSvcStub) GetRequestTorrentInfo(ctx context.Context, requestID uint) (*requests.RequestTorrentInfo, error) {
	if s.getRequestTorrentFn != nil {
		return s.getRequestTorrentFn(ctx, requestID)
	}
	return &requests.RequestTorrentInfo{}, nil
}

func (s *requestsSvcStub) GetRequestTorrentInfoFresh(ctx context.Context, requestID uint) (*requests.RequestTorrentInfo, error) {
	if s.getRequestTorrentFreshFn != nil {
		return s.getRequestTorrentFreshFn(ctx, requestID)
	}
	return s.GetRequestTorrentInfo(ctx, requestID)
}

func TestMovieDownloadHandlerStatusCodes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	okBody, _ := json.Marshal(requests.MovieDownloadRequestBody{
		Name: "Movie", IMDBID: "tt123", TorrentURL: "http://torrent", TorrentName: "name", Indexer: "1337x", Quality: "1080p",
	})

	tests := []struct {
		name       string
		body       []byte
		svcErr     error
		wantStatus int
	}{
		{name: "bad json", body: []byte("bad"), wantStatus: http.StatusBadRequest},
		{name: "service error", body: okBody, svcErr: errors.New("boom"), wantStatus: http.StatusInternalServerError},
		{name: "ok", body: okBody, wantStatus: http.StatusAccepted},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := requests.NewHandler(&requestsSvcStub{
				movieDownloadFn: func(context.Context, requests.MovieDownloadRequestBody, uint, string, string) (*requests.RequestAck, error) {
					return &requests.RequestAck{Status: "accepted"}, tt.svcErr
				},
				showDownloadFn: func(context.Context, requests.ShowDownloadRequestBody, uint, string, string) (*requests.RequestAck, error) {
					return &requests.RequestAck{}, nil
				},
				movieRemoveFn: func(context.Context, requests.MovieRemoveRequestBody, uint, string, string) (*requests.RequestAck, error) {
					return &requests.RequestAck{}, nil
				},
				showRemoveFn: func(context.Context, requests.ShowRemoveRequestBody, uint, string, string) (*requests.RequestAck, error) {
					return &requests.RequestAck{}, nil
				},
				listFn: func(context.Context, int, int) (*requests.PaginatedRequestsResponse, error) {
					return &requests.PaginatedRequestsResponse{}, nil
				},
				listQueueFn: func(context.Context, int, int) (*requests.PaginatedQueueEntriesResponse, error) {
					return &requests.PaginatedQueueEntriesResponse{}, nil
				},
			})

			r := gin.New()
			requests.RegisterRoutes(r.Group("/api/v1"), h)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/movies/download", bytes.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestRequestsRoutesRegistered(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	requests.RegisterRoutes(r.Group("/api/v1"), requests.NewHandler(&requestsSvcStub{
		movieDownloadFn: func(context.Context, requests.MovieDownloadRequestBody, uint, string, string) (*requests.RequestAck, error) {
			return &requests.RequestAck{}, nil
		},
		showDownloadFn: func(context.Context, requests.ShowDownloadRequestBody, uint, string, string) (*requests.RequestAck, error) {
			return &requests.RequestAck{}, nil
		},
		movieRemoveFn: func(context.Context, requests.MovieRemoveRequestBody, uint, string, string) (*requests.RequestAck, error) {
			return &requests.RequestAck{}, nil
		},
		showRemoveFn: func(context.Context, requests.ShowRemoveRequestBody, uint, string, string) (*requests.RequestAck, error) {
			return &requests.RequestAck{}, nil
		},
		listFn: func(context.Context, int, int) (*requests.PaginatedRequestsResponse, error) {
			return &requests.PaginatedRequestsResponse{}, nil
		},
		listQueueFn: func(context.Context, int, int) (*requests.PaginatedQueueEntriesResponse, error) {
			return &requests.PaginatedQueueEntriesResponse{}, nil
		},
	}))

	expected := map[string]bool{
		"GET /api/v1/requests":                  false,
		"GET /api/v1/requests/queue":            false,
		"POST /api/v1/requests/movies/download": false,
		"POST /api/v1/requests/shows/download":  false,
		"POST /api/v1/requests/movies/remove":   false,
		"POST /api/v1/requests/shows/remove":    false,
	}
	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}
	for key, found := range expected {
		if !found {
			t.Fatalf("missing route: %s", key)
		}
	}
}

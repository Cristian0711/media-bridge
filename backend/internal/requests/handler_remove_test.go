package requests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	"github.com/gin-gonic/gin"
)

func TestMovieRemoveHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	body, _ := json.Marshal(requests.MovieRemoveRequestBody{MediaID: 42})

	tests := []struct {
		name       string
		svcErr     error
		wantStatus int
	}{
		{name: "ok", wantStatus: http.StatusAccepted},
		{name: "service error", svcErr: errors.New("boom"), wantStatus: http.StatusInternalServerError},
		{name: "media not found", svcErr: media.ErrMediaNotFound, wantStatus: http.StatusNotFound},
		{name: "wrong media type", svcErr: requests.ErrInvalidMediaType, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := requests.NewHandler(&requestsSvcStub{
				movieRemoveFn: func(context.Context, requests.MovieRemoveRequestBody, uint, string, string) (*requests.RequestAck, error) {
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &requests.RequestAck{Status: "accepted", Message: "movie remove request queued for processing"}, nil
				},
			})
			r := gin.New()
			requests.RegisterRoutes(r.Group("/api/v1"), h)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/movies/remove", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestShowRemoveHandler_BadJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := requests.NewHandler(&requestsSvcStub{
		showRemoveFn: func(context.Context, requests.ShowRemoveRequestBody, uint, string, string) (*requests.RequestAck, error) {
			return &requests.RequestAck{}, nil
		},
	})
	r := gin.New()
	requests.RegisterRoutes(r.Group("/api/v1"), h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/shows/remove", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

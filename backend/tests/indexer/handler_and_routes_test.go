package indexer_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/indexer"
	"github.com/gin-gonic/gin"
)

func TestIndexerRoutesRegistered(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	indexer.RegisterRoutes(r.Group("/api/v1"), indexer.NewHandler(indexer.NewService()))

	expected := map[string]bool{
		"GET /api/v1/indexer/indexers":           false,
		"GET /api/v1/indexer/search/movies":      false,
		"GET /api/v1/indexer/search/shows":       false,
		"GET /api/v1/indexer/search/movies/best": false,
		"GET /api/v1/indexer/search/shows/best":  false,
		"POST /api/v1/indexer/download":          false,
	}
	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}
	for k, found := range expected {
		if !found {
			t.Fatalf("missing route: %s", k)
		}
	}
}

func TestSearchMoviesRequiresImdbID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	indexer.RegisterRoutes(r.Group("/api/v1"), indexer.NewHandler(indexer.NewService()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/indexer/search/movies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSearchShowsInvalidSeasonValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	indexer.RegisterRoutes(r.Group("/api/v1"), indexer.NewHandler(indexer.NewService()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/indexer/search/shows?season=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

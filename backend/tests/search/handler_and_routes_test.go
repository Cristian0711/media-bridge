package search_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/search"
	"github.com/gin-gonic/gin"
)

func testTMDBConfig() search.TMDBConfig {
	return search.TMDBConfig{BaseURL: "https://api.themoviedb.org/3", APIKey: "test"}
}

func TestSearchRoutesRegistered(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := search.NewService(testTMDBConfig())
	search.RegisterRoutes(r.Group("/api/v1"), search.NewHandler(svc))

	expected := map[string]bool{
		"GET /api/v1/search":                          false,
		"GET /api/v1/search/external-ids":             false,
		"GET /api/v1/search/movies":                   false,
		"GET /api/v1/search/shows":                    false,
		"GET /api/v1/browse/services":                      false,
		"GET /api/v1/browse/services/:serviceId/catalog": false,
		"GET /api/v1/browse/services/:serviceId/lists":     false,
		"GET /api/v1/browse/global/catalog":                false,
		"GET /api/v1/browse/lists":                         false,
		"GET /api/v1/browse/:id":                           false,
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

func TestSearchMissingQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := search.NewService(testTMDBConfig())
	search.RegisterRoutes(r.Group("/api/v1"), search.NewHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSearchMoviesMissingQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := search.NewService(testTMDBConfig())
	search.RegisterRoutes(r.Group("/api/v1"), search.NewHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/movies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSearchShowsMissingQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := search.NewService(testTMDBConfig())
	search.RegisterRoutes(r.Group("/api/v1"), search.NewHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/shows", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestExternalIDsMissingParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := search.NewService(testTMDBConfig())
	search.RegisterRoutes(r.Group("/api/v1"), search.NewHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/external-ids", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestAdminMiddlewareUsesUserRoleHeader(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := auth.NewHandler(&authSvcStub{
		listKeysFn: func(context.Context) (*auth.ListKeysResponse, error) {
			return &auth.ListKeysResponse{}, nil
		},
	})

	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	auth.RegisterAdminKeyRoutes(g, h)

	t.Run("non-admin forbidden", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
		req.Header.Set("X-User-Role", "user")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", w.Code)
		}
	})

	t.Run("admin allowed", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
		req.Header.Set("X-User-Role", "admin")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

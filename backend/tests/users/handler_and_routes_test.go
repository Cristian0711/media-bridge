package users_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"github.com/gin-gonic/gin"
)

type usersSvcStub struct {
	findByIDFn       func(ctx context.Context, id uint) (*users.User, error)
	findByUsernameFn func(ctx context.Context, username string) (*users.User, error)
	countFn          func(ctx context.Context) (int64, error)
	createFn         func(ctx context.Context, input users.CreateInput) (*users.User, error)
}

func (s *usersSvcStub) FindByID(ctx context.Context, id uint) (*users.User, error) {
	return s.findByIDFn(ctx, id)
}
func (s *usersSvcStub) FindByUsername(ctx context.Context, username string) (*users.User, error) {
	return s.findByUsernameFn(ctx, username)
}
func (s *usersSvcStub) Count(ctx context.Context) (int64, error) {
	if s.countFn == nil {
		return 0, nil
	}
	return s.countFn(ctx)
}
func (s *usersSvcStub) Create(ctx context.Context, input users.CreateInput) (*users.User, error) {
	return s.createFn(ctx, input)
}

func TestGetUserHandlerStatusCodes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		callerID   uint // 0 = no auth context set
		callerRole string
		svcErr     error
		wantStatus int
	}{
		{"bad id", "/api/v1/users/nope", 1, "", nil, http.StatusBadRequest},
		{"forbidden other user", "/api/v1/users/1", 2, "", nil, http.StatusForbidden},
		{"not found", "/api/v1/users/1", 1, "", users.ErrNotFound, http.StatusNotFound},
		{"unexpected", "/api/v1/users/1", 1, "", errors.New("boom"), http.StatusInternalServerError},
		{"ok self", "/api/v1/users/1", 1, "", nil, http.StatusOK},
		{"ok admin", "/api/v1/users/1", 99, users.RoleAdmin, nil, http.StatusOK},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := users.NewHandler(&usersSvcStub{
				findByIDFn: func(context.Context, uint) (*users.User, error) {
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &users.User{ID: 1, Username: "alice"}, nil
				},
			})
			r := gin.New()
			g := r.Group("/api/v1")
			if tt.callerID != 0 {
				g.Use(func(c *gin.Context) {
					c.Set("user_id", tt.callerID)
					if tt.callerRole != "" {
						c.Set("user_role", tt.callerRole)
					}
					c.Next()
				})
			}
			users.RegisterRoutes(g, h)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestUsersRoutesRegistered(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	users.RegisterRoutes(r.Group("/api/v1"), users.NewHandler(&usersSvcStub{
		findByIDFn:       func(context.Context, uint) (*users.User, error) { return &users.User{}, nil },
		findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, nil },
		createFn:         func(context.Context, users.CreateInput) (*users.User, error) { return nil, nil },
	}))

	found := false
	for _, route := range r.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/users/:id" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected GET /api/v1/users/:id route to be registered")
	}
}

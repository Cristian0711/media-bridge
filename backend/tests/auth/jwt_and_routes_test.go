package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"github.com/gin-gonic/gin"
)

func TestJWTGenerateAndParse(t *testing.T) {
	t.Parallel()
	j := auth.NewJWTManager("secret")
	token, err := j.Generate(42, "alice", users.RoleAdmin)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	claims, err := j.Parse(token)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("expected user id 42, got %d", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Fatalf("expected username alice, got %q", claims.Username)
	}

	other := auth.NewJWTManager("other-secret")
	if _, err := other.Parse(token); err == nil {
		t.Fatal("expected parse with wrong secret to fail")
	}
}

func TestAuthRoutesAreRegistered(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1")
	handler := auth.NewHandler(&authSvcStub{
		loginFn:         func(_ context.Context, _ auth.LoginRequest) (*auth.LoginResponse, error) { return &auth.LoginResponse{}, nil },
		registerFn:      func(_ context.Context, _ auth.RegisterRequest) (*auth.RegisterResponse, error) { return &auth.RegisterResponse{}, nil },
		validateTokenFn: func(_ context.Context, _ string) (*auth.ValidateResponse, error) { return &auth.ValidateResponse{}, nil },
		generateKeyFn:   func(_ context.Context, _ uint) (*auth.GenerateKeyResponse, error) { return &auth.GenerateKeyResponse{}, nil },
		listKeysFn:      func(_ context.Context, _ uint) (*auth.ListKeysResponse, error) { return &auth.ListKeysResponse{}, nil },
		getKeyStatusFn:  func(_ context.Context, _ uint, _ string) (*auth.KeyStatusResponse, error) { return &auth.KeyStatusResponse{}, nil },
	})
	auth.RegisterPublicRoutes(group, handler)
	auth.RegisterProtectedRoutes(group, handler)
	auth.RegisterValidationRoute(group, handler)

	got := map[string]bool{}
	for _, route := range r.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{
		http.MethodPost + " /api/v1/auth/login",
		http.MethodPost + " /api/v1/auth/register",
		http.MethodGet + " /api/v1/auth/validate",
		http.MethodGet + " /api/v1/keys",
		http.MethodPost + " /api/v1/keys/generate",
		http.MethodGet + " /api/v1/keys/:value/validate",
	} {
		if !got[expected] {
			t.Fatalf("missing route: %s", expected)
		}
	}
}

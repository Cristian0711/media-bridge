package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

type authSvcStub struct {
	loginFn         func(ctx context.Context, req auth.LoginRequest) (*auth.LoginResponse, error)
	registerFn      func(ctx context.Context, req auth.RegisterRequest) (*auth.RegisterResponse, error)
	validateTokenFn func(ctx context.Context, token string) (*auth.ValidateResponse, error)
	generateKeyFn   func(ctx context.Context, userID uint) (*auth.GenerateKeyResponse, error)
	getKeyStatusFn  func(ctx context.Context, value string) (*auth.KeyStatusResponse, error)
}

func (s *authSvcStub) Login(ctx context.Context, req auth.LoginRequest) (*auth.LoginResponse, error) {
	return s.loginFn(ctx, req)
}
func (s *authSvcStub) Register(ctx context.Context, req auth.RegisterRequest) (*auth.RegisterResponse, error) {
	return s.registerFn(ctx, req)
}
func (s *authSvcStub) ValidateToken(ctx context.Context, token string) (*auth.ValidateResponse, error) {
	return s.validateTokenFn(ctx, token)
}
func (s *authSvcStub) GenerateKey(ctx context.Context, userID uint) (*auth.GenerateKeyResponse, error) {
	return s.generateKeyFn(ctx, userID)
}
func (s *authSvcStub) GetKeyStatus(ctx context.Context, value string) (*auth.KeyStatusResponse, error) {
	return s.getKeyStatusFn(ctx, value)
}

func TestRegisterHandlerStatusCodes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		svcErr     error
		wantStatus int
	}{
		{"duplicate", auth.ErrUserAlreadyExists, http.StatusConflict},
		{"invalid key", auth.ErrKeyInvalid, http.StatusForbidden},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := auth.NewHandler(&authSvcStub{
				registerFn: func(context.Context, auth.RegisterRequest) (*auth.RegisterResponse, error) {
					return nil, tt.svcErr
				},
			})
			body, _ := json.Marshal(auth.RegisterRequest{Username: "alice", Password: "password", Key: "k"})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r := gin.New()
			r.POST("/api/v1/auth/register", h.Register)
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestOtherAuthHandlers(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := auth.NewHandler(&authSvcStub{
		loginFn:         func(context.Context, auth.LoginRequest) (*auth.LoginResponse, error) { return &auth.LoginResponse{Token: "t"}, nil },
		validateTokenFn: func(context.Context, string) (*auth.ValidateResponse, error) { return &auth.ValidateResponse{Valid: true, UserID: 1}, nil },
		generateKeyFn:   func(context.Context, uint) (*auth.GenerateKeyResponse, error) { return &auth.GenerateKeyResponse{Key: "k"}, nil },
		getKeyStatusFn:  func(context.Context, string) (*auth.KeyStatusResponse, error) { return &auth.KeyStatusResponse{Value: "k", IsActive: true}, nil },
		registerFn:      func(context.Context, auth.RegisterRequest) (*auth.RegisterResponse, error) { return &auth.RegisterResponse{ID: 1, Username: "a"}, nil },
	})

	r := gin.New()
	g := r.Group("/api/v1")
	auth.RegisterPublicRoutes(g, h)
	protected := g.Group("")
	protected.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("username", "alice")
		c.Next()
	})
	auth.RegisterProtectedRoutes(protected, h)
	auth.RegisterValidationRoute(g, h)

	loginBody, _ := json.Marshal(auth.LoginRequest{Username: "alice", Password: "password"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d", loginW.Code)
	}

	validateReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/validate", nil)
	validateReq.Header.Set("Authorization", "Bearer token")
	validateW := httptest.NewRecorder()
	r.ServeHTTP(validateW, validateReq)
	if validateW.Code != http.StatusOK {
		t.Fatalf("expected validate 200, got %d", validateW.Code)
	}

	generateReq := httptest.NewRequest(http.MethodPost, "/api/v1/keys/generate", nil)
	generateW := httptest.NewRecorder()
	r.ServeHTTP(generateW, generateReq)
	if generateW.Code != http.StatusCreated {
		t.Fatalf("expected generate 201, got %d", generateW.Code)
	}

	keyStatusReq := httptest.NewRequest(http.MethodGet, "/api/v1/keys/k/validate", nil)
	keyStatusW := httptest.NewRecorder()
	r.ServeHTTP(keyStatusW, keyStatusReq)
	if keyStatusW.Code != http.StatusOK {
		t.Fatalf("expected key status 200, got %d", keyStatusW.Code)
	}
}

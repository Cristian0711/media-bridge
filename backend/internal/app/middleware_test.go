package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// newTestContext builds a gin context wrapping a request with the given headers.
func newTestContext(headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/media", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c, w
}

func TestSkipTracing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		path    string
		headers map[string]string
		want    bool
	}{
		{"internal auth probe", "/api/v1/media", map[string]string{"X-Internal-Auth-Check": "1"}, true},
		{"sse stream", "/api/v1/events", nil, true},
		{"normal request", "/api/v1/media", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, tc.path, nil)
			for k, v := range tc.headers {
				c.Request.Header.Set(k, v)
			}
			if got := skipTracing(c); got != tc.want {
				t.Errorf("skipTracing = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestContextMiddlewareGeneratesRequestID(t *testing.T) {
	t.Parallel()
	c, w := newTestContext(nil)
	contextMiddleware()(c)

	id, ok := c.Get("request_id")
	if !ok || id == "" {
		t.Fatal("contextMiddleware did not set a request_id")
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("contextMiddleware did not echo X-Request-ID response header")
	}
}

func TestContextMiddlewareHonorsIncomingRequestID(t *testing.T) {
	t.Parallel()
	c, _ := newTestContext(map[string]string{"X-Request-ID": "given-id"})
	contextMiddleware()(c)

	if id, _ := c.Get("request_id"); id != "given-id" {
		t.Errorf("request_id = %v, want the incoming given-id", id)
	}
}

func TestProxyAuthMiddlewareRejectsMissingHeaders(t *testing.T) {
	t.Parallel()
	c, w := newTestContext(nil) // no X-User-ID / X-Username
	proxyAuthMiddleware()(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if !c.IsAborted() {
		t.Error("expected the middleware to abort the chain")
	}
}

func TestProxyAuthMiddlewareRejectsInvalidUserID(t *testing.T) {
	t.Parallel()
	c, w := newTestContext(map[string]string{"X-User-ID": "0", "X-Username": "alice"})
	proxyAuthMiddleware()(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for zero user id", w.Code)
	}
}

func TestProxyAuthMiddlewareAcceptsValidHeaders(t *testing.T) {
	t.Parallel()
	c, w := newTestContext(map[string]string{
		"X-User-ID":   "42",
		"X-Username":  "alice",
		"X-User-Role": "admin",
	})
	proxyAuthMiddleware()(c)

	if c.IsAborted() || w.Code == http.StatusUnauthorized {
		t.Fatalf("valid headers rejected: aborted=%v code=%d", c.IsAborted(), w.Code)
	}
	if uid, _ := c.Get("user_id"); uid != uint(42) {
		t.Errorf("user_id = %v, want uint(42)", uid)
	}
	if name, _ := c.Get("username"); name != "alice" {
		t.Errorf("username = %v, want alice", name)
	}
	if role, _ := c.Get("user_role"); role != "admin" {
		t.Errorf("user_role = %v, want admin", role)
	}
}

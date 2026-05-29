package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	http        *http.Server
	cancel      context.CancelFunc
	shutdownFns []func()
}

func newServer(port string, r *gin.Engine, cancel context.CancelFunc, shutdownFns ...func()) *Server {
	return &Server{
		http: &http.Server{
			Addr:    fmt.Sprintf(":%s", port),
			Handler: r,
			// ReadTimeout bounds time to read the full request (headers + body).
			ReadTimeout: 15 * time.Second,
			// WriteTimeout must be zero: SSE (/api/v1/events) keeps the response open
			// for hours. A non-zero value closes the stream after N seconds (no heartbeats
			// would ever reach the client on a 30s ticker).
			WriteTimeout: 0,
			IdleTimeout:  120 * time.Second,
		},
		cancel:      cancel,
		shutdownFns: shutdownFns,
	}
}

func (s *Server) Run() error {
	return s.http.ListenAndServe()
}

// Shutdown stops background workers (by cancelling their root context), closes
// the SSE brokers, then gracefully drains in-flight HTTP requests within ctx's
// deadline. Returns http.Server.Shutdown's error.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	for _, fn := range s.shutdownFns {
		if fn != nil {
			fn()
		}
	}
	return s.http.Shutdown(ctx)
}

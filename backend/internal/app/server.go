package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	http *http.Server
}

func newServer(port string, r *gin.Engine) *Server {
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
	}
}

func (s *Server) Run() error {
	return s.http.ListenAndServe()
}

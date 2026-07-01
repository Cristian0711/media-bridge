package sse

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// keepAliveComment is an SSE comment line (ignored by clients) that prevents
// proxies from treating the connection as idle between JSON heartbeats.
const keepAliveComment = ": keepalive\n\n"

// Handler serves the unified app event stream (media + requests).
type Handler struct {
	broker *Broker
}

// NewHandler wires the SSE broker into HTTP.
func NewHandler(broker *Broker) *Handler {
	return &Handler{broker: broker}
}

// Stream keeps a long-lived connection open and pushes domain events.
// Requires auth middleware (X-User-ID / X-Username from nginx).
func (h *Handler) Stream(c *gin.Context) {
	if h.broker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sse not enabled"})
		return
	}

	clientID := uuid.NewString()
	client := NewClient(clientID)
	h.broker.AddClient(client)
	defer h.broker.RemoveClient(clientID)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	if _, err := io.WriteString(c.Writer, FormatConnected(clientID)); err != nil {
		return
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	done := c.Request.Context().Done()
	// Comment pings between broker heartbeats so bytes flow before the 30s JSON heartbeat.
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-done:
			return
		case <-ping.C:
			if _, err := io.WriteString(c.Writer, keepAliveComment); err != nil {
				return
			}
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		case msg, ok := <-client.Messages:
			if !ok {
				return
			}
			if _, err := io.WriteString(c.Writer, msg); err != nil {
				return
			}
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

package requests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const torrentStreamInterval = time.Second

// TorrentStreamEventType is the JSON "type" on torrent SSE frames.
type TorrentStreamEventType string

const (
	TorrentStreamConnected TorrentStreamEventType = "connected"
	TorrentStreamUpdate    TorrentStreamEventType = "torrent.update"
	TorrentStreamError     TorrentStreamEventType = "error"
)

type torrentStreamMessage struct {
	Type      TorrentStreamEventType `json:"type"`
	RequestID uint                   `json:"request_id,omitempty"`
	Payload   *RequestTorrentInfo    `json:"payload,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

func writeTorrentStreamEvent(w io.Writer, msg torrentStreamMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Type, data)
	return err
}

func (h *Handler) StreamRequestTorrent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request id"})
		return
	}
	requestID := uint(id)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, gin.H{"error": "streaming not supported"})
		return
	}

	if err := writeTorrentStreamEvent(c.Writer, torrentStreamMessage{
		Type:      TorrentStreamConnected,
		RequestID: requestID,
	}); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(torrentStreamInterval)
	defer ticker.Stop()

	ctx := c.Request.Context()
	push := func() bool {
		info, err := h.svc.GetRequestTorrentInfoFresh(ctx, requestID)
		if err != nil {
			msg := torrentStreamMessage{Type: TorrentStreamError, RequestID: requestID}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				msg.Error = "request not found"
				_ = writeTorrentStreamEvent(c.Writer, msg)
				flusher.Flush()
				return false
			}
			msg.Error = "failed to load torrent info"
			_ = writeTorrentStreamEvent(c.Writer, msg)
			flusher.Flush()
			return true
		}
		if err := writeTorrentStreamEvent(c.Writer, torrentStreamMessage{
			Type:      TorrentStreamUpdate,
			RequestID: requestID,
			Payload:   info,
		}); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !push() {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !push() {
				return
			}
		}
	}
}

// GetRequestTorrentInfoFresh loads live qBittorrent/hardlink state without the HTTP cache.
func (s *service) GetRequestTorrentInfoFresh(ctx context.Context, requestID uint) (*RequestTorrentInfo, error) {
	req, err := s.repo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	info, err := s.torrentInfo.build(ctx, req)
	if err != nil {
		return nil, err
	}
	s.torrentInfo.cache.set(requestID, info)
	return info, nil
}

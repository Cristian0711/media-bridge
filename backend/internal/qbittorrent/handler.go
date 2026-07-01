package qbittorrent

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc    Service
	broker *Broker
}

func NewHandler(svc Service, broker *Broker) *Handler {
	return &Handler{svc: svc, broker: broker}
}

func (h *Handler) AddTorrent(c *gin.Context) {
	var req AddTorrentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	file, err := base64.StdEncoding.DecodeString(req.TorrentBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 torrent data"})
		return
	}

	resp, err := h.svc.AddTorrent(c.Request.Context(), file, req.SavePath, req.TorrentName)
	if err != nil {
		if errors.Is(err, ErrTorrentExists) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "hash": resp.Hash, "path": resp.Path})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add torrent"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) RemoveTorrent(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hash is required"})
		return
	}
	if err := h.svc.RemoveTorrent(c.Request.Context(), hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove torrent"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) ListTorrents(c *gin.Context) {
	pageQuery := c.Query("page")
	pageSizeQuery := c.Query("page_size")
	if pageQuery != "" || pageSizeQuery != "" {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		resp, err := h.svc.ListTorrentsPaginated(c.Request.Context(), page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list torrents"})
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	torrents, err := h.svc.ListTorrents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list torrents"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"torrents": torrents})
}

func (h *Handler) GetTorrentFiles(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hash is required"})
		return
	}
	files, err := h.svc.GetTorrentFiles(c.Request.Context(), hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get torrent files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (h *Handler) GetTorrent(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hash is required"})
		return
	}
	torrent, err := h.svc.GetTorrent(c.Request.Context(), hash)
	if err != nil {
		if errors.Is(err, ErrTorrentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "torrent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get torrent"})
		return
	}
	c.JSON(http.StatusOK, torrent)
}

func (h *Handler) GetTorrentStatus(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hash is required"})
		return
	}
	status, err := h.svc.GetTorrentStatus(c.Request.Context(), hash)
	if err != nil {
		if errors.Is(err, ErrTorrentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "torrent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get torrent status"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) TorrentEvents(c *gin.Context) {
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

	_, _ = io.WriteString(c.Writer, fmt.Sprintf("event: connected\ndata: {\"client_id\":\"%s\"}\n\n", clientID))
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	done := c.Request.Context().Done()
	for {
		select {
		case <-done:
			return
		case msg, ok := <-client.Messages:
			if !ok {
				return
			}
			_, err := io.WriteString(c.Writer, msg)
			if err != nil {
				return
			}
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

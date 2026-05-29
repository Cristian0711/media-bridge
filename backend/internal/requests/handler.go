package requests

import (
	"errors"
	"strconv"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func requestMeta(c *gin.Context) (uint, string, string) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	requestID := c.GetString("request_id")
	uid, _ := userID.(uint)
	uname, _ := username.(string)
	return uid, uname, requestID
}

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) MovieDownload(c *gin.Context) {
	var req MovieDownloadRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	userID, username, requestID := requestMeta(c)
	resp, err := h.svc.RequestMovieDownload(c.Request.Context(), req, userID, username, requestID)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(202, resp)
}

func (h *Handler) ShowDownload(c *gin.Context) {
	var req ShowDownloadRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	userID, username, requestID := requestMeta(c)
	resp, err := h.svc.RequestShowDownload(c.Request.Context(), req, userID, username, requestID)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(202, resp)
}

func (h *Handler) MovieRemove(c *gin.Context) {
	var req MovieRemoveRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	userID, username, requestID := requestMeta(c)
	resp, err := h.svc.RequestMovieRemove(c.Request.Context(), req, userID, username, requestID)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(202, resp)
}

func (h *Handler) ShowRemove(c *gin.Context) {
	var req ShowRemoveRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	userID, username, requestID := requestMeta(c)
	resp, err := h.svc.RequestShowRemove(c.Request.Context(), req, userID, username, requestID)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(202, resp)
}

func (h *Handler) ListRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.ListRequests(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, resp)
}

func (h *Handler) ListMyRequests(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.ListRequestsForUser(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, resp)
}

func userIDFromContext(c *gin.Context) (uint, bool) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(401, gin.H{"error": "user not authenticated"})
		return 0, false
	}
	userID, ok := userIDAny.(uint)
	if !ok {
		c.JSON(401, gin.H{"error": "invalid user context"})
		return 0, false
	}
	return userID, true
}

func (h *Handler) GetRequestTorrent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request id"})
		return
	}
	info, err := h.svc.GetRequestTorrentInfo(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "request not found"})
			return
		}
		if errors.Is(err, media.ErrMediaNotFound) {
			c.JSON(404, gin.H{"error": "media not found"})
			return
		}
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, info)
}

func (h *Handler) ListQueueEntries(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.ListQueueEntries(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, resp)
}

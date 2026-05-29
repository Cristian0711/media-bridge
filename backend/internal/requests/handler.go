package requests

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/requests")
	g.GET("", h.ListRequests)
	g.GET("/my", h.ListMyRequests)
	g.GET("/queue", h.ListQueueEntries)
	g.GET("/:id/torrent/events", h.StreamRequestTorrent)
	g.GET("/:id/torrent", h.GetRequestTorrent)
	g.POST("/movies/download", h.MovieDownload)
	g.POST("/shows/download", h.ShowDownload)
	g.POST("/movies/remove", h.MovieRemove)
	g.POST("/shows/remove", h.ShowRemove)
}

// requestMeta pulls the authenticated user and request id placed on the context
// by upstream middleware.
func requestMeta(c *gin.Context) (userID uint, username, requestID string) {
	uid, _ := c.Get("user_id")
	uname, _ := c.Get("username")
	userID, _ = uid.(uint)
	username, _ = uname.(string)
	return userID, username, c.GetString("request_id")
}

// handlePost binds the JSON body of type T, dispatches to a service call, and
// writes the standard ack/error response shared by all four POST endpoints.
func handlePost[T any](
	c *gin.Context,
	call func(ctx context.Context, body T, userID uint, username, requestID string) (*RequestAck, error),
) {
	var body T
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, username, requestID := requestMeta(c)
	resp, err := call(c.Request.Context(), body, userID, username, requestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusAccepted, resp)
}

func (h *Handler) MovieDownload(c *gin.Context) { handlePost(c, h.svc.RequestMovieDownload) }
func (h *Handler) ShowDownload(c *gin.Context)  { handlePost(c, h.svc.RequestShowDownload) }
func (h *Handler) MovieRemove(c *gin.Context)   { handlePost(c, h.svc.RequestMovieRemove) }
func (h *Handler) ShowRemove(c *gin.Context)    { handlePost(c, h.svc.RequestShowRemove) }

func (h *Handler) ListRequests(c *gin.Context) {
	page, pageSize := paginationParams(c)
	resp, err := h.svc.ListRequests(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListMyRequests(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	page, pageSize := paginationParams(c)
	resp, err := h.svc.ListRequestsForUser(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListQueueEntries(c *gin.Context) {
	page, pageSize := paginationParams(c)
	resp, err := h.svc.ListQueueEntries(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetRequestTorrent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}
	info, err := h.svc.GetRequestTorrentInfo(c.Request.Context(), uint(id))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		case errors.Is(err, media.ErrMediaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, info)
}

func userIDFromContext(c *gin.Context) (uint, bool) {
	userIDAny, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return 0, false
	}
	userID, ok := userIDAny.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
		return 0, false
	}
	return userID, true
}

func paginationParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return page, pageSize
}

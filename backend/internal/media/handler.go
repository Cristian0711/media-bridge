package media

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetAllMedia(c *gin.Context) {
	page, pageSize := parsePaginationParams(c)
	resp, err := h.svc.GetAllMediaPaginated(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch media"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetMyMedia(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	page, pageSize := parsePaginationParams(c)
	resp, err := h.svc.GetMediaForUserPaginated(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch media"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetMediaByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media id"})
		return
	}
	row, err := h.svc.GetMediaByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, ErrMediaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch media"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"media": row})
}

func (h *Handler) SearchMedia(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query is required"})
		return
	}
	page, pageSize := parsePaginationParams(c)
	resp, err := h.svc.SearchMedia(c.Request.Context(), query, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search media"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SearchMyMedia(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query is required"})
		return
	}
	page, pageSize := parsePaginationParams(c)
	resp, err := h.svc.SearchMediaForUser(c.Request.Context(), userID, query, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search media"})
		return
	}
	c.JSON(http.StatusOK, resp)
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

// parsePaginationParams parses page/page_size from the query. Range clamping is
// left to the service layer (pagination.Normalize), so the cap is defined in one
// place rather than silently differing here.
func parsePaginationParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return page, pageSize
}

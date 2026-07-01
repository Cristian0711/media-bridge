package users

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

func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, ok := userID.(uint)
	if !ok || id == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.svc.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
}

// canReadUser reports whether the caller (identified by the proxy-trusted
// context values) may read the user record with targetID: admins may read any,
// other users only their own.
func canReadUser(c *gin.Context, targetID uint) bool {
	if role, ok := c.Get("user_role"); ok {
		if r, ok := role.(string); ok && IsAdmin(r) {
			return true
		}
	}
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok && id == targetID {
			return true
		}
	}
	return false
}

func (h *Handler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	// Only the user themselves or an admin may read a user record (avoid an
	// unrestricted IDOR-style read of any account by id).
	if !canReadUser(c, uint(id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	user, err := h.svc.FindByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
}

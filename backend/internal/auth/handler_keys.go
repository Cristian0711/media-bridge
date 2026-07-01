package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func userIDFromContext(c *gin.Context) (uint, bool) {
	userID, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok && id > 0
}

func (h *Handler) writeKeyServiceError(c *gin.Context, err error) {
	if errors.Is(err, ErrForbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, ErrKeyInvalid) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func (h *Handler) GenerateKey(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	resp, err := h.svc.GenerateKey(c.Request.Context())
	if err != nil {
		h.writeKeyServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListKeys(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	resp, err := h.svc.ListKeys(c.Request.Context())
	if err != nil {
		h.writeKeyServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetKeyStatus(c *gin.Context) {
	if _, ok := userIDFromContext(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	resp, err := h.svc.GetKeyStatus(c.Request.Context(), c.Param("value"))
	if err != nil {
		h.writeKeyServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

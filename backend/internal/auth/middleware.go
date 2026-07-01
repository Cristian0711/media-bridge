package auth

import (
	"net/http"

	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"github.com/gin-gonic/gin"
)

// AdminMiddleware gates routes using X-User-Role from nginx (same trust model as X-User-ID).
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !users.IsAdmin(c.GetHeader("X-User-Role")) {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrForbidden.Error()})
			c.Abort()
			return
		}
		c.Next()
	}
}

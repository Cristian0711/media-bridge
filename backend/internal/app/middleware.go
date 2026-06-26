package app

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// proxyAuthMiddleware does NOT authenticate — it trusts the X-User-ID /
// X-Username / X-User-Role headers injected by the nginx auth layer, which
// validates the JWT and overwrites these headers on every request. This is only
// safe because the backend is never exposed directly; nginx unconditionally
// sets (and thereby strips any client-supplied) X-User-* headers. If the
// backend ever becomes directly reachable, real in-process JWT validation must
// replace this.
func proxyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)
		c.Request.Header.Set("X-Request-ID", requestID)
		c.Set("request_id", requestID)

		userIDHeader := c.GetHeader("X-User-ID")
		username := c.GetHeader("X-Username")
		if userIDHeader == "" || username == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth proxy headers"})
			c.Abort()
			return
		}

		userID, err := strconv.ParseUint(userIDHeader, 10, 64)
		if err != nil || userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user header"})
			c.Abort()
			return
		}

		c.Set("user_id", uint(userID))
		c.Set("username", username)
		if role := c.GetHeader("X-User-Role"); role != "" {
			c.Set("user_role", role)
		}

		c.Next()
	}
}

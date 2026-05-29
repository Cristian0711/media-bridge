package app

import (
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func requestLoggerMiddleware() gin.HandlerFunc {
	log := logger.Named("app.middleware.request")
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		// Skip internal auth-validation probes made by Nginx.
		if c.GetHeader("X-Internal-Auth-Check") == "1" {
			return
		}

		requestID := c.GetString("request_id")
		if requestID == "" {
			requestID = c.GetHeader("X-Request-ID")
		}

		username := c.GetString("username")
		if username == "" {
			username = c.GetHeader("X-Username")
		}

		userID, ok := c.Get("user_id")
		if !ok {
			userID = c.GetHeader("X-User-ID")
		}

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		log.Info("request",
			zap.String("route", route),
			zap.String("method", c.Request.Method),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("duration_ms", time.Since(start).Milliseconds()),
			zap.Any("params", c.Params),
			zap.Any("query", c.Request.URL.Query()),
			zap.String("request_id", requestID),
			zap.Any("user_id", userID),
			zap.String("username", username),
		)
	}
}

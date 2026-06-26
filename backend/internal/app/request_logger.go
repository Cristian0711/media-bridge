package app

import (
	"net/url"
	"strings"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sensitiveQueryFragments are substrings that mark a query parameter as
// secret. Matching params are redacted before logging so API keys, tokens, and
// passwords passed in the URL never land in logs.
var sensitiveQueryFragments = []string{"apikey", "api_key", "token", "password", "passwd", "secret", "auth", "key"}

func redactQuery(q url.Values) map[string][]string {
	out := make(map[string][]string, len(q))
	for k, vs := range q {
		lk := strings.ToLower(k)
		sensitive := false
		for _, frag := range sensitiveQueryFragments {
			if strings.Contains(lk, frag) {
				sensitive = true
				break
			}
		}
		if sensitive {
			out[k] = []string{"[REDACTED]"}
			continue
		}
		out[k] = vs
	}
	return out
}

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
			zap.Any("query", redactQuery(c.Request.URL.Query())),
			zap.String("request_id", requestID),
			zap.Any("user_id", userID),
			zap.String("username", username),
		)
	}
}

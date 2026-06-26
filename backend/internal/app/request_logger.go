package app

import (
	"net/url"
	"strings"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/gin-gonic/gin"
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
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		// Skip internal auth-validation probes made by Nginx.
		if c.GetHeader("X-Internal-Auth-Check") == "1" {
			return
		}

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		status := c.Writer.Status()
		// request_id and the actor (user_id / username / role) are added by the
		// logger's context handler — the ctx was seeded by contextMiddleware /
		// proxyAuthMiddleware — so they are not repeated here.
		ctx := c.Request.Context()
		args := []any{
			"route", route,
			"method", c.Request.Method,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"params", c.Params,
			"query", redactQuery(c.Request.URL.Query()),
		}
		// Cloudflare correlation: cf_ray joins this line to Cloudflare's logs;
		// client_ip is the real client (CF-Connecting-IP), not the tunnel.
		if ip := c.GetString("client_ip"); ip != "" {
			args = append(args, "client_ip", ip)
		}
		if ray := c.GetString("cf_ray"); ray != "" {
			args = append(args, "cf_ray", ray)
		}

		// Status-derived level: a 5xx is a server-side failure (an ERROR worth
		// reviewing); 4xx is the client's mistake (WARN); everything else is INFO.
		switch {
		case status >= 500:
			logger.Error(ctx, "http.server_error", "request", nil, args...)
		case status >= 400:
			logger.Warn(ctx, "request", args...)
		default:
			logger.Info(ctx, "request", args...)
		}
	}
}

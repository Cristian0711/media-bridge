package app

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// annotateSpan copies the request id and actor onto the active span, so a trace
// is attributable to a person (or marked system/anonymous) the same way the
// logs are.
func annotateSpan(ctx context.Context, requestID string, a logger.Actor) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{attribute.String("actor.type", string(a.Type))}
	if requestID != "" {
		attrs = append(attrs, attribute.String("app.request_id", requestID))
	}
	if a.UserID != 0 {
		attrs = append(attrs, attribute.Int64("enduser.id", int64(a.UserID)))
	}
	if a.Username != "" {
		attrs = append(attrs, attribute.String("app.username", a.Username))
	}
	if a.Role != "" {
		attrs = append(attrs, attribute.String("enduser.role", a.Role))
	}
	span.SetAttributes(attrs...)
}

// contextMiddleware runs first for every route (public and protected). It
// resolves the request id (nginx X-Request-ID, or a generated one) and seeds the
// request context with that id and a default anonymous actor, so the structured
// logger tags every downstream log line. proxyAuthMiddleware later upgrades the
// actor to the authenticated user.
func contextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)
		c.Request.Header.Set("X-Request-ID", requestID)
		c.Set("request_id", requestID)

		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		ctx = logger.WithActor(ctx, logger.AnonymousActor())
		c.Request = c.Request.WithContext(ctx)
		annotateSpan(ctx, requestID, logger.AnonymousActor())

		c.Next()
	}
}

// proxyAuthMiddleware does NOT authenticate — it trusts the X-User-ID /
// X-Username / X-User-Role headers injected by the nginx auth layer, which
// validates the JWT and overwrites these headers on every request. This is only
// safe because the backend is never exposed directly; nginx unconditionally
// sets (and thereby strips any client-supplied) X-User-* headers. If the
// backend ever becomes directly reachable, real in-process JWT validation must
// replace this.
func proxyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
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

		role := c.GetHeader("X-User-Role")
		c.Set("user_id", uint(userID))
		c.Set("username", username)
		if role != "" {
			c.Set("user_role", role)
		}

		// Upgrade the context actor from anonymous (seeded by contextMiddleware)
		// to the authenticated user, so logs and spans downstream attribute the
		// work to this person.
		actor := logger.UserActor(uint(userID), username, role)
		ctx := logger.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		annotateSpan(ctx, logger.RequestID(ctx), actor)

		c.Next()
	}
}

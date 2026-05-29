package sse

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the single app-wide SSE endpoint.
func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	r.GET("/events", h.Stream)
}

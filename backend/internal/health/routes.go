package health

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/health")
	g.GET("", h.Report)
	g.GET("/scans", h.ListScans)
	g.GET("/scans/latest", h.LatestScan)
	g.GET("/scans/:id", h.GetScan)
}

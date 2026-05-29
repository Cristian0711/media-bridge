package requests

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/requests")
	g.GET("", h.ListRequests)
	g.GET("/my", h.ListMyRequests)
	g.GET("/queue", h.ListQueueEntries)
	g.GET("/:id/torrent/events", h.StreamRequestTorrent)
	g.GET("/:id/torrent", h.GetRequestTorrent)
	g.POST("/movies/download", h.MovieDownload)
	g.POST("/shows/download", h.ShowDownload)
	g.POST("/movies/remove", h.MovieRemove)
	g.POST("/shows/remove", h.ShowRemove)
}

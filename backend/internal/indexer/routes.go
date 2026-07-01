package indexer

import (
	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/indexer")
	g.GET("/indexers", h.ListIndexers)
	g.GET("/search/movies", h.SearchMovies)
	g.GET("/search/shows", h.SearchShows)
	g.GET("/search/movies/best", h.FindBestMovie)
	g.GET("/search/shows/best", h.FindBestShow)
	g.POST("/download", h.DownloadTorrent)

	// Per-indexer configuration is admin-only.
	settings := g.Group("/settings")
	settings.Use(auth.AdminMiddleware())
	settings.GET("", h.ListIndexerSettings)
	settings.PUT("", h.UpdateIndexerSetting)
}

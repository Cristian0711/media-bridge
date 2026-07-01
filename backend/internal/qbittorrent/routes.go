package qbittorrent

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/qbittorrent/torrents")
	g.POST("", h.AddTorrent)
	g.GET("", h.ListTorrents)
	g.GET("/events", h.TorrentEvents)
	g.DELETE("/:hash", h.RemoveTorrent)
	g.GET("/:hash/files", h.GetTorrentFiles)
	g.GET("/:hash/status", h.GetTorrentStatus)
	g.GET("/:hash", h.GetTorrent)
}

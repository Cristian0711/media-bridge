package media

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/media")
	g.GET("/list", h.GetAllMedia)
	g.GET("/list/my", h.GetMyMedia)
	g.GET("/:id", h.GetMediaByID)
	g.GET("/search", h.SearchMedia)
	g.GET("/search/my", h.SearchMyMedia)
}

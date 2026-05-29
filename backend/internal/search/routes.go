package search

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	r.GET("/search", h.Search)
	r.GET("/search/external-ids", h.ExternalIDs)
	r.GET("/browse/services", h.BrowseServices)
	r.GET("/browse/services/:serviceId/lists", h.BrowseServiceLists)
	r.GET("/browse/lists", h.BrowseGlobalLists)
	r.GET("/browse/:id", h.BrowseList)

	g := r.Group("/search")
	g.GET("/movies", h.SearchMovies)
	g.GET("/shows", h.SearchShows)
}

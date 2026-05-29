package users

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/users")
	g.GET("/me", h.GetMe)
	g.GET("/:id", h.GetUser)
}

package auth

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/auth")
	g.POST("/login", h.Login)
	g.POST("/register", h.Register)
}

func RegisterProtectedRoutes(r *gin.RouterGroup, h *Handler) {
	keys := r.Group("/keys")
	keys.POST("/generate", h.GenerateKey)
	keys.GET("/:value/validate", h.GetKeyStatus)
}

func RegisterValidationRoute(r *gin.RouterGroup, h *Handler) {
	g := r.Group("/auth")
	g.GET("/validate", h.ValidateToken)
}
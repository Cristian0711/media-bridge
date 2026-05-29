package app

import (
	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/Cristian0711/media-bridge/backend/internal/health"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	"github.com/Cristian0711/media-bridge/backend/internal/search"
	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"github.com/gin-gonic/gin"
)

func newRouter(
	authH *auth.Handler,
	userH *users.Handler,
	qbitH *qbittorrent.Handler,
	indexerH *indexer.Handler,
	mediaH *media.Handler,
	requestsH *requests.Handler,
	searchH *search.Handler,
	sseH *sse.Handler,
	healthH *health.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLoggerMiddleware())

	api := r.Group("/api/v1")
	auth.RegisterPublicRoutes(api, authH)
	auth.RegisterValidationRoute(api, authH)

	protected := r.Group("/api/v1")
	protected.Use(authMiddleware())
	auth.RegisterAdminKeyRoutes(protected, authH)
	users.RegisterRoutes(protected, userH)
	qbittorrent.RegisterRoutes(protected, qbitH)
	indexer.RegisterRoutes(protected, indexerH)
	media.RegisterRoutes(protected, mediaH)
	requests.RegisterRoutes(protected, requestsH)
	search.RegisterRoutes(protected, searchH)
	sse.RegisterRoutes(protected, sseH)
	health.RegisterRoutes(protected, healthH)

	return r
}

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
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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
	// otelgin is outermost so a span exists for the whole request; it continues
	// the trace started at nginx (via the W3C traceparent header) when present.
	// SSE streams and the internal auth probe are excluded — a long-lived stream
	// would otherwise produce one sprawling, never-ending span per open
	// connection (see skipTracing).
	r.Use(otelgin.Middleware("media-bridge-backend", otelgin.WithGinFilter(func(c *gin.Context) bool {
		return !skipTracing(c)
	})))
	r.Use(contextMiddleware())
	r.Use(requestLoggerMiddleware())

	api := r.Group("/api/v1")
	auth.RegisterPublicRoutes(api, authH)
	auth.RegisterValidationRoute(api, authH)

	protected := r.Group("/api/v1")
	protected.Use(proxyAuthMiddleware())
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

package search

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Search(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter is required"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	pageResult, err := h.svc.Search(c.Request.Context(), query, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search"})
		return
	}
	results := h.svc.annotateResults(c.Request.Context(), pageResult.Results)
	if results == nil {
		results = []Result{}
	}
	c.Header("X-Search-Page", strconv.Itoa(pageResult.Page))
	c.Header("X-Search-Total-Pages", strconv.Itoa(pageResult.TotalPages))
	c.JSON(http.StatusOK, results)
}

func (h *Handler) ExternalIDs(c *gin.Context) {
	mediaType := c.Query("type")
	tmdbID, err := strconv.Atoi(c.Query("tmdb_id"))
	if mediaType == "" || err != nil || tmdbID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and tmdb_id are required"})
		return
	}
	ids, err := h.svc.ResolveExternalIDs(c.Request.Context(), mediaType, tmdbID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ids)
}

func (h *Handler) SearchMovies(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter is required"})
		return
	}

	movies, err := h.svc.SearchMovies(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch movies"})
		return
	}
	c.JSON(http.StatusOK, movies)
}

func (h *Handler) BrowseCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.BrowseCatalog())
}

func (h *Handler) BrowseServices(c *gin.Context) {
	services, err := h.svc.BrowseServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load services"})
		return
	}
	if services == nil {
		services = []BrowseService{}
	}
	c.JSON(http.StatusOK, services)
}

func (h *Handler) BrowseServiceLists(c *gin.Context) {
	serviceID := c.Param("serviceId")
	lists, err := h.svc.BrowseServiceLists(serviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lists)
}

func (h *Handler) BrowseServiceCatalog(c *gin.Context) {
	serviceID := c.Param("serviceId")
	catalog, err := h.svc.BrowseServiceCatalog(c.Request.Context(), serviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	catalog = h.svc.annotateCatalog(c.Request.Context(), catalog)
	if catalog.Lists == nil {
		catalog.Lists = []BrowseListRow{}
	}
	c.JSON(http.StatusOK, catalog)
}

func (h *Handler) BrowseGlobalLists(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.BrowseGlobalLists())
}

func (h *Handler) BrowseGlobalCatalog(c *gin.Context) {
	catalog, err := h.svc.BrowseGlobalCatalog(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load global browse"})
		return
	}
	catalog = h.svc.annotateCatalog(c.Request.Context(), catalog)
	if catalog.Lists == nil {
		catalog.Lists = []BrowseListRow{}
	}
	c.JSON(http.StatusOK, catalog)
}

func (h *Handler) BrowseList(c *gin.Context) {
	listID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	pageResult, err := h.svc.Browse(c.Request.Context(), listID, page)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	results := h.svc.annotateResults(c.Request.Context(), pageResult.Results)
	if results == nil {
		results = []Result{}
	}
	c.Header("X-Search-Page", strconv.Itoa(pageResult.Page))
	c.Header("X-Search-Total-Pages", strconv.Itoa(pageResult.TotalPages))
	c.JSON(http.StatusOK, results)
}

func (h *Handler) SearchShows(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter is required"})
		return
	}

	shows, err := h.svc.SearchShows(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch shows"})
		return
	}
	c.JSON(http.StatusOK, shows)
}

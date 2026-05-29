package indexer

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListIndexers(c *gin.Context) {
	type indexerInfo struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	indexers := h.svc.ListIndexers()
	out := make([]indexerInfo, 0, len(indexers))
	for _, idx := range indexers {
		out = append(out, indexerInfo{ID: idx.GetID(), Name: idx.GetName(), Enabled: idx.IsEnabled()})
	}
	c.JSON(http.StatusOK, gin.H{"indexers": out})
}

func (h *Handler) SearchMovies(c *gin.Context) {
	req, ok := bindSearchRequest(c, true)
	if !ok {
		return
	}
	resp, err := h.svc.SearchMovies(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search movies"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) SearchShows(c *gin.Context) {
	req, ok := bindSearchRequest(c, false)
	if !ok {
		return
	}
	resp, err := h.svc.SearchShows(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search shows"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) FindBestMovie(c *gin.Context) {
	req, ok := bindSearchRequest(c, true)
	if !ok {
		return
	}
	if req.Quality == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quality is required"})
		return
	}
	best, err := h.svc.FindBestMovie(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"movie": best})
}

func (h *Handler) FindBestShow(c *gin.Context) {
	req, ok := bindSearchRequest(c, false)
	if !ok {
		return
	}
	if req.Quality == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quality is required"})
		return
	}
	best, err := h.svc.FindBestShow(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"show": best})
}

func (h *Handler) DownloadTorrent(c *gin.Context) {
	var req DownloadTorrentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	torrentBase64, err := h.svc.DownloadTorrent(c.Request.Context(), req.Indexer, req.DownloadURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"torrent_base64": torrentBase64})
}

func bindSearchRequest(c *gin.Context, movie bool) (SearchRequest, bool) {
	query := SearchQueryRequest{
		ImdbID:   c.Query("imdb_id"),
		Season:   c.Query("season"),
		Episode:  c.Query("episode"),
		Quality:  c.Query("quality"),
		Indexers: c.Query("indexers"),
	}
	return query.ToSearchRequest(c, movie)
}

package indexer

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type SearchQueryRequest struct {
	ImdbID   string `form:"imdb_id"`
	Season   string `form:"season"`
	Episode  string `form:"episode"`
	Quality  string `form:"quality"`
	Indexers string `form:"indexers"`
}

type DownloadTorrentRequest struct {
	Indexer     string `json:"indexer" binding:"required"`
	DownloadURL string `json:"download_url" binding:"required"`
}

func (r SearchQueryRequest) ToSearchRequest(c *gin.Context, movie bool) (SearchRequest, bool) {
	imdbID := strings.TrimSpace(r.ImdbID)
	if imdbID == "" {
		c.JSON(400, gin.H{"error": "imdb_id is required"})
		return SearchRequest{}, false
	}

	req := SearchRequest{
		ImdbID:  imdbID,
		Quality: r.Quality,
	}

	if v := strings.TrimSpace(r.Indexers); v != "" {
		req.Indexers = strings.Split(v, ",")
	}

	if !movie {
		if sStr := strings.TrimSpace(r.Season); sStr != "" {
			s, err := strconv.Atoi(sStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid season parameter"})
				return SearchRequest{}, false
			}
			req.Season = s
		}
		if eStr := strings.TrimSpace(r.Episode); eStr != "" {
			e, err := strconv.Atoi(eStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid episode parameter"})
				return SearchRequest{}, false
			}
			req.Episode = e
		}
	}

	return req, true
}

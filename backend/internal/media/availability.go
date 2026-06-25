package media

import (
	"context"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
)

// maxAvailabilityItems bounds a single availability request so one page of
// search/discover results can't trigger an unbounded query.
const maxAvailabilityItems = 300

// AvailabilityItem identifies one title to check against the library. Season is
// optional and only meaningful for shows (nil = any part of the show counts).
type AvailabilityItem struct {
	Type   string `json:"type"` // "movie" | "show"
	IMDBID string `json:"imdb_id,omitempty"`
	TMDBID string `json:"tmdb_id,omitempty"`
	TVDBID string `json:"tvdb_id,omitempty"`
	Season *int   `json:"season,omitempty"`
}

type AvailabilityRequest struct {
	Items []AvailabilityItem `json:"items"`
}

// AvailabilityResult reports whether a title is on the server and at which
// qualities. Results are returned in the same order as the request items.
type AvailabilityResult struct {
	Available bool     `json:"available"`
	Qualities []string `json:"qualities"`
}

type AvailabilityResponse struct {
	Results []AvailabilityResult `json:"results"`
}

// movieQualityRow / showQualityRow are flat projections for the batch lookups.
type movieQualityRow struct {
	IMDBID  string `gorm:"column:imdb_id"`
	TMDBID  string `gorm:"column:tmdb_id"`
	Quality string `gorm:"column:quality"`
}

type showQualityRow struct {
	IMDBID  string `gorm:"column:imdb_id"`
	TVDBID  string `gorm:"column:tvdb_id"`
	Season  *int   `gorm:"column:season"`
	Quality string `gorm:"column:quality"`
}

// MovieQualitiesByExternalIDs returns one row per (movie, quality) for any movie
// matching one of the given imdb or tmdb ids. A single indexed query.
func (r *repository) MovieQualitiesByExternalIDs(ctx context.Context, imdbIDs, tmdbIDs []string) ([]movieQualityRow, error) {
	if len(imdbIDs) == 0 && len(tmdbIDs) == 0 {
		return nil, nil
	}
	q := r.db.WithContext(ctx).
		Model(&Media{}).
		Select("movies.imdb_id AS imdb_id, movies.tmdb_id AS tmdb_id, media.quality AS quality").
		Joins("JOIN movies ON movies.id = media.movie_id").
		Where("media.type = ?", MediaTypeMovie)

	switch {
	case len(imdbIDs) > 0 && len(tmdbIDs) > 0:
		q = q.Where("(movies.imdb_id IN ? OR movies.tmdb_id IN ?)", imdbIDs, tmdbIDs)
	case len(imdbIDs) > 0:
		q = q.Where("movies.imdb_id IN ?", imdbIDs)
	default:
		q = q.Where("movies.tmdb_id IN ?", tmdbIDs)
	}

	var rows []movieQualityRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ShowQualitiesByExternalIDs returns one row per (show entry, quality) for any
// show matching one of the given imdb or tvdb ids, carrying the entry season so
// callers can scope to a specific season. A single indexed query.
func (r *repository) ShowQualitiesByExternalIDs(ctx context.Context, imdbIDs, tvdbIDs []string) ([]showQualityRow, error) {
	if len(imdbIDs) == 0 && len(tvdbIDs) == 0 {
		return nil, nil
	}
	q := r.db.WithContext(ctx).
		Model(&Media{}).
		Select("shows.imdb_id AS imdb_id, shows.tvdb_id AS tvdb_id, show_entries.season AS season, media.quality AS quality").
		Joins("JOIN show_entries ON show_entries.id = media.show_entry_id").
		Joins("JOIN shows ON shows.id = show_entries.show_id")

	switch {
	case len(imdbIDs) > 0 && len(tvdbIDs) > 0:
		q = q.Where("(shows.imdb_id IN ? OR shows.tvdb_id IN ?)", imdbIDs, tvdbIDs)
	case len(imdbIDs) > 0:
		q = q.Where("shows.imdb_id IN ?", imdbIDs)
	default:
		q = q.Where("shows.tvdb_id IN ?", tvdbIDs)
	}

	var rows []showQualityRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CheckAvailability resolves, for each requested title, whether it exists in the
// library (server-wide) and the distinct qualities present. It issues at most
// two queries total (movies, shows) regardless of item count.
func (s *service) CheckAvailability(ctx context.Context, items []AvailabilityItem) ([]AvailabilityResult, error) {
	results := make([]AvailabilityResult, len(items))
	for i := range results {
		results[i] = AvailabilityResult{Available: false, Qualities: []string{}}
	}

	movieImdb := map[string]struct{}{}
	movieTmdb := map[string]struct{}{}
	showImdb := map[string]struct{}{}
	showTvdb := map[string]struct{}{}

	for _, it := range items {
		switch it.Type {
		case "movie":
			addNonEmpty(movieImdb, it.IMDBID)
			addNonEmpty(movieTmdb, it.TMDBID)
		case "show":
			addNonEmpty(showImdb, it.IMDBID)
			addNonEmpty(showTvdb, it.TVDBID)
		}
	}

	movieByImdb := map[string]map[string]struct{}{}
	movieByTmdb := map[string]map[string]struct{}{}
	if len(movieImdb) > 0 || len(movieTmdb) > 0 {
		rows, err := s.repo.MovieQualitiesByExternalIDs(ctx, setKeys(movieImdb), setKeys(movieTmdb))
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.IMDBID != "" {
				addToSetMap(movieByImdb, row.IMDBID, row.Quality)
			}
			if row.TMDBID != "" {
				addToSetMap(movieByTmdb, row.TMDBID, row.Quality)
			}
		}
	}

	showByImdb := map[string][]showQualityRow{}
	showByTvdb := map[string][]showQualityRow{}
	if len(showImdb) > 0 || len(showTvdb) > 0 {
		rows, err := s.repo.ShowQualitiesByExternalIDs(ctx, setKeys(showImdb), setKeys(showTvdb))
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.IMDBID != "" {
				showByImdb[row.IMDBID] = append(showByImdb[row.IMDBID], row)
			}
			if row.TVDBID != "" {
				showByTvdb[row.TVDBID] = append(showByTvdb[row.TVDBID], row)
			}
		}
	}

	for i, it := range items {
		quals := map[string]struct{}{}
		switch it.Type {
		case "movie":
			if it.IMDBID != "" {
				mergeSet(quals, movieByImdb[it.IMDBID])
			}
			if it.TMDBID != "" {
				mergeSet(quals, movieByTmdb[it.TMDBID])
			}
		case "show":
			var rows []showQualityRow
			if it.IMDBID != "" {
				rows = append(rows, showByImdb[it.IMDBID]...)
			}
			if it.TVDBID != "" {
				rows = append(rows, showByTvdb[it.TVDBID]...)
			}
			for _, row := range rows {
				if it.Season != nil && (row.Season == nil || *row.Season != *it.Season) {
					continue
				}
				quals[row.Quality] = struct{}{}
			}
		}
		sorted := sortedSetKeys(quals)
		results[i] = AvailabilityResult{Available: len(sorted) > 0, Qualities: sorted}
	}

	return results, nil
}

// CheckAvailability validates the body and returns per-item availability.
func (h *Handler) CheckAvailability(c *gin.Context) {
	var req AvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusOK, AvailabilityResponse{Results: []AvailabilityResult{}})
		return
	}
	if len(req.Items) > maxAvailabilityItems {
		req.Items = req.Items[:maxAvailabilityItems]
	}
	results, err := h.svc.CheckAvailability(c.Request.Context(), req.Items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check availability"})
		return
	}
	c.JSON(http.StatusOK, AvailabilityResponse{Results: results})
}

func addNonEmpty(set map[string]struct{}, v string) {
	if v != "" {
		set[v] = struct{}{}
	}
}

func setKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

func addToSetMap(m map[string]map[string]struct{}, key, val string) {
	if m[key] == nil {
		m[key] = map[string]struct{}{}
	}
	m[key][val] = struct{}{}
}

func mergeSet(dst, src map[string]struct{}) {
	for k := range src {
		dst[k] = struct{}{}
	}
}

func sortedSetKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

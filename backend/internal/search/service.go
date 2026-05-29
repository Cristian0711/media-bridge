package search

import (
	"context"
	"fmt"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

// TMDBConfig holds credentials for The Movie Database API.
type TMDBConfig struct {
	BaseURL string
	APIKey  string
}

// SearchPage holds one page of results plus pagination metadata for response headers.
type SearchPage struct {
	Results    []Result
	Page       int
	TotalPages int
}

// ExternalIDs is resolved from TMDB when the user downloads or searches torrents.
// IMDb is required for indexer search; TVDB (shows) and TMDB (movies) are included when available.
type ExternalIDs struct {
	IMDBID string `json:"imdb_id,omitempty"`
	TMDBID int    `json:"tmdb_id,omitempty"`
	TVDBID int    `json:"tvdb_id,omitempty"`
}

type Service struct {
	tmdb       *tmdbClient
	log        *zap.Logger
	logoCache  providerLogoCache
	browseCache *browseCache
}

func NewService(cfg TMDBConfig) *Service {
	return &Service{
		tmdb:        newTMDBClient(cfg),
		log:         logger.Named("search.service"),
		browseCache: newBrowseCache(),
	}
}

func (s *Service) Search(ctx context.Context, query string, page int) (*SearchPage, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}
	if page < 1 {
		page = 1
	}

	resp, err := s.tmdb.searchMulti(ctx, query, page)
	if err != nil {
		return nil, err
	}

	results := tmdbResultToResults(resp.Results)
	s.log.Info("tmdb search completed",
		zap.String("query", query),
		zap.Int("page", page),
		zap.Int("results_count", len(results)),
		zap.Int("total_pages", resp.TotalPages),
	)
	return &SearchPage{
		Results:    results,
		Page:       resp.Page,
		TotalPages: resp.TotalPages,
	}, nil
}

// ResolveExternalIDs fetches IMDb from TMDB (TVDB for shows when available) — only for download/indexer actions.
func (s *Service) ResolveExternalIDs(ctx context.Context, mediaType string, tmdbID int) (*ExternalIDs, error) {
	if tmdbID <= 0 {
		return nil, fmt.Errorf("invalid tmdb id")
	}
	switch mediaType {
	case "movie":
		imdb, err := s.tmdb.movieExternalIDs(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		if imdb == "" {
			return nil, fmt.Errorf("no imdb id for tmdb movie %d", tmdbID)
		}
		return &ExternalIDs{IMDBID: imdb, TMDBID: tmdbID}, nil
	case "show", "tv":
		imdb, tvdb, err := s.tmdb.tvExternalIDs(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		if imdb == "" {
			return nil, fmt.Errorf("no imdb id for tmdb show %d", tmdbID)
		}
		out := &ExternalIDs{IMDBID: imdb, TMDBID: tmdbID}
		if tvdb > 0 {
			out.TVDBID = tvdb
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}
}

// SearchMovies returns movie hits from page 1 of multi search (legacy route).
func (s *Service) SearchMovies(ctx context.Context, query string) ([]Movie, error) {
	page, err := s.Search(ctx, query, 1)
	if err != nil {
		return nil, err
	}
	movies := make([]Movie, 0)
	for _, r := range page.Results {
		if r.Type == "movie" && r.Movie != nil {
			movies = append(movies, *r.Movie)
		}
	}
	return movies, nil
}

// SearchShows returns show hits from page 1 of multi search (legacy route).
func (s *Service) SearchShows(ctx context.Context, query string) ([]Show, error) {
	page, err := s.Search(ctx, query, 1)
	if err != nil {
		return nil, err
	}
	shows := make([]Show, 0)
	for _, r := range page.Results {
		if r.Type == "show" && r.Show != nil {
			shows = append(shows, *r.Show)
		}
	}
	return shows, nil
}

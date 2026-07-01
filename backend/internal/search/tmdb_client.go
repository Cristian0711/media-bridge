package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"log/slog"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	tmdbImageBase = "https://image.tmdb.org/t/p/w500"
	// w300 keeps logos sharp on ~72px cards at 2–3× device pixel ratio (w92 was too small).
	tmdbLogoBase = "https://image.tmdb.org/t/p/w300"
)

type tmdbClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	log        *slog.Logger
}

func newTMDBClient(cfg TMDBConfig) *tmdbClient {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.themoviedb.org/3"
	}
	// The browse warmer fans out many concurrent requests to a single host
	// (api.themoviedb.org). http.DefaultTransport caps idle connections per host
	// at 2, forcing repeated TLS handshakes under that fan-out; raise it to match
	// the warmer's concurrency so connections are reused.
	defaultTransport, _ := http.DefaultTransport.(*http.Transport)
	transport := defaultTransport.Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 16
	return &tmdbClient{
		baseURL: base,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
			// otelhttp makes each outbound call a child span (status + latency)
			// and propagates the trace context downstream. Name spans by the
			// TMDB operation (path) so traces read "tmdb GET /search/multi"
			// rather than a generic "HTTP GET".
			Transport: otelhttp.NewTransport(transport,
				otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
					return "tmdb " + r.Method + " " + r.URL.Path
				}),
			),
		},
		log: logger.Component("search.tmdb"),
	}
}

type tmdbMultiSearchResponse struct {
	Page         int               `json:"page"`
	Results      []tmdbMultiResult `json:"results"`
	TotalPages   int               `json:"total_pages"`
	TotalResults int               `json:"total_results"`
}

type tmdbMultiResult struct {
	ID            int     `json:"id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	Popularity    float64 `json:"popularity"`
	PosterPath    string  `json:"poster_path"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	Adult         bool    `json:"adult"`
}

type tmdbMovieExternalIDs struct {
	IMDBID string `json:"imdb_id"`
}

type tmdbTVExternalIDs struct {
	IMDBID string `json:"imdb_id"`
	TVDBID int    `json:"tvdb_id"`
}

func (c *tmdbClient) searchMulti(ctx context.Context, query string, page int) (*tmdbMultiSearchResponse, error) {
	if page < 1 {
		page = 1
	}
	path := fmt.Sprintf(
		"/search/multi?include_adult=false&language=en-US&page=%d&query=%s",
		page,
		url.QueryEscape(query),
	)
	var out tmdbMultiSearchResponse
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *tmdbClient) movieExternalIDs(ctx context.Context, tmdbID int) (imdb string, err error) {
	var out tmdbMovieExternalIDs
	path := fmt.Sprintf("/movie/%d/external_ids", tmdbID)
	if err := c.get(ctx, path, &out); err != nil {
		return "", err
	}
	return out.IMDBID, nil
}

func (c *tmdbClient) tvExternalIDs(ctx context.Context, tmdbID int) (imdb string, tvdb int, err error) {
	var out tmdbTVExternalIDs
	path := fmt.Sprintf("/tv/%d/external_ids", tmdbID)
	if err := c.get(ctx, path, &out); err != nil {
		return "", 0, err
	}
	return out.IMDBID, out.TVDBID, nil
}

func (c *tmdbClient) get(ctx context.Context, path string, dest any) error {
	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Detail only: the error is returned to the caller, which logs the
		// outcome at the level it warrants (a transient external failure is not
		// itself an ERROR to review).
		c.log.DebugContext(ctx, "tmdb request failed", "url", u, logger.Err(err))
		return fmt.Errorf("tmdb request %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.DebugContext(ctx, "tmdb API error",
			"url", u,
			"status", resp.StatusCode,
			"body", string(body),
		)
		return fmt.Errorf("tmdb API error: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode tmdb response: %w", err)
	}
	return nil
}

func tmdbResultToResults(raw []tmdbMultiResult) []Result {
	out := make([]Result, 0, len(raw))
	for _, r := range raw {
		if r.PosterPath == "" {
			continue
		}
		switch r.MediaType {
		case "movie":
			title := r.Title
			if title == "" {
				title = r.OriginalTitle
			}
			m := Movie{
				Title: title,
				Year:  yearFromDate(r.ReleaseDate),
				IDs: MovieIDs{
					TMDB: r.ID,
				},
				Images: MovieImages{Poster: tmdbPosterURLs(r.PosterPath)},
			}
			out = append(out, Result{
				Type:  "movie",
				Score: r.Popularity,
				Movie: &m,
			})
		case "tv":
			title := r.Name
			if title == "" {
				title = r.OriginalName
			}
			s := Show{
				Title: title,
				Year:  yearFromDate(r.FirstAirDate),
				IDs: ShowIDs{
					TMDB: r.ID,
				},
				Images: ShowImages{Poster: tmdbPosterURLs(r.PosterPath)},
			}
			out = append(out, Result{
				Type:  "show",
				Score: r.Popularity,
				Show:  &s,
			})
		}
	}
	return out
}

func yearFromDate(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func tmdbPosterURLs(path string) []string {
	if path == "" {
		return nil
	}
	return []string{tmdbImageBase + path}
}

type tmdbPagedResults struct {
	Page         int               `json:"page"`
	Results      []tmdbMultiResult `json:"results"`
	TotalPages   int               `json:"total_pages"`
	TotalResults int               `json:"total_results"`
}

type tmdbMovieRow struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	PosterPath    string  `json:"poster_path"`
	ReleaseDate   string  `json:"release_date"`
	Popularity    float64 `json:"popularity"`
}

type tmdbTVRow struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	PosterPath   string  `json:"poster_path"`
	FirstAirDate string  `json:"first_air_date"`
	Popularity   float64 `json:"popularity"`
}

type tmdbMovieListResponse struct {
	Page       int            `json:"page"`
	Results    []tmdbMovieRow `json:"results"`
	TotalPages int            `json:"total_pages"`
}

type tmdbTVListResponse struct {
	Page       int         `json:"page"`
	Results    []tmdbTVRow `json:"results"`
	TotalPages int         `json:"total_pages"`
}

func (c *tmdbClient) trendingAllWeek(ctx context.Context, page int) (*tmdbPagedResults, error) {
	if page < 1 {
		page = 1
	}
	var out tmdbPagedResults
	path := fmt.Sprintf("/trending/all/week?language=en-US&page=%d", page)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *tmdbClient) trendingMoviesWeek(ctx context.Context, page int) (*tmdbMovieListResponse, error) {
	if page < 1 {
		page = 1
	}
	var out tmdbMovieListResponse
	path := fmt.Sprintf("/trending/movie/week?language=en-US&page=%d", page)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *tmdbClient) trendingTVWeek(ctx context.Context, page int) (*tmdbTVListResponse, error) {
	if page < 1 {
		page = 1
	}
	var out tmdbTVListResponse
	path := fmt.Sprintf("/trending/tv/week?language=en-US&page=%d", page)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *tmdbClient) discoverMoviesByProvider(ctx context.Context, providerID, genreID, page int) (*tmdbMovieListResponse, error) {
	if page < 1 {
		page = 1
	}
	var out tmdbMovieListResponse
	path := fmt.Sprintf(
		"/discover/movie?include_adult=false&language=en-US&page=%d&sort_by=popularity.desc&watch_region=US&with_watch_providers=%d&with_watch_monetization_types=flatrate%s",
		page, providerID, tmdbGenreQuery("with_genres", genreID),
	)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *tmdbClient) discoverTVByProvider(ctx context.Context, providerID, genreID, page int) (*tmdbTVListResponse, error) {
	if page < 1 {
		page = 1
	}
	var out tmdbTVListResponse
	path := fmt.Sprintf(
		"/discover/tv?include_adult=false&language=en-US&page=%d&sort_by=popularity.desc&watch_region=US&with_watch_providers=%d&with_watch_monetization_types=flatrate%s",
		page, providerID, tmdbGenreQuery("with_genres", genreID),
	)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func tmdbGenreQuery(param string, genreID int) string {
	if genreID <= 0 {
		return ""
	}
	return fmt.Sprintf("&%s=%d", param, genreID)
}

type tmdbWatchProvidersResponse struct {
	Results []tmdbWatchProvider `json:"results"`
}

type tmdbWatchProvider struct {
	ProviderID   int    `json:"provider_id"`
	LogoPath     string `json:"logo_path"`
	ProviderName string `json:"provider_name"`
}

func (c *tmdbClient) watchProviderLogos(ctx context.Context, region string) (map[int]string, error) {
	logos := make(map[int]string)
	for _, media := range []string{"movie", "tv"} {
		var out tmdbWatchProvidersResponse
		path := fmt.Sprintf("/watch/providers/%s?watch_region=%s", media, url.QueryEscape(region))
		if err := c.get(ctx, path, &out); err != nil {
			return nil, err
		}
		for _, p := range out.Results {
			if p.LogoPath != "" {
				logos[p.ProviderID] = tmdbLogoBase + p.LogoPath
			}
		}
	}
	return logos, nil
}

func tmdbMoviesToResults(raw []tmdbMovieRow) []Result {
	out := make([]Result, 0, len(raw))
	for _, r := range raw {
		if r.PosterPath == "" {
			continue
		}
		title := r.Title
		if title == "" {
			title = r.OriginalTitle
		}
		m := Movie{
			Title:  title,
			Year:   yearFromDate(r.ReleaseDate),
			IDs:    MovieIDs{TMDB: r.ID},
			Images: MovieImages{Poster: tmdbPosterURLs(r.PosterPath)},
		}
		out = append(out, Result{Type: "movie", Score: r.Popularity, Movie: &m})
	}
	return out
}

func tmdbTVShowsToResults(raw []tmdbTVRow) []Result {
	out := make([]Result, 0, len(raw))
	for _, r := range raw {
		if r.PosterPath == "" {
			continue
		}
		title := r.Name
		if title == "" {
			title = r.OriginalName
		}
		s := Show{
			Title:  title,
			Year:   yearFromDate(r.FirstAirDate),
			IDs:    ShowIDs{TMDB: r.ID},
			Images: ShowImages{Poster: tmdbPosterURLs(r.PosterPath)},
		}
		out = append(out, Result{Type: "show", Score: r.Popularity, Show: &s})
	}
	return out
}

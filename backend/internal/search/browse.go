package search

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

const browseWatchRegion = "US"

type browseServiceDef struct {
	ID         string
	Name       string
	ProviderID int
}

var browseServices = []browseServiceDef{
	{ID: "netflix", Name: "Netflix", ProviderID: 8},
	{ID: "prime", Name: "Prime Video", ProviderID: 9},
	{ID: "max", Name: "Max", ProviderID: 1899},
	{ID: "disney", Name: "Disney+", ProviderID: 337},
	{ID: "peacock", Name: "Peacock", ProviderID: 386},
	{ID: "hulu", Name: "Hulu", ProviderID: 15},
}

type browseListKind struct {
	Suffix     string
	Title      string
	MovieGenre int
	TVGenre    int
	MoviesOnly bool
	TVOnly     bool
}

var serviceListKinds = []browseListKind{
	// Movies first — genre rows are movies-only (provider browse skews toward series).
	{Suffix: "movies", Title: "Popular Movies", MoviesOnly: true},
	{Suffix: "drama-movies", Title: "Drama", MoviesOnly: true, MovieGenre: 18},
	{Suffix: "action-movies", Title: "Action", MoviesOnly: true, MovieGenre: 28},
	{Suffix: "comedy-movies", Title: "Comedy", MoviesOnly: true, MovieGenre: 35},
	{Suffix: "sci-fi-movies", Title: "Sci-Fi", MoviesOnly: true, MovieGenre: 878},
	// Series rows after movies.
	{Suffix: "series", Title: "Popular Series", TVOnly: true},
	{Suffix: "drama-series", Title: "Drama Series", TVOnly: true, TVGenre: 18},
	{Suffix: "action-series", Title: "Action Series", TVOnly: true, TVGenre: 10759},
	{Suffix: "comedy-series", Title: "Comedy Series", TVOnly: true, TVGenre: 35},
	{Suffix: "sci-fi-series", Title: "Sci-Fi Series", TVOnly: true, TVGenre: 10765},
}

// BrowseService is a streaming provider shown on the home page.
type BrowseService struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
}

// BrowseListMeta describes one horizontal row of titles.
type BrowseListMeta struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type providerLogoCache struct {
	mu    sync.RWMutex
	logos map[int]string
}

func (s *Service) providerLogos(ctx context.Context) (map[int]string, error) {
	s.logoCache.mu.RLock()
	if s.logoCache.logos != nil {
		defer s.logoCache.mu.RUnlock()
		return s.logoCache.logos, nil
	}
	s.logoCache.mu.RUnlock()

	s.logoCache.mu.Lock()
	defer s.logoCache.mu.Unlock()
	if s.logoCache.logos != nil {
		return s.logoCache.logos, nil
	}
	logos, err := s.tmdb.watchProviderLogos(ctx, browseWatchRegion)
	if err != nil {
		return nil, err
	}
	s.logoCache.logos = logos
	return logos, nil
}

// BrowseServices returns streaming services with TMDB logos (cached 24h).
func (s *Service) BrowseServices(ctx context.Context) ([]BrowseService, error) {
	if cached, ok := s.browseCache.getServices(); ok {
		return cached, nil
	}
	out, err := s.fetchBrowseServices(ctx)
	if err != nil {
		return nil, err
	}
	s.browseCache.setServices(out)
	return out, nil
}

func (s *Service) fetchBrowseServices(ctx context.Context) ([]BrowseService, error) {
	logos, err := s.providerLogos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BrowseService, 0, len(browseServices))
	for _, svc := range browseServices {
		logo := logos[svc.ProviderID]
		out = append(out, BrowseService{
			ID:      svc.ID,
			Name:    svc.Name,
			LogoURL: logo,
		})
	}
	return out, nil
}

func (s *Service) warmBrowseServices(ctx context.Context) ([]BrowseService, error) {
	out, err := s.fetchBrowseServices(ctx)
	if err != nil {
		return nil, err
	}
	s.browseCache.setServices(out)
	return out, nil
}

// BrowseServiceLists returns category rows for one service (e.g. netflix → netflix:movies).
func (s *Service) BrowseServiceLists(serviceID string) ([]BrowseListMeta, error) {
	if !isKnownService(serviceID) {
		return nil, fmt.Errorf("unknown service: %s", serviceID)
	}
	out := make([]BrowseListMeta, 0, len(serviceListKinds))
	for _, kind := range serviceListKinds {
		out = append(out, BrowseListMeta{
			ID:    serviceID + ":" + kind.Suffix,
			Title: kind.Title,
		})
	}
	return out, nil
}

// BrowseGlobalLists returns non-service rows (movies first, then series).
func (s *Service) BrowseGlobalLists() []BrowseListMeta {
	return []BrowseListMeta{
		{ID: "trending-movies", Title: "Trending Movies"},
		{ID: "trending-series", Title: "Trending Series"},
	}
}

// BrowseCatalog is kept for backwards compatibility; returns global lists only.
func (s *Service) BrowseCatalog() []BrowseListMeta {
	return s.BrowseGlobalLists()
}

// Browse returns one page of results for a list id (global or service:kind). Page 1 is cached 24h.
func (s *Service) Browse(ctx context.Context, listID string, page int) (*SearchPage, error) {
	if page < 1 {
		page = 1
	}
	if page == 1 {
		if catalogKey, ok := catalogCacheKeyForList(listID); ok {
			if catalog, ok := s.browseCache.getCatalog(catalogKey); ok {
				for _, row := range catalog.Lists {
					if row.ID == listID {
						return &SearchPage{
							Results:    append([]Result{}, row.Results...),
							Page:       row.Page,
							TotalPages: row.TotalPages,
						}, nil
					}
				}
			}
		}
	}
	if cached, ok := s.browseCache.getListPage(listID, page); ok {
		return cached, nil
	}
	result, err := s.fetchBrowse(ctx, listID, page)
	if err != nil {
		return nil, err
	}
	s.browseCache.setListPage(listID, page, result)
	return result, nil
}

func catalogCacheKeyForList(listID string) (string, bool) {
	if listID == "trending-movies" || listID == "trending-series" {
		return globalCatalogCacheKey, true
	}
	if strings.Contains(listID, ":") {
		serviceID, _, err := parseServiceListID(listID)
		if err != nil {
			return "", false
		}
		return serviceCatalogCacheKey(serviceID), true
	}
	return "", false
}

func (s *Service) warmBrowseListPage(ctx context.Context, listID string, page int) error {
	result, err := s.fetchBrowse(ctx, listID, page)
	if err != nil {
		return err
	}
	s.browseCache.setListPage(listID, page, result)
	return nil
}

func (s *Service) fetchBrowse(ctx context.Context, listID string, page int) (*SearchPage, error) {
	if strings.Contains(listID, ":") {
		serviceID, kind, err := parseServiceListID(listID)
		if err != nil {
			return nil, err
		}
		return s.browseServiceKind(ctx, serviceID, kind, page)
	}

	switch listID {
	case "trending":
		return s.browseTrending(ctx, page)
	case "trending-movies":
		return s.browseTrendingMovies(ctx, page)
	case "trending-series":
		return s.browseTrendingSeries(ctx, page)
	default:
		return nil, fmt.Errorf("unknown browse list: %s", listID)
	}
}

func parseServiceListID(listID string) (serviceID, kind string, err error) {
	parts := strings.SplitN(listID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid browse list: %s", listID)
	}
	if !isKnownService(parts[0]) {
		return "", "", fmt.Errorf("unknown service: %s", parts[0])
	}
	return parts[0], parts[1], nil
}

func isKnownService(id string) bool {
	for _, svc := range browseServices {
		if svc.ID == id {
			return true
		}
	}
	return false
}

func providerIDForService(serviceID string) (int, error) {
	for _, svc := range browseServices {
		if svc.ID == serviceID {
			return svc.ProviderID, nil
		}
	}
	return 0, fmt.Errorf("unknown service: %s", serviceID)
}

func (s *Service) browseServiceKind(ctx context.Context, serviceID, kindSuffix string, page int) (*SearchPage, error) {
	var kind *browseListKind
	for i := range serviceListKinds {
		if serviceListKinds[i].Suffix == kindSuffix {
			kind = &serviceListKinds[i]
			break
		}
	}
	if kind == nil {
		return nil, fmt.Errorf("unknown list kind: %s", kindSuffix)
	}

	providerID, err := providerIDForService(serviceID)
	if err != nil {
		return nil, err
	}

	if kind.MoviesOnly {
		return s.browseProviderMovies(ctx, providerID, kind.MovieGenre, page)
	}
	if kind.TVOnly {
		return s.browseProviderTV(ctx, providerID, kind.TVGenre, page)
	}

	return nil, fmt.Errorf("invalid list kind: %s", kindSuffix)
}

func (s *Service) browseProviderMovies(ctx context.Context, providerID, genreID, page int) (*SearchPage, error) {
	movies, err := s.tmdb.discoverMoviesByProvider(ctx, providerID, genreID, page)
	if err != nil {
		return nil, err
	}
	return &SearchPage{
		Results:    tmdbMoviesToResults(movies.Results),
		Page:       movies.Page,
		TotalPages: movies.TotalPages,
	}, nil
}

func (s *Service) browseProviderTV(ctx context.Context, providerID, genreID, page int) (*SearchPage, error) {
	shows, err := s.tmdb.discoverTVByProvider(ctx, providerID, genreID, page)
	if err != nil {
		return nil, err
	}
	return &SearchPage{
		Results:    tmdbTVShowsToResults(shows.Results),
		Page:       shows.Page,
		TotalPages: shows.TotalPages,
	}, nil
}

func (s *Service) browseTrending(ctx context.Context, page int) (*SearchPage, error) {
	resp, err := s.tmdb.trendingAllWeek(ctx, page)
	if err != nil {
		return nil, err
	}
	return &SearchPage{
		Results:    tmdbResultToResults(resp.Results),
		Page:       resp.Page,
		TotalPages: resp.TotalPages,
	}, nil
}

func (s *Service) browseTrendingMovies(ctx context.Context, page int) (*SearchPage, error) {
	resp, err := s.tmdb.trendingMoviesWeek(ctx, page)
	if err != nil {
		return nil, err
	}
	return &SearchPage{
		Results:    tmdbMoviesToResults(resp.Results),
		Page:       resp.Page,
		TotalPages: resp.TotalPages,
	}, nil
}

func (s *Service) browseTrendingSeries(ctx context.Context, page int) (*SearchPage, error) {
	resp, err := s.tmdb.trendingTVWeek(ctx, page)
	if err != nil {
		return nil, err
	}
	return &SearchPage{
		Results:    tmdbTVShowsToResults(resp.Results),
		Page:       resp.Page,
		TotalPages: resp.TotalPages,
	}, nil
}

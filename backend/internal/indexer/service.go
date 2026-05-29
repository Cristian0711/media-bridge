package indexer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

type Service struct {
	indexers map[string]Provider
	mu       sync.RWMutex
	log      *zap.Logger
}

type MovieSearchResponse struct {
	Movies             []Movie        `json:"movies"`
	Total              int            `json:"total"`
	ByIndexer          map[string]int `json:"by_indexer"`
	AvailableQualities []string       `json:"available_qualities"`
}

type ShowSearchResponse struct {
	Shows              []Show         `json:"shows"`
	Unparsed           []Show         `json:"unparsed,omitempty"`
	Total              int            `json:"total"`
	ByIndexer          map[string]int `json:"by_indexer"`
	AvailableQualities []string       `json:"available_qualities"`
	AvailableSeasons   []int          `json:"available_seasons"`
}

func NewService() *Service {
	return &Service{
		indexers: make(map[string]Provider),
		log:      logger.Named("indexer.service"),
	}
}

func (s *Service) RegisterIndexer(idx Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indexers[idx.GetID()] = idx
}

func (s *Service) ListIndexers() []Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Provider, 0, len(s.indexers))
	for _, idx := range s.indexers {
		list = append(list, idx)
	}
	return list
}

func (s *Service) SearchMovies(ctx context.Context, req SearchRequest) (MovieSearchResponse, error) {
	s.log.Info("movie search started",
		zap.String("imdb_id", req.ImdbID),
		zap.String("quality", req.Quality),
		zap.Strings("indexers", req.Indexers))
	items := s.searchAcrossIndexers(ctx, req, true)
	movies := processMovieItems(ctx, items)
	movies = filterAndSortMovies(movies, req.Quality)
	byIndexer := map[string]int{}
	qset := map[string]bool{}
	for _, m := range movies {
		byIndexer[m.IndexerName]++
		qset[m.Quality] = true
	}
	qualities := make([]string, 0, len(qset))
	for q := range qset {
		qualities = append(qualities, q)
	}
	sort.Strings(qualities)
	s.log.Info("movie search completed",
		zap.Int("raw_items", len(items)),
		zap.Int("results", len(movies)),
		zap.Any("by_indexer", byIndexer))
	return MovieSearchResponse{Movies: movies, Total: len(movies), ByIndexer: byIndexer, AvailableQualities: qualities}, nil
}

func (s *Service) SearchShows(ctx context.Context, req SearchRequest) (ShowSearchResponse, error) {
	s.log.Info("show search started",
		zap.String("imdb_id", req.ImdbID),
		zap.Int("season", req.Season),
		zap.Int("episode", req.Episode),
		zap.String("quality", req.Quality),
		zap.Strings("indexers", req.Indexers))
	items := s.searchAcrossIndexers(ctx, req, false)
	shows := processShowItems(ctx, items)
	parsed, unparsed := filterAndSortShows(shows, req.Season, req.Episode, req.Quality)
	all := append(append([]Show{}, parsed...), unparsed...)
	byIndexer := map[string]int{}
	qset := map[string]bool{}
	sset := map[int]bool{}
	for _, sh := range all {
		byIndexer[sh.IndexerName]++
		qset[sh.Quality] = true
		if sh.Season > 0 {
			sset[sh.Season] = true
		}
	}
	qualities := make([]string, 0, len(qset))
	for q := range qset {
		qualities = append(qualities, q)
	}
	seasons := make([]int, 0, len(sset))
	for n := range sset {
		seasons = append(seasons, n)
	}
	sort.Strings(qualities)
	sort.Ints(seasons)
	s.log.Info("show search completed",
		zap.Int("raw_items", len(items)),
		zap.Int("parsed_results", len(parsed)),
		zap.Int("unparsed_results", len(unparsed)),
		zap.Any("by_indexer", byIndexer))
	return ShowSearchResponse{Shows: parsed, Unparsed: unparsed, Total: len(all), ByIndexer: byIndexer, AvailableQualities: qualities, AvailableSeasons: seasons}, nil
}

func (s *Service) FindBestMovie(ctx context.Context, req SearchRequest) (Movie, error) {
	if strings.TrimSpace(req.Quality) == "" {
		return Movie{}, fmt.Errorf("quality is required")
	}
	searchReq := req
	searchReq.Quality = ""
	resp, err := s.SearchMovies(ctx, searchReq)
	if err != nil {
		return Movie{}, err
	}
	if len(resp.Movies) == 0 {
		return Movie{}, fmt.Errorf("no movies found")
	}
	return pickBestFreeleechMovie(resp.Movies, req.Quality)
}

func (s *Service) FindBestShow(ctx context.Context, req SearchRequest) (Show, error) {
	if strings.TrimSpace(req.Quality) == "" {
		return Show{}, fmt.Errorf("quality is required")
	}
	searchReq := req
	searchReq.Quality = ""
	resp, err := s.SearchShows(ctx, searchReq)
	if err != nil {
		return Show{}, err
	}
	all := append(append([]Show{}, resp.Shows...), resp.Unparsed...)
	if len(all) == 0 {
		return Show{}, fmt.Errorf("no shows found")
	}
	return pickBestFreeleechShow(all, req.Quality)
}

func (s *Service) DownloadTorrent(ctx context.Context, indexerID, downloadURL string) (string, error) {
	s.mu.RLock()
	idx, ok := s.indexers[indexerID]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("indexer not found: %s", indexerID)
	}
	return idx.DownloadTorrent(ctx, downloadURL)
}

func (s *Service) searchAcrossIndexers(ctx context.Context, req SearchRequest, movie bool) []IndexerItem {
	indexersToSearch := s.getIndexersToSearch(req.Indexers)
	searchType := "shows"
	if movie {
		searchType = "movies"
	}
	s.log.Info("dispatching indexer search",
		zap.String("type", searchType),
		zap.Int("requested_indexers", len(req.Indexers)),
		zap.Int("selected_indexers", len(indexersToSearch)))
	var wg sync.WaitGroup
	results := make(chan []IndexerItem, len(indexersToSearch))
	for _, p := range indexersToSearch {
		if !p.IsEnabled() {
			continue
		}
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()
			s.log.Debug("indexer search start", zap.String("indexer", provider.GetID()), zap.String("type", searchType))
			var out []IndexerItem
			var err error
			if movie {
				out, err = provider.SearchMovies(ctx, req)
			} else {
				out, err = provider.SearchShows(ctx, req)
			}
			if err != nil {
				s.log.Warn("indexer search failed", zap.String("indexer", provider.GetID()), zap.Error(err))
				return
			}
			s.log.Info("indexer search completed",
				zap.String("indexer", provider.GetID()),
				zap.String("type", searchType),
				zap.Int("results", len(out)))
			results <- out
		}(p)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	all := make([]IndexerItem, 0, 200)
	for r := range results {
		all = append(all, r...)
	}
	if len(all) == 0 {
		s.log.Warn("no results from any indexer", zap.String("type", searchType), zap.String("imdb_id", req.ImdbID))
	}
	return all
}

func (s *Service) getIndexersToSearch(ids []string) []Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(ids) == 0 {
		out := make([]Provider, 0, len(s.indexers))
		for _, p := range s.indexers {
			out = append(out, p)
		}
		return out
	}
	out := make([]Provider, 0, len(ids))
	for _, id := range ids {
		if p, ok := s.indexers[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

func parseID(id string) int64 {
	var n int64
	for i, c := range id {
		if i >= 10 {
			break
		}
		if c < '0' || c > '9' {
			continue
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

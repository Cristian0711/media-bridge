package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// tracer for manual spans that narrate the indexer search operation.
var tracer = otel.Tracer("media-bridge/indexer")

// CatalogLister optionally lists indexers configured in Prowlarr (child indexers).
type CatalogLister interface {
	ListCatalog(ctx context.Context) ([]CatalogEntry, error)
}

// CatalogEntry describes one indexer visible through Prowlarr.
type CatalogEntry struct {
	ID      string
	Name    string
	Enabled bool
}

type Service struct {
	indexers map[string]Provider
	catalog  CatalogLister
	settings SettingsRepository
	mu       sync.RWMutex
	log      *slog.Logger
}

// IndexerSettingView is one row in the admin indexer-configuration panel: a
// catalog indexer plus its resolved freeleech preference.
type IndexerSettingView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	FreeleechOnly bool   `json:"freeleech_only"`
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
		log:      logger.Component("indexer.service"),
	}
}

func (s *Service) RegisterIndexer(idx Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indexers[idx.GetID()] = idx
}

// SetCatalog configures an optional catalog source (e.g. Prowlarr child indexers).
func (s *Service) SetCatalog(cat CatalogLister) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalog = cat
}

// SetSettingsRepository wires the persistent per-indexer settings store. When
// unset, searches fall back to the freeleech-only default for every indexer.
func (s *Service) SetSettingsRepository(repo SettingsRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = repo
}

// buildFreeleechPolicy loads the configured per-indexer freeleech preferences.
// Any load failure degrades to the conservative freeleech-only default.
func (s *Service) buildFreeleechPolicy(ctx context.Context) freeleechPolicy {
	s.mu.RLock()
	repo := s.settings
	s.mu.RUnlock()
	if repo == nil {
		return freeleechPolicy{}
	}
	list, err := repo.List(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "indexer settings load failed, using freeleech-only default", logger.Err(err))
		return freeleechPolicy{}
	}
	byName := make(map[string]bool, len(list))
	for _, it := range list {
		byName[strings.ToLower(strings.TrimSpace(it.IndexerName))] = it.FreeleechOnly
	}
	return freeleechPolicy{byName: byName}
}

// ListIndexerSettings returns the indexer catalog merged with each indexer's
// resolved freeleech preference, for the admin configuration panel.
func (s *Service) ListIndexerSettings(ctx context.Context) []IndexerSettingView {
	policy := s.buildFreeleechPolicy(ctx)
	catalog := s.ListIndexerCatalog(ctx)
	out := make([]IndexerSettingView, 0, len(catalog))
	for _, e := range catalog {
		out = append(out, IndexerSettingView{
			ID:            e.ID,
			Name:          e.Name,
			Enabled:       e.Enabled,
			FreeleechOnly: policy.freeleechOnly(e.Name),
		})
	}
	return out
}

// UpdateIndexerSetting persists the freeleech preference for a single indexer.
func (s *Service) UpdateIndexerSetting(ctx context.Context, indexerName string, freeleechOnly bool) error {
	name := strings.TrimSpace(indexerName)
	if name == "" {
		return fmt.Errorf("indexer_name is required")
	}
	s.mu.RLock()
	repo := s.settings
	s.mu.RUnlock()
	if repo == nil {
		return fmt.Errorf("indexer settings are not available")
	}
	_, err := repo.Upsert(ctx, name, freeleechOnly)
	return err
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

// ListIndexerCatalog returns Prowlarr child indexers when configured, otherwise
// the registered provider list.
func (s *Service) ListIndexerCatalog(ctx context.Context) []CatalogEntry {
	s.mu.RLock()
	cat := s.catalog
	indexers := s.indexers
	s.mu.RUnlock()

	if cat != nil {
		entries, err := cat.ListCatalog(ctx)
		if err == nil && len(entries) > 0 {
			return entries
		}
		if err != nil {
			s.log.WarnContext(ctx, "indexer catalog list failed", logger.Err(err))
		}
	}

	out := make([]CatalogEntry, 0, len(indexers))
	for _, p := range indexers {
		out = append(out, CatalogEntry{
			ID:      p.GetID(),
			Name:    p.GetName(),
			Enabled: p.IsEnabled(),
		})
	}
	return out
}

func (s *Service) SearchMovies(ctx context.Context, req SearchRequest) (MovieSearchResponse, error) {
	s.log.InfoContext(ctx, "movie search started",
		"imdb_id", req.ImdbID,
		"quality", req.Quality,
		"indexers", req.Indexers)
	items := s.searchAcrossIndexers(ctx, req, true)
	pctx, pspan := tracer.Start(ctx, "indexer.process_results",
		trace.WithAttributes(attribute.Int("raw_items", len(items))))
	movies := processMovieItems(pctx, items)
	// Cross-seed counts are computed over every raw result, before freeleech
	// filtering, so the count reflects all indexers regardless of what survives.
	annotateMovieCrossSeed(movies)
	movies = filterAndSortMovies(movies, req.Quality, s.buildFreeleechPolicy(ctx))
	pspan.SetAttributes(attribute.Int("results", len(movies)))
	pspan.End()
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
	s.log.InfoContext(ctx, "movie search completed",
		"raw_items", len(items),
		"results", len(movies),
		"by_indexer", byIndexer)
	return MovieSearchResponse{Movies: movies, Total: len(movies), ByIndexer: byIndexer, AvailableQualities: qualities}, nil
}

func (s *Service) SearchShows(ctx context.Context, req SearchRequest) (ShowSearchResponse, error) {
	s.log.InfoContext(ctx, "show search started",
		"imdb_id", req.ImdbID,
		"season", req.Season,
		"episode", req.Episode,
		"quality", req.Quality,
		"indexers", req.Indexers)
	items := s.searchAcrossIndexers(ctx, req, false)
	pctx, pspan := tracer.Start(ctx, "indexer.process_results",
		trace.WithAttributes(attribute.Int("raw_items", len(items))))
	shows := processShowItems(pctx, items)
	// Cross-seed counts are computed over every raw result, before freeleech
	// filtering, so the count reflects all indexers regardless of what survives.
	annotateShowCrossSeed(shows)
	parsed, unparsed := filterAndSortShows(shows, req.Season, req.Episode, req.Quality, s.buildFreeleechPolicy(ctx))
	pspan.SetAttributes(attribute.Int("results", len(parsed)+len(unparsed)))
	pspan.End()
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
	s.log.InfoContext(ctx, "show search completed",
		"raw_items", len(items),
		"parsed_results", len(parsed),
		"unparsed_results", len(unparsed),
		"by_indexer", byIndexer)
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
	s.log.InfoContext(ctx, "dispatching indexer search",
		"type", searchType,
		"requested_indexers", len(req.Indexers),
		"selected_indexers", len(indexersToSearch))
	var wg sync.WaitGroup
	results := make(chan []IndexerItem, len(indexersToSearch))
	for _, p := range indexersToSearch {
		if !p.IsEnabled() {
			continue
		}
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()
			// One span per provider so the outbound indexer HTTP call nests under
			// a named "indexer.provider_search" span (and parallel indexers show
			// as sibling spans). pctx is what carries the trace into the HTTP call.
			pctx, span := tracer.Start(ctx, "indexer.provider_search",
				trace.WithAttributes(
					attribute.String("indexer.id", provider.GetID()),
					attribute.String("indexer.type", searchType),
				))
			defer span.End()
			s.log.DebugContext(pctx, "indexer search start", "indexer", provider.GetID(), "type", searchType)
			var out []IndexerItem
			var err error
			if movie {
				out, err = provider.SearchMovies(pctx, req)
			} else {
				out, err = provider.SearchShows(pctx, req)
			}
			if err != nil {
				span.RecordError(err)
				s.log.WarnContext(pctx, "indexer search failed", "indexer", provider.GetID(), logger.Err(err))
				return
			}
			span.SetAttributes(attribute.Int("indexer.results", len(out)))
			s.log.InfoContext(pctx, "indexer search completed",
				"indexer", provider.GetID(),
				"type", searchType,
				"results", len(out))
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
		s.log.WarnContext(ctx, "no results from any indexer", "type", searchType, "imdb_id", req.ImdbID)
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
	s := strings.TrimSpace(id)
	if s == "" {
		return 0
	}
	// UNIT3D download URLs: .../torrent/download/{id}.{token}
	if i := strings.LastIndex(s, "/download/"); i >= 0 {
		tail := s[i+len("/download/"):]
		if dot := strings.IndexByte(tail, '.'); dot > 0 {
			tail = tail[:dot]
		}
		if n, err := strconv.ParseInt(tail, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return n
	}
	var n int64
	for i, c := range s {
		if i >= 32 {
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

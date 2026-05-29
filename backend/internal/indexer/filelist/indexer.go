package filelist

import (
	"context"
	"fmt"
	"strings"

	idx "github.com/Cristian0711/media-bridge/backend/internal/indexer"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

type Provider struct {
	idx.BaseIndexer
	client *Client
	log    *zap.Logger
}

func NewProvider(cfg Config, enabled bool) *Provider {
	return &Provider{
		BaseIndexer: idx.BaseIndexer{Name: "FileList", ID: "filelist", Enabled: enabled},
		client:      NewClient(cfg),
		log:         logger.Named("indexer.filelist"),
	}
}

func (p *Provider) SearchMovies(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	p.log.Info("filelist movie search start", zap.String("imdb_id", req.ImdbID))
	movies, rateLimited, err := p.searchMovieThroughAPI(ctx, req.ImdbID)
	if err != nil {
		p.log.Warn("filelist api movie search failed", zap.Error(err))
	}
	if rateLimited || len(movies) == 0 {
		p.log.Warn("filelist movie search falling back to scrape",
			zap.Bool("rate_limited", rateLimited),
			zap.Int("api_results", len(movies)))
		movies, err = p.searchMovieThroughScrape(ctx, req.ImdbID)
		if err != nil {
			p.log.Warn("filelist scrape movie search failed", zap.Error(err))
		}
	}
	items := make([]idx.IndexerItem, 0, len(movies))
	for _, m := range movies {
		if m.Freeleech == 1 {
			continue
		}
		items = append(items, idx.IndexerItem{
			ID:           fmt.Sprintf("%d", m.ID),
			Name:         m.Name,
			ImdbID:       m.Imdb,
			Size:         m.Size,
			Seeders:      m.Seeders,
			Leechers:     m.Leechers,
			Downloads:    m.Downloads,
			DownloadLink: m.DownloadLink,
			Freeleech:    m.Freeleech == 1,
			Category:     m.Category,
			IndexerName:  p.Name,
		})
	}
	p.log.Info("filelist movie search complete", zap.Int("results", len(items)))
	return items, nil
}

func (p *Provider) SearchShows(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	p.log.Info("filelist show search start",
		zap.String("imdb_id", req.ImdbID),
		zap.Int("season", req.Season),
		zap.Int("episode", req.Episode))
	shows, rateLimited, err := p.searchShowThroughAPI(ctx, req.ImdbID)
	if err != nil {
		p.log.Warn("filelist api show search failed", zap.Error(err))
	}
	if rateLimited || len(shows) == 0 {
		p.log.Warn("filelist show search falling back to scrape",
			zap.Bool("rate_limited", rateLimited),
			zap.Int("api_results", len(shows)))
		shows, err = p.searchShowThroughScrape(ctx, req.ImdbID)
		if err != nil {
			p.log.Warn("filelist scrape show search failed", zap.Error(err))
		}
	}
	items := make([]idx.IndexerItem, 0, len(shows))
	for _, s := range shows {
		if s.Freeleech == 1 {
			continue
		}
		items = append(items, idx.IndexerItem{
			ID:           fmt.Sprintf("%d", s.ID),
			Name:         s.Name,
			ImdbID:       s.Imdb,
			Size:         s.Size,
			Seeders:      s.Seeders,
			Leechers:     s.Leechers,
			Downloads:    s.Downloads,
			DownloadLink: s.DownloadLink,
			Freeleech:    s.Freeleech == 1,
			Category:     s.Category,
			IndexerName:  p.Name,
		})
	}
	p.log.Info("filelist show search complete", zap.Int("results", len(items)))
	return items, nil
}

func (p *Provider) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	return p.client.DownloadTorrent(ctx, downloadURL)
}

type searchStrategy struct {
	params map[string]string
}

func (p *Provider) searchMovieThroughAPI(ctx context.Context, imdbID string) ([]idx.Movie, bool, error) {
	strategy := searchStrategy{params: map[string]string{
		"action": "search-torrents",
		"type":   "imdb",
		"query":  imdbID,
	}}
	p.log.Debug("filelist movie api strategy", zap.Any("params", strategy.params))
	var temp []idx.Movie
	if err := p.client.Request(ctx, strategy.params, &temp); err != nil {
		p.log.Warn("filelist movie api strategy failed", zap.Any("params", strategy.params), zap.Error(err))
		if isRateLimitError(err) {
			return nil, true, err
		}
		return nil, false, err
	}
	if len(temp) > 0 {
		for i := range temp {
			temp[i].Quality = idx.ParseTorrentQuality(temp[i].Name)
		}
		return temp, false, nil
	}
	return nil, false, nil
}

func (p *Provider) searchMovieThroughScrape(ctx context.Context, imdbID string) ([]idx.Movie, error) {
	params := map[string]string{"search": imdbID, "searchin": "3"}
	p.log.Debug("filelist movie scrape strategy", zap.Any("params", params))
	movies := p.browseMoviesPaginated(ctx, params["search"], params["searchin"])
	if len(movies) > 0 {
		return movies, nil
	}
	return nil, nil
}

func (p *Provider) searchShowThroughAPI(ctx context.Context, imdbID string) ([]idx.Show, bool, error) {
	strategy := searchStrategy{params: map[string]string{
		"action": "search-torrents",
		"type":   "imdb",
		"query":  imdbID,
	}}
	p.log.Debug("filelist show api strategy", zap.Any("params", strategy.params))
	var temp []idx.Show
	if err := p.client.Request(ctx, strategy.params, &temp); err != nil {
		p.log.Warn("filelist show api strategy failed", zap.Any("params", strategy.params), zap.Error(err))
		if isRateLimitError(err) {
			return nil, true, err
		}
		return nil, false, err
	}
	if len(temp) > 0 {
		return temp, false, nil
	}
	return nil, false, nil
}

func (p *Provider) searchShowThroughScrape(ctx context.Context, imdbID string) ([]idx.Show, error) {
	params := map[string]string{"search": imdbID, "searchin": "3"}
	p.log.Debug("filelist show scrape strategy", zap.Any("params", params))
	shows := p.browseShowsPaginated(ctx, params["search"], params["searchin"])
	if len(shows) > 0 {
		return shows, nil
	}
	return nil, nil
}

func (p *Provider) browseMoviesPaginated(ctx context.Context, search, searchin string) []idx.Movie {
	const maxPages = 5
	all := make([]idx.Movie, 0, 100)
	seen := make(map[string]struct{}, 100)
	for page := 0; page <= maxPages; page++ {
		params := map[string]string{"cat": "0", "sort": "2", "search": search, "searchin": searchin, "page": fmt.Sprintf("%d", page)}
		html, err := p.client.Browse(ctx, params)
		if err != nil {
			p.log.Warn("filelist movie browse page failed", zap.Int("page", page), zap.Any("params", params), zap.Error(err))
			break
		}
		movies := idx.ParseBrowseMovies(html)
		if len(movies) == 0 {
			break
		}
		added := 0
		for _, m := range movies {
			if _, ok := seen[m.DownloadLink]; ok {
				continue
			}
			seen[m.DownloadLink] = struct{}{}
			all = append(all, m)
			added++
		}
		if added == 0 {
			p.log.Debug("filelist movie browse page deduplicated to zero", zap.Int("page", page))
			break
		}
	}
	return all
}

func (p *Provider) browseShowsPaginated(ctx context.Context, search, searchin string) []idx.Show {
	const maxPages = 5
	all := make([]idx.Show, 0, 100)
	seen := make(map[string]struct{}, 100)
	for page := 0; page <= maxPages; page++ {
		params := map[string]string{"cat": "0", "sort": "2", "search": search, "searchin": searchin, "page": fmt.Sprintf("%d", page)}
		html, err := p.client.Browse(ctx, params)
		if err != nil {
			p.log.Warn("filelist show browse page failed", zap.Int("page", page), zap.Any("params", params), zap.Error(err))
			break
		}
		shows := idx.ParseBrowseShows(html)
		if len(shows) == 0 {
			break
		}
		added := 0
		for _, s := range shows {
			if _, ok := seen[s.DownloadLink]; ok {
				continue
			}
			seen[s.DownloadLink] = struct{}{}
			all = append(all, s)
			added++
		}
		if added == 0 {
			p.log.Debug("filelist show browse page deduplicated to zero", zap.Int("page", page))
			break
		}
	}
	return all
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") || strings.Contains(lower, "429")
}

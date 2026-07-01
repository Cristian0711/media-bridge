package prowlarr

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	idx "github.com/Cristian0711/media-bridge/backend/internal/indexer"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
)

const (
	providerID   = "prowlarr"
	providerName = "Prowlarr"
	searchMovie  = "movie"
	searchTV     = "tvsearch"
)

type Provider struct {
	idx.BaseIndexer
	client *Client
	log    *slog.Logger
}

func NewProvider(cfg Config, enabled bool) *Provider {
	return &Provider{
		BaseIndexer: idx.BaseIndexer{Name: providerName, ID: providerID, Enabled: enabled},
		client:      NewClient(cfg),
		log:         logger.Component("indexer.prowlarr"),
	}
}

func (p *Provider) SearchMovies(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	return p.search(ctx, req, false)
}

func (p *Provider) SearchShows(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	return p.search(ctx, req, true)
}

func (p *Provider) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	return p.client.DownloadTorrent(ctx, downloadURL)
}

func (p *Provider) search(ctx context.Context, req idx.SearchRequest, tv bool) ([]idx.IndexerItem, error) {
	var query string
	searchType := searchMovie
	if tv {
		searchType = searchTV
		query = TVSearchQuery(req.ImdbID, req.Season, req.Episode)
	} else {
		query = MovieSearchQuery(req.ImdbID)
	}

	indexerIDs, err := p.resolveIndexerIDs(ctx, req.Indexers)
	if err != nil {
		return nil, err
	}

	releases, err := p.client.Search(ctx, query, searchType, indexerIDs)
	if err != nil {
		return nil, err
	}

	items := ToIndexerItems(releases, req.ImdbID, tv)
	if len(req.Indexers) > 0 {
		filtered := make([]idx.IndexerItem, 0, len(items))
		for _, item := range items {
			if MatchIndexerFilter(item.IndexerName, req.Indexers) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	p.log.InfoContext(ctx, "prowlarr mapped results",
		"type", searchType,
		"raw", len(releases),
		"mapped", len(items))
	return items, nil
}

func (p *Provider) resolveIndexerIDs(ctx context.Context, names []string) ([]int, error) {
	if len(names) == 0 {
		return nil, nil
	}
	indexers, err := p.client.ListIndexers(ctx)
	if err != nil {
		p.log.WarnContext(ctx, "prowlarr indexer list failed, searching all indexers", logger.Err(err))
		return nil, nil
	}
	return ResolveIndexerIDs(indexers, names), nil
}

// ListCatalog implements indexer.CatalogLister for Prowlarr child indexers.
func (p *Provider) ListCatalog(ctx context.Context) ([]idx.CatalogEntry, error) {
	indexers, err := p.client.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]idx.CatalogEntry, 0, len(indexers))
	for _, ix := range indexers {
		out = append(out, idx.CatalogEntry{
			ID:      fmt.Sprintf("%d", ix.ID),
			Name:    strings.TrimSpace(ix.Name),
			Enabled: ix.IsEnabled(),
		})
	}
	return out, nil
}

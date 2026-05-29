package blutopia

import (
	"context"
	"fmt"
	"strings"

	idx "github.com/Cristian0711/media-bridge/backend/internal/indexer"
)

type Provider struct {
	idx.BaseIndexer
	client *Client
}

func NewProvider(cfg Config, enabled bool) *Provider {
	return &Provider{
		BaseIndexer: idx.BaseIndexer{Name: "Blutopia", ID: "blutopia", Enabled: enabled},
		client:      NewClient(cfg),
	}
}

func (p *Provider) SearchMovies(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	items, err := p.client.FilterFreeleech(ctx, req.ImdbID)
	if err != nil {
		return nil, err
	}
	return p.toIndexerItems(filterByCategory(items, isMovieCategory)), nil
}

func (p *Provider) SearchShows(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	items, err := p.client.FilterFreeleech(ctx, req.ImdbID)
	if err != nil {
		return nil, err
	}
	return p.toIndexerItems(filterByCategory(items, isShowCategory)), nil
}

func (p *Provider) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	return p.client.DownloadTorrent(ctx, downloadURL)
}

func filterByCategory(items []torrentAttributes, match func(string, int) bool) []torrentAttributes {
	out := make([]torrentAttributes, 0, len(items))
	for _, item := range items {
		if match(item.Category, item.CategoryID) {
			out = append(out, item)
		}
	}
	return out
}

func (p *Provider) toIndexerItems(items []torrentAttributes) []idx.IndexerItem {
	result := make([]idx.IndexerItem, 0, len(items))
	for _, item := range items {
		category := strings.TrimSpace(item.Category)
		if item.Type != "" {
			category = strings.TrimSpace(item.Category + " / " + item.Type)
		}
		if item.Resolution != "" {
			if category != "" {
				category += " · " + item.Resolution
			} else {
				category = item.Resolution
			}
		}
		result = append(result, idx.IndexerItem{
			ID:           item.DownloadLink,
			Name:         item.Name,
			ImdbID:       imdbFromAttrs(item),
			Size:         item.Size,
			Seeders:      item.Seeders,
			Leechers:     item.Leechers,
			Downloads:    item.TimesCompleted,
			DownloadLink: item.DownloadLink,
			Freeleech:    true,
			Category:     category,
			UploadDate:   item.CreatedAt,
			IndexerName:  p.Name,
		})
	}
	return result
}

func imdbFromAttrs(item torrentAttributes) string {
	if item.IMDBID != nil && *item.IMDBID > 0 {
		return fmt.Sprintf("tt%d", *item.IMDBID)
	}
	return ""
}

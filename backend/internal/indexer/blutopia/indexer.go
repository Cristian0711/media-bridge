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

func filterByCategory(items []torrentResource, match func(string, int) bool) []torrentResource {
	out := make([]torrentResource, 0, len(items))
	for _, item := range items {
		attrs := item.Attributes
		if match(attrs.Category, attrs.CategoryID) {
			out = append(out, item)
		}
	}
	return out
}

func (p *Provider) toIndexerItems(items []torrentResource) []idx.IndexerItem {
	result := make([]idx.IndexerItem, 0, len(items))
	for _, item := range items {
		attrs := item.Attributes
		category := strings.TrimSpace(attrs.Category)
		if attrs.Type != "" {
			category = strings.TrimSpace(attrs.Category + " / " + attrs.Type)
		}
		if attrs.Resolution != "" {
			if category != "" {
				category += " · " + attrs.Resolution
			} else {
				category = attrs.Resolution
			}
		}
		result = append(result, idx.IndexerItem{
			ID:           item.ID,
			Name:         attrs.Name,
			ImdbID:       imdbFromAttrs(attrs),
			Size:         attrs.Size,
			Seeders:      attrs.Seeders,
			Leechers:     attrs.Leechers,
			Downloads:    attrs.TimesCompleted,
			DownloadLink: attrs.DownloadLink,
			Freeleech:    true,
			Category:     category,
			UploadDate:   attrs.CreatedAt,
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

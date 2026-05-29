package torrentleech

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
		BaseIndexer: idx.BaseIndexer{Name: "TorrentLeech", ID: "torrentleech", Enabled: enabled},
		client:      NewClient(cfg),
	}
}

func (p *Provider) SearchMovies(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	resp, err := p.client.Search(ctx, req.ImdbID)
	if err != nil {
		return nil, err
	}
	return p.toIndexerItems(resp.TorrentList), nil
}

func (p *Provider) SearchShows(ctx context.Context, req idx.SearchRequest) ([]idx.IndexerItem, error) {
	if req.ImdbID == "" {
		return nil, fmt.Errorf("imdb_id is required")
	}
	resp, err := p.client.Search(ctx, req.ImdbID)
	if err != nil {
		return nil, err
	}
	return p.toIndexerItems(resp.TorrentList), nil
}

func (p *Provider) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	return p.client.DownloadTorrent(ctx, downloadURL)
}

func (p *Provider) toIndexerItems(items []Item) []idx.IndexerItem {
	result := make([]idx.IndexerItem, 0, len(items))
	for _, item := range items {
		result = append(result, idx.IndexerItem{
			ID:           item.Fid,
			Name:         strings.TrimSuffix(item.Filename, ".torrent"),
			ImdbID:       item.ImdbID,
			Size:         item.Size,
			Seeders:      item.Seeders,
			Leechers:     item.Leechers,
			Downloads:    item.Completed,
			DownloadLink: fmt.Sprintf("https://www.torrentleech.org/download/%s/%s", item.Fid, item.Filename),
			Freeleech:    item.DownloadMultiplier == 0,
			Category:     categoryName(item.CategoryID),
			UploadDate:   item.AddedTimestamp,
			IndexerName:  p.Name,
		})
	}
	return result
}

func categoryName(categoryID int) string {
	categories := map[int]string{
		11: "Movies",
		12: "Movies/DVD-R",
		13: "Movies/HD",
		14: "Movies/HD",
		37: "Movies/WEB-DL",
		43: "Movies/4K",
		47: "Movies/4K",
		26: "TV/Episodes",
		27: "TV/Episodes HD",
		32: "TV/Box Sets",
		41: "TV/WEB-DL",
	}
	if name, ok := categories[categoryID]; ok {
		return name
	}
	return fmt.Sprintf("Category %d", categoryID)
}

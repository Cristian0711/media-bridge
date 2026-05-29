package indexer

import "context"

type Provider interface {
	GetName() string
	GetID() string
	IsEnabled() bool
	SearchMovies(ctx context.Context, req SearchRequest) ([]IndexerItem, error)
	SearchShows(ctx context.Context, req SearchRequest) ([]IndexerItem, error)
	DownloadTorrent(ctx context.Context, downloadURL string) (string, error)
}

type BaseIndexer struct {
	Name    string
	ID      string
	Enabled bool
}

func (b *BaseIndexer) GetName() string {
	return b.Name
}

func (b *BaseIndexer) GetID() string {
	return b.ID
}

func (b *BaseIndexer) IsEnabled() bool {
	return b.Enabled
}

package setup

import (
	"github.com/Cristian0711/media-bridge/backend/internal/config"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer/prowlarr"
)

func NewService(cfg config.IndexerConfig) *indexer.Service {
	svc := indexer.NewService()

	prov := prowlarr.NewProvider(prowlarr.Config{
		BaseURL: cfg.Prowlarr.URL,
		APIKey:  cfg.Prowlarr.APIKey,
	}, cfg.Prowlarr.Enabled)
	svc.RegisterIndexer(prov)
	svc.SetCatalog(prov)

	return svc
}

package setup

import (
	"github.com/Cristian0711/media-bridge/backend/internal/config"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer/blutopia"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer/filelist"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer/torrentleech"
)

func NewService(cfg config.IndexerConfig) *indexer.Service {
	svc := indexer.NewService()

	svc.RegisterIndexer(filelist.NewProvider(filelist.Config{
		Username: cfg.FileList.Username,
		Passkey:  cfg.FileList.PassKey,
		UUID:     cfg.FileList.UUID,
		PassID:   cfg.FileList.PassID,
		SessID:   cfg.FileList.SessID,
	}, cfg.FileList.Enabled))

	svc.RegisterIndexer(torrentleech.NewProvider(torrentleech.Config{
		PHPSESSID:   cfg.TorrentLeech.PHPSESSID,
		TLUid:       cfg.TorrentLeech.TLUID,
		TLPass:      cfg.TorrentLeech.TLPASS,
		LastBrowse1: cfg.TorrentLeech.LastBrowse1,
		LastBrowse2: cfg.TorrentLeech.LastBrowse2,
	}, cfg.TorrentLeech.Enabled))

	svc.RegisterIndexer(blutopia.NewProvider(blutopia.Config{
		BaseURL: cfg.Blutopia.BaseURL,
		APIKey:  cfg.Blutopia.APIKey,
	}, cfg.Blutopia.Enabled))

	return svc
}

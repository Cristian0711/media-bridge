package source

import (
	"context"

	"github.com/Cristian0711/media-bridge/backend/internal/download"
	"github.com/Cristian0711/media-bridge/backend/internal/requests"
)

type DownloadSource struct {
	repo requests.Repository
}

func NewDownloadSource(repo requests.Repository) *DownloadSource {
	return &DownloadSource{repo: repo}
}

func (s *DownloadSource) FindByID(ctx context.Context, id uint) (*download.RequestDetails, error) {
	req, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &download.RequestDetails{
		RequestEntryID: req.ID,
		RequestID:      req.RequestID,
		MediaID:        req.MediaID,
		Type:           req.Type,
		Name:           req.Name,
		IMDBID:         req.IMDBID,
		TMDBID:         req.TMDBID,
		TVDBID:         req.TVDBID,
		Season:         req.Season,
		Episode:        req.Episode,
		PosterURL:      req.PosterURL,
		TorrentURL:     req.TorrentURL,
		TorrentName:    req.TorrentName,
		Indexer:        req.Indexer,
		Quality:        req.Quality,
		UserID:         req.UserID,
		Username:       req.Username,
	}, nil
}

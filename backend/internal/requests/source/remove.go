package source

import (
	"context"

	"github.com/Cristian0711/media-bridge/backend/internal/remove"
	"github.com/Cristian0711/media-bridge/backend/internal/requests"
)

type RemoveSource struct {
	repo requests.Repository
}

func NewRemoveSource(repo requests.Repository) *RemoveSource {
	return &RemoveSource{repo: repo}
}

func (s *RemoveSource) FindByID(ctx context.Context, id uint) (*remove.RequestDetails, error) {
	req, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &remove.RequestDetails{
		RequestEntryID: req.ID,
		RequestID:      req.RequestID,
		MediaID:        req.MediaID,
		Type:           req.Type,
		UserID:         req.UserID,
		Username:       req.Username,
	}, nil
}

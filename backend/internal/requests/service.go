package requests

import (
	"context"
	"errors"
	"fmt"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"gorm.io/gorm"
)

type Service interface {
	RequestMovieDownload(ctx context.Context, req MovieDownloadRequestBody, userID uint, username, requestID string) (*RequestAck, error)
	RequestShowDownload(ctx context.Context, req ShowDownloadRequestBody, userID uint, username, requestID string) (*RequestAck, error)
	RequestMovieRemove(ctx context.Context, req MovieRemoveRequestBody, userID uint, username, requestID string) (*RequestAck, error)
	RequestShowRemove(ctx context.Context, req ShowRemoveRequestBody, userID uint, username, requestID string) (*RequestAck, error)
	ListRequests(ctx context.Context, page, pageSize int) (*PaginatedRequestsResponse, error)
	ListRequestsForUser(ctx context.Context, userID uint, page, pageSize int) (*PaginatedRequestsResponse, error)
	ListQueueEntries(ctx context.Context, page, pageSize int) (*PaginatedQueueEntriesResponse, error)
	GetRequestTorrentInfo(ctx context.Context, requestID uint) (*RequestTorrentInfo, error)
	GetRequestTorrentInfoFresh(ctx context.Context, requestID uint) (*RequestTorrentInfo, error)
}

type service struct {
	repo        Repository
	mediaRepo   media.Repository
	processor   *QueueProcessor
	torrentInfo torrentInfoDeps
}

func NewService(
	repo Repository,
	mediaRepo media.Repository,
	processor *QueueProcessor,
	mediaSvc media.Service,
	qbitSvc qbittorrent.Service,
	hardlinkSvc hardlink.Service,
) Service {
	s := &service{
		repo:      repo,
		mediaRepo: mediaRepo,
		processor: processor,
		torrentInfo: torrentInfoDeps{
			mediaSvc:    mediaSvc,
			qbitSvc:     qbitSvc,
			hardlinkSvc: hardlinkSvc,
			cache:       newTorrentInfoCache(defaultTorrentInfoCacheTTL),
		},
	}
	if r, ok := repo.(torrentInfoCacheInvalidator); ok {
		r.SetTorrentInfoInvalidator(s)
	}
	return s
}

func (s *service) InvalidateTorrentInfo(requestID uint) {
	s.torrentInfo.cache.invalidate(requestID)
}

func (s *service) GetRequestTorrentInfo(ctx context.Context, requestID uint) (*RequestTorrentInfo, error) {
	if info, ok := s.torrentInfo.cache.get(requestID); ok {
		return info, nil
	}
	req, err := s.repo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	info, err := s.torrentInfo.build(ctx, req)
	if err != nil {
		return nil, err
	}
	s.torrentInfo.cache.set(requestID, info)
	return info, nil
}

func (s *service) RequestMovieDownload(ctx context.Context, req MovieDownloadRequestBody, userID uint, username, requestID string) (*RequestAck, error) {
	existingMovieIDs, err := s.mediaRepo.FindMovieIDsByExternalIDAndQuality(ctx, req.IMDBID, req.TMDBID, req.Quality)
	if err != nil {
		return nil, err
	}
	if len(existingMovieIDs) > 0 {
		return &RequestAck{
			Status:  "accepted",
			Message: "movie already exists in media for this quality",
		}, nil
	}

	entry := &Request{
		Type:        "movie_download",
		Status:      "pending",
		Name:        req.Name,
		UserID:      userID,
		Username:    username,
		RequestID:   requestID,
		IMDBID:      req.IMDBID,
		TMDBID:      req.TMDBID,
		PosterURL:   req.PosterURL,
		TorrentURL:  req.TorrentURL,
		TorrentName: req.TorrentName,
		Indexer:     req.Indexer,
		Quality:     req.Quality,
	}
	_, created, err := s.repo.CreateMovieDownloadIfAbsent(ctx, entry, req.IMDBID, req.TMDBID, req.Quality, func(tx *gorm.DB, e *Request) error {
		return s.processor.EnqueueInGormTx(ctx, tx, QueuePayload{
			RequestEntryID: e.ID,
			RequestID:      requestID,
			Type:           e.Type,
			UserID:         userID,
			Username:       username,
		})
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return &RequestAck{
			Status:  "accepted",
			Message: "movie download already in progress",
		}, nil
	}
	return &RequestAck{Status: "accepted", Message: "movie download request queued for processing"}, nil
}

func (s *service) RequestShowDownload(ctx context.Context, req ShowDownloadRequestBody, userID uint, username, requestID string) (*RequestAck, error) {
	show, err := s.mediaRepo.FindShowByExternalIDOrName(ctx, req.IMDBID, req.TVDBID, req.Name)
	if err != nil {
		return nil, err
	}
	if show != nil {
		var season *int
		var episode *int
		if req.Season > 0 {
			season = &req.Season
		}
		if req.Episode > 0 {
			episode = &req.Episode
		}
		entryIDs, err := s.mediaRepo.FindShowEntryIDsByShowAndScope(ctx, show.ID, req.Quality, season, episode)
		if err != nil {
			return nil, err
		}
		if len(entryIDs) > 0 {
			return &RequestAck{
				Status:  "accepted",
				Message: "show entry already exists in media for this quality/season/episode",
			}, nil
		}
	}

	entry := &Request{
		Type:        "show_download",
		Status:      "pending",
		Name:        req.Name,
		UserID:      userID,
		Username:    username,
		RequestID:   requestID,
		IMDBID:      req.IMDBID,
		TVDBID:      req.TVDBID,
		Season:      req.Season,
		Episode:     req.Episode,
		PosterURL:   req.PosterURL,
		TorrentURL:  req.TorrentURL,
		TorrentName: req.TorrentName,
		Indexer:     req.Indexer,
		Quality:     req.Quality,
	}
	_, created, err := s.repo.CreateShowDownloadIfAbsent(ctx, entry, req.IMDBID, req.TVDBID, req.Quality, req.Season, req.Episode, func(tx *gorm.DB, e *Request) error {
		return s.processor.EnqueueInGormTx(ctx, tx, QueuePayload{
			RequestEntryID: e.ID,
			RequestID:      requestID,
			Type:           e.Type,
			UserID:         userID,
			Username:       username,
		})
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return &RequestAck{
			Status:  "accepted",
			Message: "show download already in progress",
		}, nil
	}
	return &RequestAck{Status: "accepted", Message: "show download request queued for processing"}, nil
}

func (s *service) RequestMovieRemove(ctx context.Context, req MovieRemoveRequestBody, userID uint, username, requestID string) (*RequestAck, error) {
	mediaRow, err := s.loadMediaForRemove(ctx, req.MediaID)
	if err != nil {
		return nil, err
	}
	if mediaRow.Movie == nil {
		return nil, fmt.Errorf("media %d is not a movie", req.MediaID)
	}
	movie := mediaRow.Movie

	entry := &Request{
		Type:        "movie_remove",
		Status:      "pending",
		Name:        mediaRow.Name,
		UserID:      userID,
		Username:    username,
		RequestID:   requestID,
		MediaID:     req.MediaID,
		IMDBID:      movie.IMDBID,
		TMDBID:      movie.TMDBID,
		PosterURL:   derefString(movie.PosterURL),
		TorrentURL:  derefString(movie.TorrentURL),
		TorrentName: derefString(movie.TorrentName),
		Indexer:     mediaRow.Indexer,
		Quality:     mediaRow.Quality,
	}
	_, created, err := s.repo.CreateRemoveIfAbsent(ctx, entry, req.MediaID, "movie_remove", func(tx *gorm.DB, e *Request) error {
		return s.processor.EnqueueInGormTx(ctx, tx, QueuePayload{
			RequestEntryID: e.ID,
			RequestID:      requestID,
			Type:           e.Type,
			UserID:         userID,
			Username:       username,
		})
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return &RequestAck{
			Status:  "accepted",
			Message: "movie remove already in progress",
		}, nil
	}
	return &RequestAck{Status: "accepted", Message: "movie remove request queued for processing"}, nil
}

func (s *service) RequestShowRemove(ctx context.Context, req ShowRemoveRequestBody, userID uint, username, requestID string) (*RequestAck, error) {
	mediaRow, err := s.loadMediaForRemove(ctx, req.MediaID)
	if err != nil {
		return nil, err
	}
	if mediaRow.ShowEntry == nil || mediaRow.ShowEntry.Show == nil {
		return nil, fmt.Errorf("media %d is not a show entry", req.MediaID)
	}
	showEntry := mediaRow.ShowEntry
	show := showEntry.Show

	entry := &Request{
		Type:        "show_remove",
		Status:      "pending",
		Name:        show.Name,
		UserID:      userID,
		Username:    username,
		RequestID:   requestID,
		MediaID:     req.MediaID,
		IMDBID:      show.IMDBID,
		TVDBID:      show.TVDBID,
		Season:      derefInt(showEntry.Season),
		Episode:     derefInt(showEntry.Episode),
		PosterURL:   derefString(show.PosterURL),
		TorrentURL:  derefString(showEntry.TorrentURL),
		TorrentName: derefString(showEntry.TorrentName),
		Indexer:     mediaRow.Indexer,
		Quality:     mediaRow.Quality,
	}
	_, created, err := s.repo.CreateRemoveIfAbsent(ctx, entry, req.MediaID, "show_remove", func(tx *gorm.DB, e *Request) error {
		return s.processor.EnqueueInGormTx(ctx, tx, QueuePayload{
			RequestEntryID: e.ID,
			RequestID:      requestID,
			Type:           e.Type,
			UserID:         userID,
			Username:       username,
		})
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return &RequestAck{
			Status:  "accepted",
			Message: "show remove already in progress",
		}, nil
	}
	return &RequestAck{Status: "accepted", Message: "show remove request queued for processing"}, nil
}

func (s *service) loadMediaForRemove(ctx context.Context, mediaID uint) (*media.Media, error) {
	row, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}
	return row, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func (s *service) ListRequests(ctx context.Context, page, pageSize int) (*PaginatedRequestsResponse, error) {
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedRequestsResponse{
		Requests:   rows,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		TotalPages: calcTotalPages(total, pageSize),
	}, nil
}

func (s *service) ListRequestsForUser(ctx context.Context, userID uint, page, pageSize int) (*PaginatedRequestsResponse, error) {
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedRequestsResponse{
		Requests:   rows,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		TotalPages: calcTotalPages(total, pageSize),
	}, nil
}

func (s *service) ListQueueEntries(ctx context.Context, page, pageSize int) (*PaginatedQueueEntriesResponse, error) {
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, err := s.processor.ListEntries(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedQueueEntriesResponse{
		Entries:    rows,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
		TotalPages: calcTotalPages(total, pageSize),
	}, nil
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func calcTotalPages(total int64, pageSize int) int {
	if total <= 0 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

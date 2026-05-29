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
	torrentInfo *torrentInfoProvider
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
		repo:        repo,
		mediaRepo:   mediaRepo,
		processor:   processor,
		torrentInfo: newTorrentInfoProvider(mediaSvc, qbitSvc, hardlinkSvc, defaultTorrentInfoCacheTTL),
	}
	repo.SetTorrentInfoInvalidator(s.torrentInfo)
	return s
}

func (s *service) RequestMovieDownload(ctx context.Context, req MovieDownloadRequestBody, userID uint, username, requestID string) (*RequestAck, error) {
	existingMovieIDs, err := s.mediaRepo.FindMovieIDsByExternalIDAndQuality(ctx, req.IMDBID, req.TMDBID, req.Quality)
	if err != nil {
		return nil, err
	}
	if len(existingMovieIDs) > 0 {
		return accepted("movie already exists in media for this quality"), nil
	}

	entry := &Request{
		Type:        TypeMovieDownload,
		Status:      StatusPending,
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
	return s.submit(ctx, entry,
		func(enqueue func(*gorm.DB, *Request) error) (*Request, bool, error) {
			return s.repo.CreateMovieDownloadIfAbsent(ctx, entry, req.IMDBID, req.TMDBID, req.Quality, enqueue)
		},
		"movie download already in progress",
		"movie download request queued for processing",
	)
}

func (s *service) RequestShowDownload(ctx context.Context, req ShowDownloadRequestBody, userID uint, username, requestID string) (*RequestAck, error) {
	exists, err := s.showEntryExists(ctx, req)
	if err != nil {
		return nil, err
	}
	if exists {
		return accepted("show entry already exists in media for this quality/season/episode"), nil
	}

	entry := &Request{
		Type:        TypeShowDownload,
		Status:      StatusPending,
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
	return s.submit(ctx, entry,
		func(enqueue func(*gorm.DB, *Request) error) (*Request, bool, error) {
			return s.repo.CreateShowDownloadIfAbsent(ctx, entry, req.IMDBID, req.TVDBID, req.Quality, req.Season, req.Episode, enqueue)
		},
		"show download already in progress",
		"show download request queued for processing",
	)
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
		Type:        TypeMovieRemove,
		Status:      StatusPending,
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
	return s.submit(ctx, entry,
		func(enqueue func(*gorm.DB, *Request) error) (*Request, bool, error) {
			return s.repo.CreateRemoveIfAbsent(ctx, entry, req.MediaID, TypeMovieRemove, enqueue)
		},
		"movie remove already in progress",
		"movie remove request queued for processing",
	)
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
		Type:        TypeShowRemove,
		Status:      StatusPending,
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
	return s.submit(ctx, entry,
		func(enqueue func(*gorm.DB, *Request) error) (*Request, bool, error) {
			return s.repo.CreateRemoveIfAbsent(ctx, entry, req.MediaID, TypeShowRemove, enqueue)
		},
		"show remove already in progress",
		"show remove request queued for processing",
	)
}

// submit runs the shared create-then-enqueue flow for every request entry point.
// create wraps the matching repository dedup-creator; the enqueue closure routes
// the new row to the requests queue inside the same transaction (R2). The ack
// message depends on whether a new row was created or a duplicate was folded in.
func (s *service) submit(
	ctx context.Context,
	entry *Request,
	create func(enqueue func(*gorm.DB, *Request) error) (*Request, bool, error),
	inProgressMsg, queuedMsg string,
) (*RequestAck, error) {
	_, created, err := create(func(tx *gorm.DB, e *Request) error {
		return s.processor.EnqueueInGormTx(ctx, tx, QueuePayload{
			RequestEntryID: e.ID,
			RequestID:      e.RequestID,
			Type:           e.Type,
			UserID:         e.UserID,
			Username:       e.Username,
		})
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return accepted(inProgressMsg), nil
	}
	return accepted(queuedMsg), nil
}

// showEntryExists reports whether the requested show/season/episode is already
// present in the media library for the given quality.
func (s *service) showEntryExists(ctx context.Context, req ShowDownloadRequestBody) (bool, error) {
	show, err := s.mediaRepo.FindShowByExternalIDOrName(ctx, req.IMDBID, req.TVDBID, req.Name)
	if err != nil {
		return false, err
	}
	if show == nil {
		return false, nil
	}
	var season, episode *int
	if req.Season > 0 {
		season = &req.Season
	}
	if req.Episode > 0 {
		episode = &req.Episode
	}
	entryIDs, err := s.mediaRepo.FindShowEntryIDsByShowAndScope(ctx, show.ID, req.Quality, season, episode)
	if err != nil {
		return false, err
	}
	return len(entryIDs) > 0, nil
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

func (s *service) GetRequestTorrentInfo(ctx context.Context, requestID uint) (*RequestTorrentInfo, error) {
	if info, ok := s.torrentInfo.cached(requestID); ok {
		return info, nil
	}
	req, err := s.repo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return s.torrentInfo.buildAndCache(ctx, req)
}

// GetRequestTorrentInfoFresh loads live qBittorrent/hardlink state without the
// short HTTP cache, used by the torrent SSE stream.
func (s *service) GetRequestTorrentInfoFresh(ctx context.Context, requestID uint) (*RequestTorrentInfo, error) {
	req, err := s.repo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return s.torrentInfo.buildAndCache(ctx, req)
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

func accepted(message string) *RequestAck {
	return &RequestAck{Status: "accepted", Message: message}
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

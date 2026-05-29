package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CreateFromRequestInput struct {
	MediaID     uint
	Type        string
	Name        string
	IMDBID      string
	TMDBID      string
	TVDBID      string
	Season      int
	Episode     int
	PosterURL   string
	TorrentURL  string
	TorrentName string
	Indexer     string
	Quality     string
	UserID      uint
	Username    string
	RequestID   string
	SavePath    *string
	TorrentHash *string
	SizeBytes   *int64
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type Service interface {
	CreateFromRequest(ctx context.Context, input CreateFromRequestInput) (uint, error)
	// FindExistingDownloadMediaID returns the newest media row for this download scope, or 0 if none.
	FindExistingDownloadMediaID(ctx context.Context, input CreateFromRequestInput) (uint, error)
	RemoveFromRequest(ctx context.Context, input CreateFromRequestInput) error
	UpdateLibraryPath(ctx context.Context, mediaID uint, libraryPath string) error
	GetAllMediaPaginated(ctx context.Context, page, pageSize int) (*PaginatedMediaResponse, error)
	GetMediaForUserPaginated(ctx context.Context, userID uint, page, pageSize int) (*PaginatedMediaResponse, error)
	GetMediaByID(ctx context.Context, id uint) (*Media, error)
	SearchMedia(ctx context.Context, query string, page, pageSize int) (*PaginatedMediaResponse, error)
	SearchMediaForUser(ctx context.Context, userID uint, query string, page, pageSize int) (*PaginatedMediaResponse, error)
}

type service struct {
	repo      Repository
	publisher sse.Publisher
}

// NewService builds the media service. Pass sse.NoopPublisher{} when SSE is disabled.
func NewService(repo Repository, publisher sse.Publisher) Service {
	if publisher == nil {
		publisher = sse.NoopPublisher{}
	}
	return &service{repo: repo, publisher: publisher}
}

func (s *service) CreateFromRequest(ctx context.Context, input CreateFromRequestInput) (uint, error) {
	if strings.HasPrefix(input.Type, "movie_") {
		return s.createMovie(ctx, input)
	}
	if strings.HasPrefix(input.Type, "show_") {
		return s.createShow(ctx, input)
	}
	return 0, fmt.Errorf("unsupported request type for media tracking: %s", input.Type)
}

// FindExistingDownloadMediaID locates a media row already created for this download
// (e.g. a prior attempt linked the DB row but failed before setting request.media_id).
func (s *service) FindExistingDownloadMediaID(ctx context.Context, input CreateFromRequestInput) (uint, error) {
	switch {
	case strings.HasPrefix(input.Type, "movie_download"):
		ids, err := s.repo.FindMediaIDsForMovieDownload(ctx, input.IMDBID, input.TMDBID, input.Quality)
		if err != nil {
			return 0, err
		}
		return newestID(ids), nil
	case strings.HasPrefix(input.Type, "show_download"):
		show, err := s.repo.FindShowByExternalIDOrName(ctx, input.IMDBID, input.TVDBID, input.Name)
		if err != nil {
			return 0, err
		}
		if show == nil {
			return 0, nil
		}
		var season, episode *int
		if input.Season > 0 {
			season = &input.Season
		}
		if input.Episode > 0 {
			episode = &input.Episode
		}
		ids, err := s.repo.FindMediaIDsByShowScope(ctx, show.ID, input.Quality, season, episode)
		if err != nil {
			return 0, err
		}
		return newestID(ids), nil
	default:
		return 0, nil
	}
}

func newestID(ids []uint) uint {
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func (s *service) RemoveFromRequest(ctx context.Context, input CreateFromRequestInput) error {
	if strings.HasPrefix(input.Type, "movie_") {
		return s.removeMovie(ctx, input)
	}
	if strings.HasPrefix(input.Type, "show_") {
		return s.removeShow(ctx, input)
	}
	return fmt.Errorf("unsupported request type for media removal: %s", input.Type)
}

func (s *service) UpdateLibraryPath(ctx context.Context, mediaID uint, libraryPath string) error {
	if mediaID == 0 || libraryPath == "" {
		return nil
	}
	return s.repo.UpdateLibraryPath(ctx, mediaID, libraryPath)
}

func (s *service) createMovie(ctx context.Context, input CreateFromRequestInput) (uint, error) {
	movie := Movie{
		IMDBID:      input.IMDBID,
		TMDBID:      input.TMDBID,
		PosterURL:   stringPtr(input.PosterURL),
		TorrentHash: input.TorrentHash,
		TorrentURL:  stringPtr(input.TorrentURL),
		TorrentName: stringPtr(input.TorrentName),
		SavePath:    input.SavePath,
		StartedAt:   input.StartedAt,
		CompletedAt: input.CompletedAt,
	}
	media := Media{
		Type:      MediaTypeMovie,
		Name:      input.Name,
		Path:      resolvePath(input),
		Indexer:   input.Indexer,
		Quality:   input.Quality,
		SizeBytes: derefSizeBytes(input.SizeBytes),
		UserID:    input.UserID,
		Username:  input.Username,
	}
	if err := s.repo.CreateMovieWithMedia(ctx, &movie, &media); err != nil {
		return 0, err
	}
	s.emitMediaAdded(ctx, media.ID)
	return media.ID, nil
}

func (s *service) GetAllMediaPaginated(ctx context.Context, page, pageSize int) (*PaginatedMediaResponse, error) {
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, totalSize, err := s.repo.ListPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedMediaResponse{
		Media:          rows,
		Page:           page,
		PageSize:       pageSize,
		TotalCount:     total,
		TotalSizeBytes: totalSize,
		TotalPages:     calcTotalPages(total, pageSize),
	}, nil
}

func (s *service) GetMediaForUserPaginated(ctx context.Context, userID uint, page, pageSize int) (*PaginatedMediaResponse, error) {
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, totalSize, err := s.repo.ListByUserPaginated(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedMediaResponse{
		Media:          rows,
		Page:           page,
		PageSize:       pageSize,
		TotalCount:     total,
		TotalSizeBytes: totalSize,
		TotalPages:     calcTotalPages(total, pageSize),
	}, nil
}

func (s *service) GetMediaByID(ctx context.Context, id uint) (*Media, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrMediaNotFound
		}
		return nil, err
	}
	return row, nil
}

func (s *service) SearchMedia(ctx context.Context, query string, page, pageSize int) (*PaginatedMediaResponse, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return &PaginatedMediaResponse{Media: []Media{}, Page: 1, PageSize: pageSize}, nil
	}
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, totalSize, err := s.repo.SearchByName(ctx, q, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedMediaResponse{
		Media:          rows,
		Page:           page,
		PageSize:       pageSize,
		TotalCount:     total,
		TotalSizeBytes: totalSize,
		TotalPages:     calcTotalPages(total, pageSize),
	}, nil
}

func (s *service) SearchMediaForUser(ctx context.Context, userID uint, query string, page, pageSize int) (*PaginatedMediaResponse, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return &PaginatedMediaResponse{Media: []Media{}, Page: 1, PageSize: pageSize}, nil
	}
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, totalSize, err := s.repo.SearchByNameForUser(ctx, userID, q, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PaginatedMediaResponse{
		Media:          rows,
		Page:           page,
		PageSize:       pageSize,
		TotalCount:     total,
		TotalSizeBytes: totalSize,
		TotalPages:     calcTotalPages(total, pageSize),
	}, nil
}

func (s *service) createShow(ctx context.Context, input CreateFromRequestInput) (uint, error) {
	show := Show{
		Name:      input.Name,
		IMDBID:    input.IMDBID,
		TVDBID:    input.TVDBID,
		PosterURL: stringPtr(input.PosterURL),
	}
	var season *int
	var episode *int
	if input.Season > 0 {
		season = &input.Season
	}
	if input.Episode > 0 {
		episode = &input.Episode
	}

	showEntry := ShowEntry{
		Season:      season,
		Episode:     episode,
		TorrentHash: input.TorrentHash,
		TorrentURL:  stringPtr(input.TorrentURL),
		TorrentName: stringPtr(input.TorrentName),
		SavePath:    input.SavePath,
		StartedAt:   input.StartedAt,
		CompletedAt: input.CompletedAt,
	}

	media := Media{
		Type:      showEntry.GetShowEntryType(),
		Name:      input.Name,
		Path:      resolvePath(input),
		Indexer:   input.Indexer,
		Quality:   input.Quality,
		SizeBytes: derefSizeBytes(input.SizeBytes),
		UserID:    input.UserID,
		Username:  input.Username,
	}
	if err := s.repo.CreateShowEntryWithMedia(ctx, &show, &showEntry, &media); err != nil {
		return 0, err
	}
	s.emitMediaAdded(ctx, media.ID)
	return media.ID, nil
}

func resolvePath(input CreateFromRequestInput) string {
	if input.SavePath != nil && *input.SavePath != "" {
		return *input.SavePath
	}
	if input.TorrentName != "" {
		return "pending://" + input.TorrentName
	}
	return "pending://request/" + input.RequestID
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func derefSizeBytes(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func (s *service) removeMovie(ctx context.Context, input CreateFromRequestInput) error {
	log := logger.Named("media.remove")
	if input.MediaID == 0 {
		return fmt.Errorf("movie removal requires media_id")
	}
	mediaRow, err := s.repo.FindByID(ctx, input.MediaID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Info("movie remove skipped: media not found", zap.Uint("media_id", input.MediaID))
			return nil
		}
		return err
	}
	if mediaRow.MovieID == nil {
		return fmt.Errorf("media %d is not linked to a movie", input.MediaID)
	}
	s.publisher.PublishMediaRemoved(ctx, ToSSEPayload(mediaRow))
	if err := s.repo.DeleteMovieMediaCascade(ctx, input.MediaID, *mediaRow.MovieID); err != nil {
		return err
	}
	log.Info("removed media row",
		zap.Uint("media_id", mediaRow.ID),
		zap.String("media_type", string(mediaRow.Type)),
		zap.String("name", mediaRow.Name),
		zap.Uint("movie_id", *mediaRow.MovieID),
	)
	return nil
}

func (s *service) removeShow(ctx context.Context, input CreateFromRequestInput) error {
	log := logger.Named("media.remove")
	if input.MediaID == 0 {
		return fmt.Errorf("show removal requires media_id")
	}
	mediaRow, err := s.repo.FindByID(ctx, input.MediaID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Info("show remove skipped: media not found", zap.Uint("media_id", input.MediaID))
			return nil
		}
		return err
	}
	if mediaRow.ShowEntryID == nil {
		return fmt.Errorf("media %d is not linked to a show entry", input.MediaID)
	}

	s.publisher.PublishMediaRemoved(ctx, ToSSEPayload(mediaRow))
	showID := uint(0)
	if mediaRow.ShowEntry != nil && mediaRow.ShowEntry.ShowID != 0 {
		showID = mediaRow.ShowEntry.ShowID
	}
	if err := s.repo.DeleteShowMediaCascade(ctx, input.MediaID, *mediaRow.ShowEntryID, showID); err != nil {
		return err
	}
	log.Info("removed media row",
		zap.Uint("media_id", mediaRow.ID),
		zap.String("media_type", string(mediaRow.Type)),
		zap.String("name", mediaRow.Name),
		zap.Uint("show_entry_id", *mediaRow.ShowEntryID),
	)
	return nil
}

// emitMediaAdded loads the persisted row (with associations) and notifies SSE clients.
func (s *service) emitMediaAdded(ctx context.Context, mediaID uint) {
	row, err := s.repo.FindByID(ctx, mediaID)
	if err != nil {
		logger.Named("media.sse").Warn("skip media.added event: load failed",
			zap.Uint("media_id", mediaID),
			zap.Error(err),
		)
		return
	}
	s.publisher.PublishMediaAdded(ctx, ToSSEPayload(row))
}

func mediaString[T any](src *T, extract func(*T) string) string {
	if src == nil {
		return ""
	}
	return extract(src)
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

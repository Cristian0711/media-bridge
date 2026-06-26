package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/Cristian0711/media-bridge/backend/shared/pagination"
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
	// CheckAvailability reports, per requested title, whether it is in the library and at which qualities.
	CheckAvailability(ctx context.Context, items []AvailabilityItem) ([]AvailabilityResult, error)
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
	return paginatedResponse(rows, total, totalSize, page, pageSize), nil
}

func (s *service) GetMediaForUserPaginated(ctx context.Context, userID uint, page, pageSize int) (*PaginatedMediaResponse, error) {
	page, pageSize = normalizePagination(page, pageSize)
	rows, total, totalSize, err := s.repo.ListByUserPaginated(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return paginatedResponse(rows, total, totalSize, page, pageSize), nil
}

func (s *service) GetMediaByID(ctx context.Context, id uint) (*Media, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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
	return paginatedResponse(rows, total, totalSize, page, pageSize), nil
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
	return paginatedResponse(rows, total, totalSize, page, pageSize), nil
}

func (s *service) createShow(ctx context.Context, input CreateFromRequestInput) (uint, error) {
	show := Show{
		Name:      input.Name,
		IMDBID:    input.IMDBID,
		TMDBID:    input.TMDBID,
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
	return s.removeMedia(ctx, "movie", input.MediaID, func(ctx context.Context, row *Media) (slog.Attr, error) {
		if row.MovieID == nil {
			return slog.Attr{}, fmt.Errorf("media %d is not linked to a movie", row.ID)
		}
		s.publisher.PublishMediaRemoved(ctx, ToSSEPayload(row))
		if err := s.repo.DeleteMovieMediaCascade(ctx, row.ID, *row.MovieID); err != nil {
			return slog.Attr{}, err
		}
		return slog.Uint64("movie_id", uint64(*row.MovieID)), nil
	})
}

func (s *service) removeShow(ctx context.Context, input CreateFromRequestInput) error {
	return s.removeMedia(ctx, "show", input.MediaID, func(ctx context.Context, row *Media) (slog.Attr, error) {
		if row.ShowEntryID == nil {
			return slog.Attr{}, fmt.Errorf("media %d is not linked to a show entry", row.ID)
		}
		s.publisher.PublishMediaRemoved(ctx, ToSSEPayload(row))
		if err := s.repo.DeleteShowMediaCascade(ctx, row.ID, *row.ShowEntryID); err != nil {
			return slog.Attr{}, err
		}
		return slog.Uint64("show_entry_id", uint64(*row.ShowEntryID)), nil
	})
}

// removeMedia is the shared skeleton for movie/show removal: it validates the
// id, loads the row (treating a missing row as already-removed success), then
// delegates linkage validation + SSE publish + cascade delete to remove, which
// returns the type-specific log field.
func (s *service) removeMedia(
	ctx context.Context,
	kind string,
	mediaID uint,
	remove func(ctx context.Context, row *Media) (slog.Attr, error),
) error {
	log := logger.Component("media.remove")
	if mediaID == 0 {
		return fmt.Errorf("%s removal requires media_id", kind)
	}
	mediaRow, err := s.repo.FindByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.InfoContext(ctx, kind+" remove skipped: media not found", "media_id", mediaID)
			return nil
		}
		return err
	}
	idField, err := remove(ctx, mediaRow)
	if err != nil {
		return err
	}
	args := []any{"media_id", mediaRow.ID, "media_type", string(mediaRow.Type), "name", mediaRow.Name}
	if idField.Key != "" {
		args = append(args, idField)
	}
	log.InfoContext(ctx, "removed media row", args...)
	return nil
}

// emitMediaAdded loads the persisted row (with associations) and notifies SSE clients.
func (s *service) emitMediaAdded(ctx context.Context, mediaID uint) {
	row, err := s.repo.FindByID(ctx, mediaID)
	if err != nil {
		logger.Component("media.sse").WarnContext(ctx, "skip media.added event: load failed",
			"media_id", mediaID,
			logger.Err(err),
		)
		return
	}
	s.publisher.PublishMediaAdded(ctx, ToSSEPayload(row))
}

func paginatedResponse(rows []Media, total, totalSize int64, page, pageSize int) *PaginatedMediaResponse {
	return &PaginatedMediaResponse{
		Media:          rows,
		Page:           page,
		PageSize:       pageSize,
		TotalCount:     total,
		TotalSizeBytes: totalSize,
		TotalPages:     calcTotalPages(total, pageSize),
	}
}

func normalizePagination(page, pageSize int) (int, int) {
	return pagination.Normalize(page, pageSize)
}

func calcTotalPages(total int64, pageSize int) int {
	return pagination.TotalPages(total, pageSize)
}

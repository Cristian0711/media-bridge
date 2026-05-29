package media

import (
	"context"

	"gorm.io/gorm"
)

type Repository interface {
	CreateMovieWithMedia(ctx context.Context, movie *Movie, media *Media) error
	CreateShowEntryWithMedia(ctx context.Context, show *Show, entry *ShowEntry, media *Media) error
	FindByID(ctx context.Context, id uint) (*Media, error)
	ListPaginated(ctx context.Context, page, pageSize int) ([]Media, int64, int64, error)
	ListByUserPaginated(ctx context.Context, userID uint, page, pageSize int) ([]Media, int64, int64, error)
	SearchByName(ctx context.Context, name string, page, pageSize int) ([]Media, int64, int64, error)
	SearchByNameForUser(ctx context.Context, userID uint, name string, page, pageSize int) ([]Media, int64, int64, error)
	FindMovieIDsByExternalIDAndQuality(ctx context.Context, imdbID, tmdbID, quality string) ([]uint, error)
	// FindMediaIDsForMovieDownload returns media row IDs for an in-flight movie download scope.
	FindMediaIDsForMovieDownload(ctx context.Context, imdbID, tmdbID, quality string) ([]uint, error)
	// FindMediaIDsByShowScope returns media row IDs for a show + season/episode scope.
	FindMediaIDsByShowScope(ctx context.Context, showID uint, quality string, season, episode *int) ([]uint, error)
	DeleteMediaByIDs(ctx context.Context, mediaIDs []uint) error
	DeleteMediaByMovieIDs(ctx context.Context, movieIDs []uint) error
	CountMediaByMovieID(ctx context.Context, movieID uint) (int64, error)
	DeleteMoviesByIDs(ctx context.Context, movieIDs []uint) error
	FindShowByExternalIDOrName(ctx context.Context, imdbID, tvdbID, name string) (*Show, error)
	FindShowEntryIDsByShowAndScope(ctx context.Context, showID uint, quality string, season, episode *int) ([]uint, error)
	CountMediaByShowEntryID(ctx context.Context, showEntryID uint) (int64, error)
	DeleteMediaByShowEntryIDs(ctx context.Context, showEntryIDs []uint) error
	DeleteShowEntriesByIDs(ctx context.Context, showEntryIDs []uint) error
	CountShowEntriesByShowID(ctx context.Context, showID uint) (int64, error)
	DeleteShowByID(ctx context.Context, showID uint) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindByID(ctx context.Context, id uint) (*Media, error) {
	var row Media
	if err := r.db.WithContext(ctx).
		Model(&Media{}).
		Preload("Movie").
		Preload("ShowEntry.Show").
		First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func mediaCountAndSize(db *gorm.DB) (count int64, sizeSum int64, err error) {
	if err = db.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return 0, 0, err
	}
	err = db.Session(&gorm.Session{}).Select("COALESCE(SUM(size_bytes), 0)").Scan(&sizeSum).Error
	return count, sizeSum, err
}

func (r *repository) ListPaginated(ctx context.Context, page, pageSize int) ([]Media, int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&Media{})
	total, totalSize, err := mediaCountAndSize(base)
	if err != nil {
		return nil, 0, 0, err
	}
	rows := make([]Media, 0, pageSize)
	if err := r.db.WithContext(ctx).
		Model(&Media{}).
		Preload("Movie").
		Preload("ShowEntry.Show").
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, 0, err
	}
	return rows, total, totalSize, nil
}

func (r *repository) ListByUserPaginated(ctx context.Context, userID uint, page, pageSize int) ([]Media, int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&Media{}).Where("user_id = ?", userID)
	total, totalSize, err := mediaCountAndSize(base)
	if err != nil {
		return nil, 0, 0, err
	}
	rows := make([]Media, 0, pageSize)
	if err := r.db.WithContext(ctx).Model(&Media{}).Where("user_id = ?", userID).
		Preload("Movie").
		Preload("ShowEntry.Show").
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, 0, err
	}
	return rows, total, totalSize, nil
}

func (r *repository) SearchByName(ctx context.Context, name string, page, pageSize int) ([]Media, int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&Media{}).Where("name % ?", name)
	total, totalSize, err := mediaCountAndSize(base)
	if err != nil {
		return nil, 0, 0, err
	}
	rows := make([]Media, 0, pageSize)
	err = r.db.WithContext(ctx).Model(&Media{}).Where("name % ?", name).
		Select("media.*, similarity(media.name, ?) AS score", name).
		Preload("Movie").
		Preload("ShowEntry.Show").
		Order("score DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, 0, err
	}
	return rows, total, totalSize, nil
}

func (r *repository) SearchByNameForUser(ctx context.Context, userID uint, name string, page, pageSize int) ([]Media, int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&Media{}).Where("name % ?", name).Where("user_id = ?", userID)
	total, totalSize, err := mediaCountAndSize(base)
	if err != nil {
		return nil, 0, 0, err
	}
	rows := make([]Media, 0, pageSize)
	err = r.db.WithContext(ctx).Model(&Media{}).Where("name % ?", name).Where("user_id = ?", userID).
		Select("media.*, similarity(media.name, ?) AS score", name).
		Preload("Movie").
		Preload("ShowEntry.Show").
		Order("score DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, 0, err
	}
	return rows, total, totalSize, nil
}

func (r *repository) CreateMovieWithMedia(ctx context.Context, movie *Movie, media *Media) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(movie).Error; err != nil {
			return err
		}
		media.MovieID = &movie.ID
		return tx.Create(media).Error
	})
}

func (r *repository) CreateShowEntryWithMedia(ctx context.Context, show *Show, entry *ShowEntry, media *Media) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(show).Error; err != nil {
			return err
		}
		entry.ShowID = show.ID
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		media.ShowEntryID = &entry.ID
		return tx.Create(media).Error
	})
}

func (r *repository) FindMediaIDsForMovieDownload(ctx context.Context, imdbID, tmdbID, quality string) ([]uint, error) {
	if imdbID == "" && tmdbID == "" {
		return nil, nil
	}
	query := r.db.WithContext(ctx).
		Model(&Media{}).
		Joins("JOIN movies ON movies.id = media.movie_id").
		Where("media.quality = ?", quality).
		Where("media.type = ?", MediaTypeMovie)
	if imdbID != "" {
		query = query.Where("movies.imdb_id = ?", imdbID)
	} else {
		query = query.Where("movies.tmdb_id = ?", tmdbID)
	}
	var ids []uint
	if err := query.Order("media.id DESC").Pluck("media.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *repository) FindMediaIDsByShowScope(ctx context.Context, showID uint, quality string, season, episode *int) ([]uint, error) {
	query := r.db.WithContext(ctx).
		Model(&Media{}).
		Joins("JOIN show_entries ON show_entries.id = media.show_entry_id").
		Where("show_entries.show_id = ?", showID).
		Where("media.quality = ?", quality)
	if season != nil {
		query = query.Where("show_entries.season = ?", *season)
	} else {
		query = query.Where("show_entries.season IS NULL")
	}
	if episode != nil {
		query = query.Where("show_entries.episode = ?", *episode)
	} else {
		query = query.Where("show_entries.episode IS NULL")
	}
	var ids []uint
	if err := query.Order("media.id DESC").Pluck("media.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *repository) FindMovieIDsByExternalIDAndQuality(ctx context.Context, imdbID, tmdbID, quality string) ([]uint, error) {
	query := r.db.WithContext(ctx).
		Model(&Movie{}).
		Distinct("movies.id").
		Joins("JOIN media ON media.movie_id = movies.id").
		Where("media.quality = ?", quality)

	if imdbID != "" {
		query = query.Where("movies.imdb_id = ?", imdbID)
	} else {
		query = query.Where("movies.tmdb_id = ?", tmdbID)
	}

	var ids []uint
	if err := query.Pluck("movies.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *repository) DeleteMediaByMovieIDs(ctx context.Context, movieIDs []uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("movie_id IN ?", movieIDs).Delete(&Media{}).Error
}

func (r *repository) DeleteMediaByIDs(ctx context.Context, mediaIDs []uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("id IN ?", mediaIDs).Delete(&Media{}).Error
}

func (r *repository) CountMediaByMovieID(ctx context.Context, movieID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Media{}).Where("movie_id = ?", movieID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) DeleteMoviesByIDs(ctx context.Context, movieIDs []uint) error {
	return r.db.WithContext(ctx).Delete(&Movie{}, movieIDs).Error
}

func (r *repository) FindShowByExternalIDOrName(ctx context.Context, imdbID, tvdbID, name string) (*Show, error) {
	query := r.db.WithContext(ctx).Model(&Show{})
	if imdbID != "" {
		query = query.Where("imdb_id = ?", imdbID)
	} else if tvdbID != "" {
		query = query.Where("tvdb_id = ?", tvdbID)
	} else {
		query = query.Where("name = ?", name)
	}

	var show Show
	if err := query.First(&show).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &show, nil
}

func (r *repository) FindShowEntryIDsByShowAndScope(ctx context.Context, showID uint, quality string, season, episode *int) ([]uint, error) {
	query := r.db.WithContext(ctx).
		Model(&ShowEntry{}).
		Distinct("show_entries.id").
		Joins("JOIN media ON media.show_entry_id = show_entries.id").
		Where("show_entries.show_id = ?", showID).
		Where("media.quality = ?", quality)

	if season != nil {
		query = query.Where("show_entries.season = ?", *season)
	}
	if episode != nil {
		query = query.Where("show_entries.episode = ?", *episode)
	}

	var ids []uint
	if err := query.Pluck("show_entries.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *repository) DeleteMediaByShowEntryIDs(ctx context.Context, showEntryIDs []uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("show_entry_id IN ?", showEntryIDs).Delete(&Media{}).Error
}

func (r *repository) CountMediaByShowEntryID(ctx context.Context, showEntryID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Media{}).Where("show_entry_id = ?", showEntryID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) DeleteShowEntriesByIDs(ctx context.Context, showEntryIDs []uint) error {
	return r.db.WithContext(ctx).Delete(&ShowEntry{}, showEntryIDs).Error
}

func (r *repository) CountShowEntriesByShowID(ctx context.Context, showID uint) (int64, error) {
	var remaining int64
	if err := r.db.WithContext(ctx).Model(&ShowEntry{}).Where("show_id = ?", showID).Count(&remaining).Error; err != nil {
		return 0, err
	}
	return remaining, nil
}

func (r *repository) DeleteShowByID(ctx context.Context, showID uint) error {
	return r.db.WithContext(ctx).Delete(&Show{}, showID).Error
}

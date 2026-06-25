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
	FindShowByExternalIDOrName(ctx context.Context, imdbID, tvdbID, name string) (*Show, error)
	FindShowEntryIDsByShowAndScope(ctx context.Context, showID uint, quality string, season, episode *int) ([]uint, error)
	// MovieQualitiesByExternalIDs / ShowQualitiesByExternalIDs back the bulk availability lookup.
	MovieQualitiesByExternalIDs(ctx context.Context, imdbIDs, tmdbIDs []string) ([]movieQualityRow, error)
	ShowQualitiesByExternalIDs(ctx context.Context, imdbIDs, tvdbIDs []string) ([]showQualityRow, error)
	// DeleteMovieMediaCascade removes media + orphan movie in one transaction (R3).
	DeleteMovieMediaCascade(ctx context.Context, mediaID, movieID uint) error
	// DeleteShowMediaCascade removes media + orphan show_entry/show in one transaction (R3).
	// The show id is resolved from the show_entry inside the transaction.
	DeleteShowMediaCascade(ctx context.Context, mediaID, showEntryID uint) error
	UpdateLibraryPath(ctx context.Context, mediaID uint, libraryPath string) error
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
		if err := findOrCreateShow(tx, show); err != nil {
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

// findOrCreateShow reuses an existing Show matched by external id (or name as a
// last resort) so multiple episode downloads of the same series do not create
// duplicate Show rows. On match, *show is replaced with the stored row so the
// caller links the entry to the existing show id.
func findOrCreateShow(tx *gorm.DB, show *Show) error {
	q := tx.Model(&Show{})
	switch {
	case show.IMDBID != "":
		q = q.Where("imdb_id = ?", show.IMDBID)
	case show.TVDBID != "":
		q = q.Where("tvdb_id = ?", show.TVDBID)
	default:
		q = q.Where("name = ?", show.Name)
	}

	var existing Show
	err := q.First(&existing).Error
	if err == nil {
		*show = existing
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return tx.Create(show).Error
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

	// Match the exact scope: a nil season/episode represents a full show or full
	// season, which must dedup only against a stored NULL row — not against
	// arbitrary individual episodes. This mirrors FindMediaIDsByShowScope so the
	// dedup pre-check and the retry idempotency lookup agree (B2).
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
	if err := query.Pluck("show_entries.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *repository) DeleteMovieMediaCascade(ctx context.Context, mediaID, movieID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", mediaID).Delete(&Media{}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&Media{}).Where("movie_id = ?", movieID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		return tx.Delete(&Movie{}, movieID).Error
	})
}

func (r *repository) DeleteShowMediaCascade(ctx context.Context, mediaID, showEntryID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Resolve the parent show id from the entry before deleting it, rather
		// than trusting a preloaded association on the caller's media row.
		var showID uint
		if err := tx.Model(&ShowEntry{}).
			Where("id = ?", showEntryID).
			Select("show_id").
			Scan(&showID).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", mediaID).Delete(&Media{}).Error; err != nil {
			return err
		}
		var entryCount int64
		if err := tx.Model(&Media{}).Where("show_entry_id = ?", showEntryID).Count(&entryCount).Error; err != nil {
			return err
		}
		if entryCount > 0 {
			return nil
		}
		if err := tx.Where("id = ?", showEntryID).Delete(&ShowEntry{}).Error; err != nil {
			return err
		}
		if showID == 0 {
			return nil
		}
		var showCount int64
		if err := tx.Model(&ShowEntry{}).Where("show_id = ?", showID).Count(&showCount).Error; err != nil {
			return err
		}
		if showCount > 0 {
			return nil
		}
		return tx.Delete(&Show{}, showID).Error
	})
}

func (r *repository) UpdateLibraryPath(ctx context.Context, mediaID uint, libraryPath string) error {
	return r.db.WithContext(ctx).
		Model(&Media{}).
		Where("id = ?", mediaID).
		Update("library_path", libraryPath).Error
}

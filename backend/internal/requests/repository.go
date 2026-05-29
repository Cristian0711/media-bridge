package requests

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, req *Request) error
	// CreateMovieDownloadIfAbsent atomically dedupes in-flight movie downloads (M1).
	// When enqueue is non-nil it runs in the same transaction after a new row is created (R2).
	CreateMovieDownloadIfAbsent(ctx context.Context, req *Request, imdbID, tmdbID, quality string, enqueue func(tx *gorm.DB, entry *Request) error) (*Request, bool, error)
	// CreateShowDownloadIfAbsent atomically dedupes in-flight show downloads (M1).
	CreateShowDownloadIfAbsent(ctx context.Context, req *Request, imdbID, tvdbID, quality string, season, episode int, enqueue func(tx *gorm.DB, entry *Request) error) (*Request, bool, error)
	// CreateRemoveIfAbsent atomically dedupes in-flight remove requests for a media row (R1).
	CreateRemoveIfAbsent(ctx context.Context, req *Request, mediaID uint, requestType string, enqueue func(tx *gorm.DB, entry *Request) error) (*Request, bool, error)
	UpdateStatus(ctx context.Context, requestID uint, status string) error
	// MarkDownloadingIfQueued moves queued → downloading after qBittorrent accepts the torrent (M2).
	MarkDownloadingIfQueued(ctx context.Context, requestID uint) (bool, error)
	// MarkDownloadedIfDownloading sets status to 'downloaded' only when still 'downloading'.
	// Returns whether a row was updated (false if cancelled, removed, etc.).
	MarkDownloadedIfDownloading(ctx context.Context, requestID uint) (bool, error)
	// MarkFailedIfInFlight sets status to 'failed' when still pending, queued, or downloading.
	MarkFailedIfInFlight(ctx context.Context, requestID uint) (bool, error)
	// MarkRemovedIfRemoving sets status to 'removed' when still removing.
	MarkRemovedIfRemoving(ctx context.Context, requestID uint) (bool, error)
	// MarkFailedIfRemoving sets status to 'failed' when still removing.
	MarkFailedIfRemoving(ctx context.Context, requestID uint) (bool, error)
	// MarkQueuedIfPending moves pending → queued when forwarding to download queue (R7).
	MarkQueuedIfPending(ctx context.Context, requestID uint) (bool, error)
	// MarkRemovingIfPending moves pending → removing when forwarding to remove queue (R7).
	MarkRemovingIfPending(ctx context.Context, requestID uint) (bool, error)
	List(ctx context.Context, page, pageSize int) ([]Request, int64, error)
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]Request, int64, error)
	FindByID(ctx context.Context, id uint) (*Request, error)

	// UpdateMediaID sets the media_id column on a request. The download worker
	// calls this immediately after creating the media row so a later remove
	// can look up the original download request by media_id.
	UpdateMediaID(ctx context.Context, requestID uint, mediaID uint) error

	// CancelDownloadsByMediaID flips every download request still in an active
	// state (pending, queued, or downloading) for the given media_id to 'cancelled'.
	// Called at the start of remove processing so the completion watcher cannot
	// mark the request 'downloaded' while files are being deleted.
	// Returns the number of rows updated.
	CancelDownloadsByMediaID(ctx context.Context, mediaID uint) (int64, error)

	// FindActiveMovieDownload returns the most recent in-flight movie_download
	// request matching (imdb_id, tmdb_id, quality), or (nil, nil) if none.
	// "In flight" means status in ('pending', 'queued', 'downloading') — before the
	// media row is finalized. Used for request-level dedup so two near-
	// simultaneous POSTs don't both add a torrent to qBittorrent.
	FindActiveMovieDownload(ctx context.Context, imdbID, tmdbID, quality string) (*Request, error)

	// FindActiveShowDownload returns the most recent in-flight show_download
	// request matching the show identifier and scope. season/episode 0 means
	// "unspecified" and is matched as-is (a full-show request stores 0/0).
	FindActiveShowDownload(ctx context.Context, imdbID, tvdbID, quality string, season, episode int) (*Request, error)

	// FindActiveRemoveByMediaID returns the most recent in-flight remove
	// request (pending or already forwarded to remove queue) for the given
	// media_id and type. Used for request-level dedup.
	FindActiveRemoveByMediaID(ctx context.Context, mediaID uint, requestType string) (*Request, error)

	// ListDownloading returns download requests still in the 'downloading' state
	// with a linked media row (ready for hardlink/torrent completion checks).
	ListDownloading(ctx context.Context) ([]Request, error)

	// ListOrphanedPending returns pending requests older than minAge with no queue job (R2 reconciler).
	ListOrphanedPending(ctx context.Context, minAge time.Duration) ([]Request, error)
	// ListStuckRemoving returns removing requests with no active remove queue job (R2 reconciler).
	ListStuckRemoving(ctx context.Context, minAge time.Duration, hasRemoveJob func(ctx context.Context, requestEntryID uint) (bool, error)) ([]Request, error)
	// ListStuckQueued returns queued requests with no active download queue job (R2 reconciler).
	ListStuckQueued(ctx context.Context, minAge time.Duration, hasDownloadJob func(ctx context.Context, requestEntryID uint) (bool, error)) ([]Request, error)

	// PurgeTerminalOlderThan deletes terminal request rows older than retention (R9).
	PurgeTerminalOlderThan(ctx context.Context, retention time.Duration) (int64, error)

	// SetTorrentInfoInvalidator wires cache eviction for the torrent details modal:
	// the repository evicts on every status mutation it publishes.
	SetTorrentInfoInvalidator(inv TorrentInfoInvalidator)
}

type repository struct {
	db        *gorm.DB
	publisher sse.Publisher

	torrentInfoInvalidator TorrentInfoInvalidator
}

// NewRepository persists requests. Pass sse.NoopPublisher{} when SSE is disabled.
func NewRepository(db *gorm.DB, publisher sse.Publisher) Repository {
	if publisher == nil {
		publisher = sse.NoopPublisher{}
	}
	return &repository{db: db, publisher: publisher}
}

func (r *repository) SetTorrentInfoInvalidator(inv TorrentInfoInvalidator) {
	r.torrentInfoInvalidator = inv
}

func (r *repository) Create(ctx context.Context, req *Request) error {
	if err := r.db.WithContext(ctx).Create(req).Error; err != nil {
		return err
	}
	// Re-read so JSON timestamps and DB defaults are populated for SSE consumers.
	if row, err := r.FindByID(ctx, req.ID); err == nil {
		r.publisher.PublishRequestCreated(ctx, ToSSEPayload(row))
	}
	return nil
}

func (r *repository) UpdateStatus(ctx context.Context, requestID uint, status string) error {
	if err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("id = ?", requestID).
		Update("status", status).Error; err != nil {
		return err
	}
	r.notifyRequestStatus(ctx, requestID)
	return nil
}

func (r *repository) MarkDownloadingIfQueued(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, []string{StatusQueued}, StatusDownloading)
}

func (r *repository) MarkDownloadedIfDownloading(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, []string{StatusDownloading}, StatusDownloaded)
}

func (r *repository) MarkFailedIfInFlight(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, activeDownloadStatuses, StatusFailed)
}

func (r *repository) MarkRemovedIfRemoving(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, []string{StatusRemoving}, StatusRemoved)
}

func (r *repository) MarkFailedIfRemoving(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, []string{StatusRemoving}, StatusFailed)
}

func (r *repository) MarkQueuedIfPending(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, []string{StatusPending}, StatusQueued)
}

func (r *repository) MarkRemovingIfPending(ctx context.Context, requestID uint) (bool, error) {
	return r.updateStatusWhen(ctx, requestID, []string{StatusPending}, StatusRemoving)
}

func (r *repository) updateStatusWhen(ctx context.Context, requestID uint, from []string, to string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("id = ?", requestID).
		Where("status IN ?", from).
		Update("status", to)
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected > 0 {
		r.notifyRequestStatus(ctx, requestID)
	}
	return res.RowsAffected > 0, nil
}

func (r *repository) UpdateMediaID(ctx context.Context, requestID uint, mediaID uint) error {
	return r.db.WithContext(ctx).
		Model(&Request{}).
		Where("id = ?", requestID).
		Update("media_id", mediaID).Error
}

func (r *repository) CancelDownloadsByMediaID(ctx context.Context, mediaID uint) (int64, error) {
	if mediaID == 0 {
		return 0, nil
	}

	// Capture IDs first so we only emit events for rows this call actually changed.
	var ids []uint
	if err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("media_id = ?", mediaID).
		Where("type IN ?", downloadTypes).
		Where("status IN ?", cancellableDownloadStatusesForRemove).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("id IN ?", ids).
		Update("status", StatusCancelled)
	if res.Error != nil {
		return res.RowsAffected, res.Error
	}
	for _, id := range ids {
		r.notifyRequestStatus(ctx, id)
	}
	return res.RowsAffected, nil
}

// notifyRequestStatus loads the row and emits request.status_changed.
func (r *repository) notifyRequestStatus(ctx context.Context, requestID uint) {
	if r.torrentInfoInvalidator != nil {
		r.torrentInfoInvalidator.InvalidateTorrentInfo(requestID)
	}
	row, err := r.FindByID(ctx, requestID)
	if err != nil {
		return
	}
	r.publisher.PublishRequestStatusChanged(ctx, ToSSEPayload(row))
}

func (r *repository) List(ctx context.Context, page, pageSize int) ([]Request, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&Request{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var rows []Request
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func (r *repository) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]Request, int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&Request{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var rows []Request
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *repository) FindByID(ctx context.Context, id uint) (*Request, error) {
	var row Request
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// createIfAbsent runs the dedup-then-create-then-enqueue transaction shared by
// all request entry points. findExisting builds the query that decides whether
// an equivalent in-flight request already exists; when it matches, no row is
// created and the existing row is returned with created=false. When enqueue is
// non-nil it runs inside the same transaction so the request row and its queue
// job commit atomically (R1, R2).
func (r *repository) createIfAbsent(
	ctx context.Context,
	req *Request,
	findExisting func(tx *gorm.DB) *gorm.DB,
	enqueue func(tx *gorm.DB, entry *Request) error,
) (*Request, bool, error) {
	var existing Request
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := findExisting(tx).Order("created_at DESC").First(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(req).Error; err != nil {
			return err
		}
		created = true
		if enqueue != nil {
			return enqueue(tx, req)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if existing.ID != 0 {
		return &existing, false, nil
	}
	if row, err := r.FindByID(ctx, req.ID); err == nil {
		r.publisher.PublishRequestCreated(ctx, ToSSEPayload(row))
	}
	return req, created, nil
}

func (r *repository) CreateMovieDownloadIfAbsent(
	ctx context.Context,
	req *Request,
	imdbID, tmdbID, quality string,
	enqueue func(tx *gorm.DB, entry *Request) error,
) (*Request, bool, error) {
	if imdbID == "" && tmdbID == "" {
		return nil, false, fmt.Errorf("movie download requires imdb_id or tmdb_id")
	}
	return r.createIfAbsent(ctx, req, func(tx *gorm.DB) *gorm.DB {
		q := tx.Model(&Request{}).
			Where("type = ?", TypeMovieDownload).
			Where("status IN ?", activeDownloadStatuses).
			Where("quality = ?", quality)
		if imdbID != "" {
			return q.Where("imdb_id = ?", imdbID)
		}
		return q.Where("tmdb_id = ?", tmdbID)
	}, enqueue)
}

func (r *repository) CreateShowDownloadIfAbsent(
	ctx context.Context,
	req *Request,
	imdbID, tvdbID, quality string,
	season, episode int,
	enqueue func(tx *gorm.DB, entry *Request) error,
) (*Request, bool, error) {
	if imdbID == "" && tvdbID == "" {
		return nil, false, fmt.Errorf("show download requires imdb_id or tvdb_id")
	}
	return r.createIfAbsent(ctx, req, func(tx *gorm.DB) *gorm.DB {
		q := tx.Model(&Request{}).
			Where("type = ?", TypeShowDownload).
			Where("status IN ?", activeDownloadStatuses).
			Where("quality = ?", quality).
			Where("season = ?", season).
			Where("episode = ?", episode)
		if imdbID != "" {
			return q.Where("imdb_id = ?", imdbID)
		}
		return q.Where("tvdb_id = ?", tvdbID)
	}, enqueue)
}

func (r *repository) CreateRemoveIfAbsent(
	ctx context.Context,
	req *Request,
	mediaID uint,
	requestType string,
	enqueue func(tx *gorm.DB, entry *Request) error,
) (*Request, bool, error) {
	if mediaID == 0 {
		return nil, false, fmt.Errorf("remove requires media_id")
	}
	return r.createIfAbsent(ctx, req, func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&Request{}).
			Where("media_id = ?", mediaID).
			Where("type = ?", requestType).
			Where("status IN ?", activeRemoveStatuses)
	}, enqueue)
}

func (r *repository) FindActiveMovieDownload(ctx context.Context, imdbID, tmdbID, quality string) (*Request, error) {
	if imdbID == "" && tmdbID == "" {
		return nil, nil
	}
	q := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("type = ?", TypeMovieDownload).
		Where("status IN ?", activeDownloadStatuses).
		Where("quality = ?", quality)
	if imdbID != "" {
		q = q.Where("imdb_id = ?", imdbID)
	} else {
		q = q.Where("tmdb_id = ?", tmdbID)
	}

	var row Request
	if err := q.Order("created_at DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) FindActiveShowDownload(ctx context.Context, imdbID, tvdbID, quality string, season, episode int) (*Request, error) {
	if imdbID == "" && tvdbID == "" {
		return nil, nil
	}
	q := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("type = ?", TypeShowDownload).
		Where("status IN ?", activeDownloadStatuses).
		Where("quality = ?", quality).
		Where("season = ?", season).
		Where("episode = ?", episode)
	if imdbID != "" {
		q = q.Where("imdb_id = ?", imdbID)
	} else {
		q = q.Where("tvdb_id = ?", tvdbID)
	}

	var row Request
	if err := q.Order("created_at DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) ListDownloading(ctx context.Context) ([]Request, error) {
	var rows []Request
	err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("type IN ?", downloadTypes).
		Where("status = ?", StatusDownloading).
		Where("media_id > 0").
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) FindActiveRemoveByMediaID(ctx context.Context, mediaID uint, requestType string) (*Request, error) {
	if mediaID == 0 {
		return nil, nil
	}
	var row Request
	err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("media_id = ?", mediaID).
		Where("type = ?", requestType).
		Where("status IN ?", activeRemoveStatuses).
		Order("created_at DESC").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) ListOrphanedPending(ctx context.Context, minAge time.Duration) ([]Request, error) {
	cutoff := time.Now().Add(-minAge)
	var rows []Request
	err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("status = ?", StatusPending).
		Where("created_at < ?", cutoff).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *repository) ListStuckQueued(
	ctx context.Context,
	minAge time.Duration,
	hasDownloadJob func(ctx context.Context, requestEntryID uint) (bool, error),
) ([]Request, error) {
	cutoff := time.Now().Add(-minAge)
	var rows []Request
	if err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("type IN ?", downloadTypes).
		Where("status = ?", StatusQueued).
		Where("updated_at < ?", cutoff).
		Order("updated_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return filterWithoutJob(ctx, rows, hasDownloadJob)
}

func (r *repository) ListStuckRemoving(
	ctx context.Context,
	minAge time.Duration,
	hasRemoveJob func(ctx context.Context, requestEntryID uint) (bool, error),
) ([]Request, error) {
	cutoff := time.Now().Add(-minAge)
	var rows []Request
	if err := r.db.WithContext(ctx).
		Model(&Request{}).
		Where("type IN ?", removeTypes).
		Where("status = ?", StatusRemoving).
		Where("updated_at < ?", cutoff).
		Order("updated_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return filterWithoutJob(ctx, rows, hasRemoveJob)
}

func filterWithoutJob(
	ctx context.Context,
	rows []Request,
	hasJob func(ctx context.Context, requestEntryID uint) (bool, error),
) ([]Request, error) {
	if hasJob == nil {
		return rows, nil
	}
	out := make([]Request, 0, len(rows))
	for i := range rows {
		ok, err := hasJob(ctx, rows[i].ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			out = append(out, rows[i])
		}
	}
	return out, nil
}

func (r *repository) PurgeTerminalOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	res := r.db.WithContext(ctx).
		Unscoped().
		Where("status IN ?", terminalRequestStatuses).
		Where("updated_at < ?", cutoff).
		Delete(&Request{})
	return res.RowsAffected, res.Error
}

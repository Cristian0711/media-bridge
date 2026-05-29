package health

import (
	"context"

	"gorm.io/gorm"
)

// inFlightMediaIDs returns media rows with active download, hardlink, or remove work.
func inFlightMediaIDs(ctx context.Context, db *gorm.DB) (map[uint]struct{}, error) {
	out := make(map[uint]struct{})

	var fromRequests []uint
	if err := db.WithContext(ctx).
		Table("requests").
		Where("media_id > 0").
		Where("status IN ?", []string{"pending", "queued", "downloading", "removing"}).
		Distinct().
		Pluck("media_id", &fromRequests).Error; err != nil {
		return nil, err
	}
	for _, id := range fromRequests {
		out[id] = struct{}{}
	}

	var fromQueue []uint
	queueSQL := `
		SELECT DISTINCT (payload->>'media_id')::bigint AS media_id
		FROM processing_queue
		WHERE queue_name IN ('hardlink_processing_queue', 'remove_processing_queue', 'download_processing_queue')
		  AND status IN ('pending', 'processing')
		  AND (payload->>'media_id') IS NOT NULL
		  AND (payload->>'media_id')::bigint > 0
	`
	if err := db.WithContext(ctx).Raw(queueSQL).Scan(&fromQueue).Error; err != nil {
		return nil, err
	}
	for _, id := range fromQueue {
		out[id] = struct{}{}
	}
	return out, nil
}

// hasInFlightDownloadRequests is true while a download may exist in qBit before media is linked.
func hasInFlightDownloadRequests(ctx context.Context, db *gorm.DB) (bool, error) {
	var n int64
	err := db.WithContext(ctx).
		Table("requests").
		Where("type IN ?", []string{"movie_download", "show_download"}).
		Where("status IN ?", []string{"pending", "queued", "downloading"}).
		Count(&n).Error
	return n > 0, err
}

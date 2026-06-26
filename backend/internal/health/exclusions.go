package health

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/pipeline"
	"gorm.io/gorm"
)

// inFlightStatuses are request statuses that mean a media row still has active
// download/remove work and must be excluded from filesystem audits.
var inFlightStatuses = []string{
	pipeline.StatusPending,
	pipeline.StatusQueued,
	pipeline.StatusDownloading,
	pipeline.StatusRemoving,
}

type PathExclusions struct {
	Prefixes      []string
	RemovingDest  []string
	InFlightMedia int
	ByReason      map[string]int
}

func collectExclusions(ctx context.Context, db *gorm.DB, mediaSvc media.Service) (PathExclusions, error) {
	out := PathExclusions{ByReason: make(map[string]int)}
	seen := make(map[string]struct{})

	add := func(prefix, reason string) {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			return
		}
		prefix = filepath.Clean(prefix)
		if _, ok := seen[prefix]; ok {
			return
		}
		seen[prefix] = struct{}{}
		out.Prefixes = append(out.Prefixes, prefix)
		out.ByReason[reason]++
	}

	var mediaIDs []uint
	if err := db.WithContext(ctx).
		Table("requests").
		Where("media_id > 0").
		Where("status IN ?", inFlightStatuses).
		Distinct().
		Pluck("media_id", &mediaIDs).Error; err != nil {
		return out, err
	}

	var removingIDs []uint
	if err := db.WithContext(ctx).
		Table("requests").
		Where("media_id > 0").
		Where("status = ?", pipeline.StatusRemoving).
		Distinct().
		Pluck("media_id", &removingIDs).Error; err != nil {
		return out, err
	}
	removingSet := make(map[uint]struct{}, len(removingIDs))
	for _, id := range removingIDs {
		removingSet[id] = struct{}{}
	}

	var queueMediaIDs []uint
	queueSQL := `
		SELECT DISTINCT (payload->>'media_id')::bigint AS media_id
		FROM processing_queue
		WHERE queue_name IN ?
		  AND status IN ('pending', 'processing')
		  AND (payload->>'media_id') IS NOT NULL
		  AND (payload->>'media_id')::bigint > 0
	`
	if err := db.WithContext(ctx).Raw(queueSQL, pipeline.MediaQueues).Scan(&queueMediaIDs).Error; err != nil {
		return out, err
	}

	idSet := make(map[uint]struct{}, len(mediaIDs)+len(queueMediaIDs))
	for _, id := range mediaIDs {
		idSet[id] = struct{}{}
	}
	for _, id := range queueMediaIDs {
		idSet[id] = struct{}{}
	}
	out.InFlightMedia = len(idSet)

	for id := range idSet {
		row, err := mediaSvc.GetMediaByID(ctx, id)
		if err != nil {
			continue
		}
		savePath, destPath := mediaPaths(row)
		add(savePath, "in_flight_media")
		add(destPath, "in_flight_media")
		if _, removing := removingSet[id]; removing {
			if destPath != "" {
				out.RemovingDest = append(out.RemovingDest, destPath)
			}
		}
	}
	return out, nil
}

// mediaPaths returns qBittorrent save path and library destination for exclusions.
func mediaPaths(row *media.Media) (savePath, destPath string) {
	if row == nil {
		return "", ""
	}
	destPath = filepath.Clean(row.Path)
	switch row.Type {
	case media.MediaTypeMovie:
		if row.Movie != nil && row.Movie.SavePath != nil {
			savePath = filepath.Clean(*row.Movie.SavePath)
		}
	case media.MediaTypeShowFull, media.MediaTypeShowSeason, media.MediaTypeShowEpisode:
		if row.ShowEntry != nil && row.ShowEntry.SavePath != nil {
			savePath = filepath.Clean(*row.ShowEntry.SavePath)
		}
	}
	return savePath, destPath
}

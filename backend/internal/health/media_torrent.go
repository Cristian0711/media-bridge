package health

import (
	"context"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"gorm.io/gorm"
)

const maxMediaTorrentIssues = 40

type mediaTorrentIssue struct {
	Kind     string `json:"kind"`
	MediaID  uint   `json:"media_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Hash     string `json:"hash,omitempty"`
	Torrent  string `json:"torrent_name,omitempty"`
	Message  string `json:"message"`
}

type mediaTorrentRow struct {
	ID   uint   `gorm:"column:id"`
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
	Hash string `gorm:"column:torrent_hash"`
}

func (s *Service) checkMediaTorrentRegistry(
	ctx context.Context,
	torrentByHash map[string]qbittorrent.Torrent,
) Check {
	start := time.Now()
	check := Check{
		ID: "media_torrent_registry", Name: "Media ↔ torrent registry",
	}

	if torrentByHash == nil {
		check.Status = CheckFail
		check.Message = "skipped: qBittorrent torrent list unavailable"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	inFlight, err := inFlightMediaIDs(ctx, s.db)
	if err != nil {
		check.Status = CheckFail
		check.Message = err.Error()
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	downloadsInFlight, err := hasInFlightDownloadRequests(ctx, s.db)
	if err != nil {
		check.Status = CheckFail
		check.Message = err.Error()
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	rows, err := loadMediaTorrentRows(ctx, s.db)
	if err != nil {
		check.Status = CheckFail
		check.Message = err.Error()
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	hashToMedia := make(map[string][]mediaTorrentRow)
	var mediaIssues []mediaTorrentIssue

	for _, row := range rows {
		hash := qbittorrent.NormalizeHash(row.Hash)
		_, inFlightRow := inFlight[row.ID]

		if hash == "" {
			if !inFlightRow {
				mediaIssues = appendMediaTorrentIssue(mediaIssues, mediaTorrentIssue{
					Kind: "missing_hash", MediaID: row.ID, Name: row.Name,
					Message: "media row has no torrent_hash",
				})
			}
			continue
		}

		hashToMedia[hash] = append(hashToMedia[hash], row)

		if inFlightRow {
			continue
		}
		if _, ok := torrentByHash[hash]; !ok {
			mediaIssues = appendMediaTorrentIssue(mediaIssues, mediaTorrentIssue{
				Kind: "torrent_missing", MediaID: row.ID, Name: row.Name, Hash: hash,
				Message: "torrent_hash not found in qBittorrent (plexmedia)",
			})
		}
	}

	var orphanIssues []mediaTorrentIssue
	for hash, t := range torrentByHash {
		if len(hashToMedia[hash]) > 0 {
			continue
		}
		if downloadsInFlight {
			continue
		}
		orphanIssues = appendMediaTorrentIssue(orphanIssues, mediaTorrentIssue{
			Kind: "orphan_torrent", Hash: hash, Torrent: t.Name,
			Message: "plexmedia torrent is not referenced by any media row",
		})
	}

	totalIssues := len(mediaIssues) + len(orphanIssues)
	check.DurationMS = time.Since(start).Milliseconds()
	check.Details = map[string]any{
		"media_count":              len(rows),
		"torrent_count":            len(torrentByHash),
		"in_flight_media":          len(inFlight),
		"downloads_in_flight":      downloadsInFlight,
		"media_issues":             mediaIssues,
		"orphan_torrents":          orphanIssues,
		"media_issue_count":        countByKind(mediaIssues, ""),
		"orphan_torrent_count":     len(orphanIssues),
		"missing_hash_count":       countByKind(mediaIssues, "missing_hash"),
		"torrent_missing_count":    countByKind(mediaIssues, "torrent_missing"),
	}

	switch {
	case totalIssues == 0:
		check.Status = CheckOK
		check.Message = fmt.Sprintf(
			"%d media rows and %d plexmedia torrents are in sync",
			len(rows), len(torrentByHash),
		)
	case downloadsInFlight && len(orphanIssues) > 0 && len(mediaIssues) == 0:
		check.Status = CheckWarn
		check.Message = fmt.Sprintf(
			"%d media issue(s), %d orphan torrent(s) (downloads in flight — orphans may be transient)",
			len(mediaIssues), len(orphanIssues),
		)
	default:
		check.Status = CheckFail
		check.Message = fmt.Sprintf(
			"%d media without matching torrent, %d torrent(s) without media",
			len(mediaIssues), len(orphanIssues),
		)
	}

	return check
}

func loadMediaTorrentRows(ctx context.Context, db *gorm.DB) ([]mediaTorrentRow, error) {
	const q = `
		SELECT m.id, m.name, m.type,
			COALESCE(mov.torrent_hash, se.torrent_hash, '') AS torrent_hash
		FROM media m
		LEFT JOIN movies mov ON m.movie_id = mov.id
		LEFT JOIN show_entries se ON m.show_entry_id = se.id
		WHERE m.deleted_at IS NULL
	`
	var rows []mediaTorrentRow
	err := db.WithContext(ctx).Raw(q).Scan(&rows).Error
	return rows, err
}

func appendMediaTorrentIssue(list []mediaTorrentIssue, issue mediaTorrentIssue) []mediaTorrentIssue {
	if len(list) >= maxMediaTorrentIssues {
		return list
	}
	return append(list, issue)
}

func countByKind(issues []mediaTorrentIssue, kind string) int {
	n := 0
	for _, i := range issues {
		if kind == "" || i.Kind == kind {
			n++
		}
	}
	return n
}

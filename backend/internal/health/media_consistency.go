package health

import (
	"context"
	"fmt"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"gorm.io/gorm"
)

const maxMediaConsistencyIssues = 40

// MediaEntryIssue describes a single media ↔ catalog inconsistency.
type MediaEntryIssue struct {
	Kind        string `json:"kind"`
	MediaID     uint   `json:"media_id,omitempty"`
	MovieID     uint   `json:"movie_id,omitempty"`
	ShowEntryID uint   `json:"show_entry_id,omitempty"`
	Type        string `json:"type,omitempty"`
	Message     string `json:"message"`
}

// MediaEntryRow is the minimal live-media projection used for correlation.
type MediaEntryRow struct {
	ID          uint   `gorm:"column:id"`
	Type        string `gorm:"column:type"`
	MovieID     *uint  `gorm:"column:movie_id"`
	ShowEntryID *uint  `gorm:"column:show_entry_id"`
}

// MediaConsistencyResult is the pure outcome of correlating media against the
// movie/show-entry catalog.
type MediaConsistencyResult struct {
	MediaCount        int
	MovieCount        int
	ShowEntryCount    int
	MediaIssues       []MediaEntryIssue // media pointing at a missing/dangling/duplicate/unknown link
	OrphanMovies      []MediaEntryIssue // movie rows not referenced by any live media
	OrphanShowEntries []MediaEntryIssue // live show entries not referenced by any live media
}

// CountMatches reports whether every media maps 1:1 to a catalog row, i.e.
// media == movies + show entries.
func (r MediaConsistencyResult) CountMatches() bool {
	return r.MediaCount == r.MovieCount+r.ShowEntryCount
}

// TotalIssues is the number of media-side problems plus orphans on either side.
func (r MediaConsistencyResult) TotalIssues() int {
	return len(r.MediaIssues) + len(r.OrphanMovies) + len(r.OrphanShowEntries)
}

func (s *Service) checkMediaConsistency(ctx context.Context) Check {
	start := time.Now()
	check := Check{ID: "media_consistency", Name: "Media ↔ catalog consistency"}

	mediaRows, err := loadMediaEntryRows(ctx, s.db)
	if err != nil {
		return failedCheck(check, start, err)
	}
	movieIDs, err := loadIDSet(ctx, s.db, "SELECT id FROM movies")
	if err != nil {
		return failedCheck(check, start, err)
	}
	showEntryIDs, err := loadIDSet(ctx, s.db, "SELECT id FROM show_entries WHERE deleted_at IS NULL")
	if err != nil {
		return failedCheck(check, start, err)
	}
	downloadsInFlight, err := hasInFlightDownloadRequests(ctx, s.db)
	if err != nil {
		return failedCheck(check, start, err)
	}

	res := CorrelateMediaEntries(mediaRows, movieIDs, showEntryIDs)

	check.DurationMS = time.Since(start).Milliseconds()
	check.Details = map[string]any{
		"media_count":         res.MediaCount,
		"movie_count":         res.MovieCount,
		"show_entry_count":    res.ShowEntryCount,
		"expected_media":      res.MovieCount + res.ShowEntryCount,
		"count_matches":       res.CountMatches(),
		"downloads_in_flight": downloadsInFlight,
		"media_issues":        res.MediaIssues,
		"orphan_movies":       res.OrphanMovies,
		"orphan_show_entries": res.OrphanShowEntries,
	}

	onlyOrphans := len(res.MediaIssues) == 0 && (len(res.OrphanMovies) > 0 || len(res.OrphanShowEntries) > 0)

	switch {
	case res.TotalIssues() == 0 && res.CountMatches():
		check.Status = CheckOK
		check.Message = fmt.Sprintf(
			"%d media = %d movies + %d show entries; all links valid",
			res.MediaCount, res.MovieCount, res.ShowEntryCount,
		)
	case downloadsInFlight && onlyOrphans:
		// A movie/show-entry can briefly exist before its media row is linked
		// during an active download; treat as transient rather than a hard fail.
		check.Status = CheckWarn
		check.Message = fmt.Sprintf(
			"%d orphan catalog row(s) while downloads are in flight (may be transient)",
			len(res.OrphanMovies)+len(res.OrphanShowEntries),
		)
	default:
		check.Status = CheckFail
		check.Message = fmt.Sprintf(
			"%d media without a valid entry, %d orphan movie(s), %d orphan show entry(ies); media=%d expected=%d",
			len(res.MediaIssues), len(res.OrphanMovies), len(res.OrphanShowEntries),
			res.MediaCount, res.MovieCount+res.ShowEntryCount,
		)
	}

	return check
}

// CorrelateMediaEntries is the pure core of the consistency check. It verifies
// that every live media row points at exactly one existing catalog row matching
// its type, and that every catalog row (movies, live show entries) is referenced
// by exactly one live media row.
func CorrelateMediaEntries(
	mediaRows []MediaEntryRow,
	movieIDs map[uint]struct{},
	showEntryIDs map[uint]struct{},
) MediaConsistencyResult {
	res := MediaConsistencyResult{
		MediaCount:     len(mediaRows),
		MovieCount:     len(movieIDs),
		ShowEntryCount: len(showEntryIDs),
	}

	refMovies := make(map[uint]uint, len(mediaRows))      // movieID -> first mediaID
	refShowEntries := make(map[uint]uint, len(mediaRows)) // showEntryID -> first mediaID

	for _, m := range mediaRows {
		switch media.MediaType(m.Type) {
		case media.MediaTypeMovie:
			validateLink(&res, m, m.MovieID, movieIDs, refMovies, true)
		case media.MediaTypeShowFull, media.MediaTypeShowSeason, media.MediaTypeShowEpisode:
			validateLink(&res, m, m.ShowEntryID, showEntryIDs, refShowEntries, false)
		default:
			res.MediaIssues = appendMediaEntryIssue(res.MediaIssues, MediaEntryIssue{
				Kind: "unknown_type", MediaID: m.ID, Type: m.Type,
				Message: "media row has an unrecognized type",
			})
		}
	}

	for id := range movieIDs {
		if _, ok := refMovies[id]; !ok {
			res.OrphanMovies = appendMediaEntryIssue(res.OrphanMovies, MediaEntryIssue{
				Kind: "orphan_movie", MovieID: id,
				Message: "movie is not referenced by any live media row",
			})
		}
	}
	for id := range showEntryIDs {
		if _, ok := refShowEntries[id]; !ok {
			res.OrphanShowEntries = appendMediaEntryIssue(res.OrphanShowEntries, MediaEntryIssue{
				Kind: "orphan_show_entry", ShowEntryID: id,
				Message: "show entry is not referenced by any live media row",
			})
		}
	}

	return res
}

// validateLink checks a single media row's foreign key against the catalog,
// recording missing/dangling/duplicate issues and tracking references for the
// orphan pass. isMovie selects which fields/messages to use.
func validateLink(
	res *MediaConsistencyResult,
	m MediaEntryRow,
	linkID *uint,
	catalog map[uint]struct{},
	refs map[uint]uint,
	isMovie bool,
) {
	noun := "show_entry"
	if isMovie {
		noun = "movie"
	}

	if linkID == nil {
		res.MediaIssues = appendMediaEntryIssue(res.MediaIssues, MediaEntryIssue{
			Kind: "missing_link", MediaID: m.ID, Type: m.Type,
			Message: fmt.Sprintf("media of type %q has no %s_id", m.Type, noun),
		})
		return
	}

	issue := MediaEntryIssue{MediaID: m.ID, Type: m.Type}
	if isMovie {
		issue.MovieID = *linkID
	} else {
		issue.ShowEntryID = *linkID
	}

	if _, ok := catalog[*linkID]; !ok {
		issue.Kind = "dangling_" + noun
		issue.Message = fmt.Sprintf("media references %s %d which does not exist", noun, *linkID)
		res.MediaIssues = appendMediaEntryIssue(res.MediaIssues, issue)
		return
	}

	if first, dup := refs[*linkID]; dup {
		issue.Kind = "shared_" + noun
		issue.Message = fmt.Sprintf("%s %d is also referenced by media %d", noun, *linkID, first)
		res.MediaIssues = appendMediaEntryIssue(res.MediaIssues, issue)
		return
	}
	refs[*linkID] = m.ID
}

func loadMediaEntryRows(ctx context.Context, db *gorm.DB) ([]MediaEntryRow, error) {
	const q = `
		SELECT id, type, movie_id, show_entry_id
		FROM media
		WHERE deleted_at IS NULL
	`
	var rows []MediaEntryRow
	err := db.WithContext(ctx).Raw(q).Scan(&rows).Error
	return rows, err
}

func loadIDSet(ctx context.Context, db *gorm.DB, query string) (map[uint]struct{}, error) {
	var ids []uint
	if err := db.WithContext(ctx).Raw(query).Scan(&ids).Error; err != nil {
		return nil, err
	}
	set := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

func appendMediaEntryIssue(list []MediaEntryIssue, issue MediaEntryIssue) []MediaEntryIssue {
	if len(list) >= maxMediaConsistencyIssues {
		return list
	}
	return append(list, issue)
}

func failedCheck(check Check, start time.Time, err error) Check {
	check.Status = CheckFail
	check.Message = err.Error()
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

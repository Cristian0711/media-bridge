package health_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/health"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
)

func uptr(v uint) *uint { return &v }

func idSet(ids ...uint) map[uint]struct{} {
	set := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func TestCorrelateMediaEntries_AllConsistent(t *testing.T) {
	t.Parallel()

	rows := []health.MediaEntryRow{
		{ID: 1, Type: string(media.MediaTypeMovie), MovieID: uptr(10)},
		{ID: 2, Type: string(media.MediaTypeShowFull), ShowEntryID: uptr(20)},
		{ID: 3, Type: string(media.MediaTypeShowEpisode), ShowEntryID: uptr(21)},
	}
	res := health.CorrelateMediaEntries(rows, idSet(10), idSet(20, 21))

	if res.TotalIssues() != 0 {
		t.Fatalf("expected no issues, got %d (%+v)", res.TotalIssues(), res)
	}
	if !res.CountMatches() {
		t.Fatalf("expected counts to match: media=%d movies=%d showEntries=%d",
			res.MediaCount, res.MovieCount, res.ShowEntryCount)
	}
	if res.MediaCount != 3 || res.MovieCount != 1 || res.ShowEntryCount != 2 {
		t.Fatalf("unexpected counts: %+v", res)
	}
}

func TestCorrelateMediaEntries_MissingLink(t *testing.T) {
	t.Parallel()

	rows := []health.MediaEntryRow{
		{ID: 1, Type: string(media.MediaTypeMovie), MovieID: nil},
		{ID: 2, Type: string(media.MediaTypeShowSeason), ShowEntryID: nil},
	}
	res := health.CorrelateMediaEntries(rows, idSet(), idSet())

	if len(res.MediaIssues) != 2 {
		t.Fatalf("expected 2 media issues, got %d", len(res.MediaIssues))
	}
	for _, iss := range res.MediaIssues {
		if iss.Kind != "missing_link" {
			t.Fatalf("expected missing_link, got %q", iss.Kind)
		}
	}
}

func TestCorrelateMediaEntries_DanglingLinks(t *testing.T) {
	t.Parallel()

	rows := []health.MediaEntryRow{
		{ID: 1, Type: string(media.MediaTypeMovie), MovieID: uptr(99)},
		{ID: 2, Type: string(media.MediaTypeShowFull), ShowEntryID: uptr(88)},
	}
	// catalog is empty -> both links dangle
	res := health.CorrelateMediaEntries(rows, idSet(), idSet())

	kinds := map[string]int{}
	for _, iss := range res.MediaIssues {
		kinds[iss.Kind]++
	}
	if kinds["dangling_movie"] != 1 || kinds["dangling_show_entry"] != 1 {
		t.Fatalf("expected one dangling_movie and one dangling_show_entry, got %+v", kinds)
	}
}

func TestCorrelateMediaEntries_Orphans(t *testing.T) {
	t.Parallel()

	rows := []health.MediaEntryRow{
		{ID: 1, Type: string(media.MediaTypeMovie), MovieID: uptr(10)},
	}
	// movie 11 and show entry 20 exist but no media references them
	res := health.CorrelateMediaEntries(rows, idSet(10, 11), idSet(20))

	if len(res.OrphanMovies) != 1 || res.OrphanMovies[0].MovieID != 11 {
		t.Fatalf("expected orphan movie 11, got %+v", res.OrphanMovies)
	}
	if len(res.OrphanShowEntries) != 1 || res.OrphanShowEntries[0].ShowEntryID != 20 {
		t.Fatalf("expected orphan show entry 20, got %+v", res.OrphanShowEntries)
	}
	if res.CountMatches() {
		t.Fatal("expected count mismatch with orphans present")
	}
}

func TestCorrelateMediaEntries_SharedReference(t *testing.T) {
	t.Parallel()

	rows := []health.MediaEntryRow{
		{ID: 1, Type: string(media.MediaTypeMovie), MovieID: uptr(10)},
		{ID: 2, Type: string(media.MediaTypeMovie), MovieID: uptr(10)}, // duplicate ref
	}
	res := health.CorrelateMediaEntries(rows, idSet(10), idSet())

	if len(res.MediaIssues) != 1 || res.MediaIssues[0].Kind != "shared_movie" {
		t.Fatalf("expected one shared_movie issue, got %+v", res.MediaIssues)
	}
	// Movie 10 is referenced (by media 1), so it is not an orphan.
	if len(res.OrphanMovies) != 0 {
		t.Fatalf("expected no orphan movies, got %+v", res.OrphanMovies)
	}
}

func TestCorrelateMediaEntries_UnknownType(t *testing.T) {
	t.Parallel()

	rows := []health.MediaEntryRow{
		{ID: 1, Type: "bogus", MovieID: uptr(10)},
	}
	res := health.CorrelateMediaEntries(rows, idSet(10), idSet())

	if len(res.MediaIssues) != 1 || res.MediaIssues[0].Kind != "unknown_type" {
		t.Fatalf("expected one unknown_type issue, got %+v", res.MediaIssues)
	}
	// Unknown-type media doesn't reference movie 10, so it's an orphan.
	if len(res.OrphanMovies) != 1 {
		t.Fatalf("expected movie 10 to be orphan, got %+v", res.OrphanMovies)
	}
}

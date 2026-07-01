package prowlarr_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/indexer/prowlarr"
)

func TestNormalizeIMDB(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":             "",
		"tt0111161":    "tt0111161",
		"111161":       "tt0111161",
		"imdb:111161":  "tt0111161",
		"  tt111161  ": "tt0111161",
		"tt0000000":    "tt0000000", // all-zero digits: n<=0 keeps original
		"not-an-id":    "not-an-id", // non-numeric tail returns input unchanged
		"tt12abc":      "tt12abc",
	}
	for in, want := range cases {
		if got := prowlarr.NormalizeIMDB(in); got != want {
			t.Errorf("NormalizeIMDB(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMovieSearchQuery(t *testing.T) {
	t.Parallel()
	if got, want := prowlarr.MovieSearchQuery("111161"), "{imdbid:tt0111161}"; got != want {
		t.Errorf("MovieSearchQuery = %q, want %q", got, want)
	}
}

func TestTVSearchQuery(t *testing.T) {
	t.Parallel()
	cases := []struct {
		imdb            string
		season, episode int
		want            string
	}{
		{"111161", 0, 0, "{imdbid:tt0111161}"},
		{"111161", 2, 0, "{imdbid:tt0111161}{season:2}"},
		{"111161", 2, 5, "{imdbid:tt0111161}{season:2}{episode:5}"},
		{"111161", 0, 5, "{imdbid:tt0111161}{episode:5}"},
	}
	for _, tc := range cases {
		if got := prowlarr.TVSearchQuery(tc.imdb, tc.season, tc.episode); got != tc.want {
			t.Errorf("TVSearchQuery(%q,%d,%d) = %q, want %q", tc.imdb, tc.season, tc.episode, got, tc.want)
		}
	}
}

func TestMatchIndexerFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		indexer string
		names   []string
		want    bool
	}{
		{"FileList", nil, true},                        // empty filter matches all
		{"FileList", []string{}, true},                 //
		{"FileList", []string{"file"}, true},           // case-insensitive substring
		{"FileList", []string{"FILELIST"}, true},       //
		{"FileList", []string{"rarbg"}, false},         // no match
		{"FileList", []string{"", "  ", "list"}, true}, // blanks skipped, "list" matches
	}
	for _, tc := range cases {
		if got := prowlarr.MatchIndexerFilter(tc.indexer, tc.names); got != tc.want {
			t.Errorf("MatchIndexerFilter(%q,%v) = %v, want %v", tc.indexer, tc.names, got, tc.want)
		}
	}
}

func TestResolveIndexerIDs(t *testing.T) {
	t.Parallel()
	indexers := []prowlarr.Indexer{
		{ID: 1, Name: "FileList", Enable: true},
		{ID: 2, Name: "RARBG", Enabled: true},
		{ID: 3, Name: "Disabled", Enable: false},
	}

	if got := prowlarr.ResolveIndexerIDs(indexers, nil); got != nil {
		t.Errorf("nil names should resolve to nil, got %v", got)
	}

	got := prowlarr.ResolveIndexerIDs(indexers, []string{"file", "rarbg"})
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("by name = %v, want [1 2]", got)
	}

	// Disabled indexers are never resolved by name.
	if got := prowlarr.ResolveIndexerIDs(indexers, []string{"disabled"}); len(got) != 0 {
		t.Errorf("disabled indexer resolved: %v", got)
	}

	// A numeric name is accepted verbatim as an id, and dedup avoids repeats.
	got = prowlarr.ResolveIndexerIDs(indexers, []string{"7", "7", "file"})
	if len(got) != 2 || got[0] != 7 || got[1] != 1 {
		t.Errorf("numeric+dedup = %v, want [7 1]", got)
	}
}

func TestToIndexerItemsFiltersByKind(t *testing.T) {
	t.Parallel()
	movie := prowlarr.Release{
		GUID:       "FileList-957709",
		Title:      "A Movie 2020",
		IMDBID:     111161,
		Categories: []prowlarr.Category{{ID: 2000, Name: "Movies"}},
		Indexer:    " FileList ",
	}
	tv := prowlarr.Release{
		GUID:       "abc",
		Title:      "A Show S01E01",
		Categories: []prowlarr.Category{{ID: 5000, Name: "TV"}},
	}

	movies := prowlarr.ToIndexerItems([]prowlarr.Release{movie, tv}, "", false)
	if len(movies) != 1 || movies[0].Name != "A Movie 2020" {
		t.Fatalf("movie filter = %+v, want only the movie release", movies)
	}
	// releaseID pulls the numeric tail off the GUID; IndexerName is trimmed.
	if movies[0].ID != "957709" {
		t.Errorf("ID = %q, want 957709", movies[0].ID)
	}
	if movies[0].IndexerName != "FileList" {
		t.Errorf("IndexerName = %q, want trimmed FileList", movies[0].IndexerName)
	}
	if movies[0].ImdbID != "tt0111161" {
		t.Errorf("ImdbID = %q, want tt0111161", movies[0].ImdbID)
	}

	shows := prowlarr.ToIndexerItems([]prowlarr.Release{movie, tv}, "", true)
	if len(shows) != 1 || shows[0].Name != "A Show S01E01" {
		t.Fatalf("tv filter = %+v, want only the tv release", shows)
	}
}

func TestToIndexerItemsUsesIMDBFallback(t *testing.T) {
	t.Parallel()
	r := prowlarr.Release{
		GUID:       "x-1",
		Title:      "No IMDB",
		Categories: []prowlarr.Category{{ID: 2000}},
	}
	items := prowlarr.ToIndexerItems([]prowlarr.Release{r}, "111161", false)
	if len(items) != 1 || items[0].ImdbID != "tt0111161" {
		t.Fatalf("fallback imdb not applied: %+v", items)
	}
}

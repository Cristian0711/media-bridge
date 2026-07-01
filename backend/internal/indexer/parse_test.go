package indexer

import "testing"

func TestParseSeasonEpisode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		title        string
		wantSeason   int
		wantEpisode  int
		wantComplete bool
		wantOK       bool
	}{
		{"single episode", "Show.S01E02.1080p", 1, 2, false, true},
		{"season pack", "Show.S01.1080p", 1, 0, true, true},
		{"specials S00", "Show.S00E01.Special.1080p", 0, 1, false, true},
		{"specials season pack", "Show.S00.1080p", 0, 0, true, true},
		{"no season tag", "Show.1080p.WEB", 0, 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, e, complete, ok := parseSeasonEpisode(tc.title)
			if s != tc.wantSeason || e != tc.wantEpisode || complete != tc.wantComplete || ok != tc.wantOK {
				t.Fatalf("parseSeasonEpisode(%q) = (s=%d e=%d complete=%v ok=%v), want (s=%d e=%d complete=%v ok=%v)",
					tc.title, s, e, complete, ok, tc.wantSeason, tc.wantEpisode, tc.wantComplete, tc.wantOK)
			}
		})
	}
}

// TestFilterAndSortShows_SpecialsAreParsed pins that S00 specials land in the
// parsed bucket (filterable) rather than being dumped into "unparsed".
func TestFilterAndSortShows_SpecialsAreParsed(t *testing.T) {
	t.Parallel()
	shows := []Show{
		{Name: "Special", Season: 0, Episode: 1, Parsed: true, Seeders: 5, Quality: "1080p", Freeleech: 1},
		{Name: "Garbage", Season: 0, Parsed: false, Seeders: 5, Quality: "1080p", Freeleech: 1},
	}
	parsed, unparsed := filterAndSortShows(shows, 0, 0, "", freeleechPolicy{})
	if len(parsed) != 1 || parsed[0].Name != "Special" {
		t.Fatalf("expected S00 special in parsed bucket, got %+v", parsed)
	}
	if len(unparsed) != 1 || unparsed[0].Name != "Garbage" {
		t.Fatalf("expected only unparseable in unparsed bucket, got %+v", unparsed)
	}
}

// default policy (empty) => every indexer is freeleech-only.
func TestFilterAndSortShows_DefaultDropsNonFreeleech(t *testing.T) {
	t.Parallel()
	shows := []Show{
		{Name: "Free", Season: 1, Episode: 1, Parsed: true, Seeders: 5, Quality: "1080p", Freeleech: 1, IndexerName: "TorrentLeech"},
		{Name: "Paid", Season: 1, Episode: 1, Parsed: true, Seeders: 5, Quality: "1080p", Freeleech: 0, IndexerName: "TorrentLeech"},
	}
	parsed, _ := filterAndSortShows(shows, 0, 0, "", freeleechPolicy{})
	if len(parsed) != 1 || parsed[0].Name != "Free" {
		t.Fatalf("expected only freeleech show, got %+v", parsed)
	}
}

// an indexer configured freeleech_only=false keeps its non-freeleech results.
func TestFilterAndSortShows_ConfiguredIndexerKeepsNonFreeleech(t *testing.T) {
	t.Parallel()
	policy := freeleechPolicy{byName: map[string]bool{"filelist.io": false}}
	shows := []Show{
		{Name: "FLPaid", Season: 1, Episode: 1, Parsed: true, Seeders: 5, Quality: "1080p", Freeleech: 0, IndexerName: "FileList.io"},
		{Name: "TLPaid", Season: 1, Episode: 1, Parsed: true, Seeders: 5, Quality: "1080p", Freeleech: 0, IndexerName: "TorrentLeech"},
	}
	parsed, _ := filterAndSortShows(shows, 0, 0, "", policy)
	if len(parsed) != 1 || parsed[0].Name != "FLPaid" {
		t.Fatalf("expected non-freeleech configured-indexer show kept, other dropped, got %+v", parsed)
	}
}

func TestFilterAndSortMovies_DefaultDropsNonFreeleech(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Free", Seeders: 5, Quality: "1080p", Freeleech: 1, IndexerName: "TorrentLeech"},
		{Name: "Paid", Seeders: 5, Quality: "1080p", Freeleech: 0, IndexerName: "TorrentLeech"},
	}
	filtered := filterAndSortMovies(movies, "", freeleechPolicy{})
	if len(filtered) != 1 || filtered[0].Name != "Free" {
		t.Fatalf("expected only freeleech movie, got %+v", filtered)
	}
}

func TestFilterAndSortMovies_ConfiguredIndexerKeepsNonFreeleech(t *testing.T) {
	t.Parallel()
	policy := freeleechPolicy{byName: map[string]bool{"filelist.io": false}}
	movies := []Movie{
		{Name: "FLPaid", Seeders: 5, Quality: "1080p", Freeleech: 0, IndexerName: "FileList.io"},
		{Name: "TLPaid", Seeders: 5, Quality: "1080p", Freeleech: 0, IndexerName: "TorrentLeech"},
	}
	filtered := filterAndSortMovies(movies, "", policy)
	if len(filtered) != 1 || filtered[0].Name != "FLPaid" {
		t.Fatalf("expected non-freeleech configured-indexer movie kept, other dropped, got %+v", filtered)
	}
}

func TestFreeleechPolicy_Default(t *testing.T) {
	t.Parallel()
	var p freeleechPolicy
	if !p.freeleechOnly("Anything") {
		t.Fatal("unconfigured indexer should default to freeleech-only")
	}
	p = freeleechPolicy{byName: map[string]bool{"filelist.io": false}}
	if p.freeleechOnly("FileList.io") {
		t.Fatal("configured indexer should honor freeleech_only=false (case-insensitive)")
	}
	if !p.freeleechOnly("TorrentLeech") {
		t.Fatal("indexer absent from config should default to freeleech-only")
	}
}

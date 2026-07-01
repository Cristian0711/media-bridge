package indexer

import "testing"

const gib = int64(1) << 30

func TestReleaseGroup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{"Memento 2000 1080p AMZN WEB-DL DD+5 1 H 264-NiSHKRiY0", "nishkriy0"},
		{"Memento.2000.1080p.AMZN.WEB-DL.DDP5.1.H.264-NiSHKRiY0.mkv", "nishkriy0"},
		{"Memento.2000.BluRay.1080p.DTS-HD.MA.5.1.AVC.HYBRID.REMUX-FraMeSToR.mkv", "framestor"},
		{"Memento 2000 1080p BluRay DD 5 1 x264-BHDStudio", "bhdstudio"},
		// Internal descriptor dash, no real group tag -> empty.
		{"Memento.2000.1080p.JPN.Blu-ray.AVC.DTS-HD.MA.5.1", ""},
		{"Memento 2000 Complete Bluray", ""},
		// Trailing token is a codec, not a group -> rejected.
		{"Memento 2000 1080p BluRay-x264", ""},
		// Stray suffix after a space.
		{"Memento 2000 REMASTERED 1080p BluRay x264-WLM TL", "wlm"},
	}
	for _, tc := range cases {
		if got := releaseGroup(tc.name); got != tc.want {
			t.Errorf("releaseGroup(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseSourceAndResolution(t *testing.T) {
	t.Parallel()
	if parseSource("Memento 2000 1080p AMZN WEB-DL DDP 5 1 H 264-PiRaTeS") != "AMZN" {
		t.Error("expected AMZN")
	}
	if parseSource("Memento 2000 1080p PCOK WEB-DL DDP 5 1 H 264-PiRaTeS") != "PCOK" {
		t.Error("expected PCOK")
	}
	if parseSource("Memento 2000 1080p BluRay x264-EbP") != "" {
		t.Error("expected no source for a plain BluRay")
	}
	if parseResolution("Memento 2000 2160p UHD BluRay-GRP") != "2160" {
		t.Error("expected 2160")
	}
}

// Same release group + size, names formatted differently across two indexers.
func TestCrossSeed_MergesFormattingVariants(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento 2000 1080p AMZN WEB-DL DD+5 1 H 264-NiSHKRiY0", Size: 7*gib + 900*(1<<20), IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento.2000.1080p.AMZN.WEB-DL.DDP5.1.H.264-NiSHKRiY0.mkv", Size: 7*gib + 900*(1<<20), IndexerName: "Blutopia (API)"},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 2 {
			t.Fatalf("expected count 2 for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
}

// cross-seed is permissive when a group can't be parsed: an untagged retail rip
// and a tagged copy of the same disc (same res, no source, same size) match.
func TestCrossSeed_PermissiveMissingGroup(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento.2000.1080p.JPN.Blu-ray.AVC.DTS-HD.MA.5.1", Size: 42*gib + 620*(1<<20), IndexerName: "FileList.io"},
		{Name: "Memento 2000 1080p JPN Blu-ray AVC DTS-HD MA 5.1-EbP", Size: 42*gib + 620*(1<<20), IndexerName: "Blutopia (API)"},
	}
	annotateMovieCrossSeed(movies)
	if movies[0].CrossSeedCount != 2 || movies[1].CrossSeedCount != 2 {
		t.Fatalf("expected untagged+tagged same-disc to match, got %d/%d", movies[0].CrossSeedCount, movies[1].CrossSeedCount)
	}
}

// Different source (AMZN vs PCOK) => different releases.
func TestCrossSeed_DifferentSourceNotMerged(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento 2000 1080p AMZN WEB-DL DDP 5 1 H 264-PiRaTeS", Size: 7*gib + 920*(1<<20), IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 1080p PCOK WEB-DL DDP 5 1 H 264-PiRaTeS", Size: 6*gib + 320*(1<<20), IndexerName: "Blutopia (API)"},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 1 {
			t.Fatalf("expected count 1 (different source) for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
}

// Same group/res/source but sizes outside the fuzzy threshold => not merged.
func TestCrossSeed_FuzzySizeSeparatesEncodes(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento 2000 1080p BluRay x264-EbP", Size: 10 * gib, IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 1080p BluRay x264-EbP", Size: 5 * gib, IndexerName: "Blutopia (API)"},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 1 {
			t.Fatalf("expected count 1 (size out of tolerance), got %d", m.CrossSeedCount)
		}
	}
	// Within 2% -> merged.
	movies = []Movie{
		{Name: "Memento 2000 1080p BluRay x264-EbP", Size: 10 * gib, IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 1080p BluRay x264-EbP", Size: 10*gib + 100*(1<<20), IndexerName: "Blutopia (API)"},
	}
	annotateMovieCrossSeed(movies)
	if movies[0].CrossSeedCount != 2 {
		t.Fatalf("expected count 2 (within tolerance), got %d", movies[0].CrossSeedCount)
	}
}

// Same size but different release groups => not merged.
func TestCrossSeed_DifferentGroupSameSizeNotMerged(t *testing.T) {
	t.Parallel()
	size := 4*gib + 920*(1<<20)
	movies := []Movie{
		{Name: "Memento 2000 1080p BluRay x264-nikt0", Size: size, IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 1080p BluRay x264-OFT", Size: size, IndexerName: "Blutopia (API)"},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 1 {
			t.Fatalf("expected count 1 (different groups) for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
}

// A duplicate listing on an already-counted indexer must not inflate the count.
func TestCrossSeed_DistinctIndexersOnly(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento 2000 1080p BluRay x264-EbP", Size: 10 * gib, IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 Hybrid 1080p BluRay DD 5.1 x264-EbP", Size: 10 * gib, IndexerName: "Blutopia (API)"},
		{Name: "Memento 2000 1080p BluRay x264-EbP", Size: 10 * gib, IndexerName: "TorrentLeech (IMDb)"},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 2 {
			t.Fatalf("expected distinct-indexer count 2 for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
}

// Different seasons of the same group/size must not merge.
func TestCrossSeed_ShowsSeasonDiscriminates(t *testing.T) {
	t.Parallel()
	shows := []Show{
		{Name: "Show.S01.1080p.BluRay.x264-GRP", Size: 20 * gib, Season: 1, IndexerName: "TorrentLeech"},
		{Name: "Show S01 1080p BluRay x264-GRP.mkv", Size: 20 * gib, Season: 1, IndexerName: "FileList"},
		{Name: "Show.S02.1080p.BluRay.x264-GRP", Size: 20 * gib, Season: 2, IndexerName: "TorrentLeech"},
	}
	annotateShowCrossSeed(shows)
	if shows[0].CrossSeedCount != 2 || shows[1].CrossSeedCount != 2 {
		t.Fatalf("expected S01 count 2, got %d/%d", shows[0].CrossSeedCount, shows[1].CrossSeedCount)
	}
	if shows[2].CrossSeedCount != 1 {
		t.Fatalf("expected S02 count 1 (different season), got %d", shows[2].CrossSeedCount)
	}
}

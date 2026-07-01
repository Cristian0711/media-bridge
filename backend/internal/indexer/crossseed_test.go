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
		// Stray trailing suffix after a space.
		{"Memento 2000 REMASTERED 1080p BluRay x264-WLM TL", "wlm"},
	}
	for _, tc := range cases {
		if got := releaseGroup(tc.name); got != tc.want {
			t.Errorf("releaseGroup(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Same group + same size, but names differ in formatting (DD+5 1 vs DDP5.1,
// dots vs spaces, .mkv suffix) and the copies span two indexers -> count 2.
func TestCrossSeed_GroupSizeMergesFormattingVariants(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento 2000 1080p AMZN WEB-DL DD+5 1 H 264-NiSHKRiY0", Size: 7*gib + 900*(1<<20), IndexerName: "TorrentLeech (IMDb)", Freeleech: 0},
		{Name: "Memento.2000.1080p.AMZN.WEB-DL.DDP5.1.H.264-NiSHKRiY0.mkv", Size: 7*gib + 900*(1<<20), IndexerName: "Blutopia (API)", Freeleech: 0},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 2 {
			t.Fatalf("expected cross-seed count 2 for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
}

// Same group, clearly different sizes (AMZN vs PCOK encodes) -> not cross-seeds.
func TestCrossSeed_SameGroupDifferentSizeNotMerged(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "Memento 2000 1080p AMZN WEB-DL DDP 5 1 H 264-PiRaTeS", Size: 7*gib + 920*(1<<20), IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 1080p PCOK WEB-DL DDP 5 1 H 264-PiRaTeS", Size: 6*gib + 320*(1<<20), IndexerName: "TorrentLeech (IMDb)"},
	}
	annotateMovieCrossSeed(movies)
	for _, m := range movies {
		if m.CrossSeedCount != 1 {
			t.Fatalf("expected count 1 (different encodes) for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
}

// Same size but different release groups -> not cross-seeds.
func TestCrossSeed_DifferentGroupSameSizeNotMerged(t *testing.T) {
	t.Parallel()
	size := 4*gib + 920*(1<<20)
	movies := []Movie{
		{Name: "Memento 2000 1080p BluRay x264-nikt0", Size: size, IndexerName: "TorrentLeech (IMDb)"},
		{Name: "Memento 2000 1080p BluRay x264-OFT", Size: size, IndexerName: "TorrentLeech (IMDb)"},
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

func TestCrossSeed_ShowsGroupSize(t *testing.T) {
	t.Parallel()
	shows := []Show{
		{Name: "Show.S01.1080p.BluRay.x264-GRP", Size: 20 * gib, Season: 1, Complete: true, Parsed: true, IndexerName: "TorrentLeech"},
		{Name: "Show S01 1080p BluRay x264-GRP.mkv", Size: 20 * gib, Season: 1, Complete: true, Parsed: true, IndexerName: "FileList"},
		{Name: "Show.S02.1080p.BluRay.x264-GRP", Size: 20 * gib, Season: 2, Complete: true, Parsed: true, IndexerName: "TorrentLeech"},
	}
	annotateShowCrossSeed(shows)
	if shows[0].CrossSeedCount != 2 || shows[1].CrossSeedCount != 2 {
		t.Fatalf("expected S01 count 2, got %d/%d", shows[0].CrossSeedCount, shows[1].CrossSeedCount)
	}
	if shows[2].CrossSeedCount != 1 {
		t.Fatalf("expected S02 count 1 (different season), got %d", shows[2].CrossSeedCount)
	}
}

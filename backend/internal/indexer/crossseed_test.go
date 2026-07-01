package indexer

import "testing"

func TestCrossSeedKey_NormalizesCosmeticDifferences(t *testing.T) {
	t.Parallel()
	a := crossSeedKey("Avatar Fire and Ash 2025 2160p UHD BluRay REMUX TrueHD 7 1 Atmos-CiNEPHiLES")
	b := crossSeedKey("Avatar.Fire.and.Ash.2025.2160p.UHD.Blu-ray.REMUX.TrueHD7.1.Atmos-CiNEPHiLES")
	if a != b {
		t.Fatalf("expected cosmetic variants to share a key:\n a=%q\n b=%q", a, b)
	}
	if a == crossSeedKey("Avatar Fire and Ash 2025 2160p UHD BluRay REMUX TrueHD 7 1 Atmos-SPHD") {
		t.Fatal("different release groups must not collapse to the same key")
	}
}

func TestAnnotateMovieCrossSeed_CountsDistinctIndexers(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		// Same release on two indexers (cross-seedable), one non-freeleech.
		{Name: "Avatar.2025.REMUX-CiNEPHiLES", IndexerName: "TorrentLeech (IMDb)", Freeleech: 1},
		{Name: "Avatar 2025 REMUX-CiNEPHiLES", IndexerName: "FileList.io", Freeleech: 0},
		// A duplicate listing on an already-counted indexer must not inflate.
		{Name: "Avatar 2025 REMUX-CiNEPHiLES", IndexerName: "TorrentLeech (IMDb)", Freeleech: 1},
		// A unique release, single indexer.
		{Name: "Avatar.2025.REMUX-SPHD", IndexerName: "TorrentLeech (IMDb)", Freeleech: 1},
	}
	annotateMovieCrossSeed(movies)

	for _, m := range movies[:3] {
		if m.CrossSeedCount != 2 {
			t.Fatalf("expected cross-seed count 2 for %q, got %d", m.Name, m.CrossSeedCount)
		}
	}
	if movies[3].CrossSeedCount != 1 {
		t.Fatalf("expected cross-seed count 1 for unique release, got %d", movies[3].CrossSeedCount)
	}
}

func TestAnnotateShowCrossSeed_CountsDistinctIndexers(t *testing.T) {
	t.Parallel()
	shows := []Show{
		{Name: "Show.S01.1080p-GRP", IndexerName: "TorrentLeech"},
		{Name: "Show S01 1080p-GRP", IndexerName: "FileList"},
		{Name: "Show.S02.1080p-GRP", IndexerName: "TorrentLeech"},
	}
	annotateShowCrossSeed(shows)
	if shows[0].CrossSeedCount != 2 || shows[1].CrossSeedCount != 2 {
		t.Fatalf("expected S01 count 2, got %d/%d", shows[0].CrossSeedCount, shows[1].CrossSeedCount)
	}
	if shows[2].CrossSeedCount != 1 {
		t.Fatalf("expected S02 count 1, got %d", shows[2].CrossSeedCount)
	}
}

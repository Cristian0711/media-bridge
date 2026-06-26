package indexer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/indexer/prowlarr"
)

func loadProwlarrFixture(t *testing.T) []prowlarr.Release {
	t.Helper()
	path := filepath.Join("..", "..", "..", "prowlar_response.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("prowlar_response.json not available: %v", err)
	}
	var releases []prowlarr.Release
	if err := json.Unmarshal(data, &releases); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return releases
}

func TestToIndexerItems_MapsFixtureMovies(t *testing.T) {
	t.Parallel()
	releases := loadProwlarrFixture(t)
	items := prowlarr.ToIndexerItems(releases, "tt0137523", false)

	if len(items) == 0 {
		t.Fatal("expected movie items from fixture")
	}
	var withFreeleech, withBlutopia bool
	for _, item := range items {
		if item.IndexerName == "" || item.DownloadLink == "" {
			t.Fatalf("incomplete item: %+v", item)
		}
		if item.Freeleech {
			withFreeleech = true
		}
		if item.IndexerName == "Blutopia (API)" {
			withBlutopia = true
			if item.ID == "" {
				t.Fatal("expected id from guid")
			}
		}
	}
	if !withFreeleech {
		t.Fatal("expected at least one freeleech item in fixture")
	}
	if !withBlutopia {
		t.Fatal("expected Blutopia movie in fixture")
	}
}

func TestToIndexerItems_FiltersTV(t *testing.T) {
	t.Parallel()
	releases := loadProwlarrFixture(t)
	movies := prowlarr.ToIndexerItems(releases, "tt0137523", false)
	shows := prowlarr.ToIndexerItems(releases, "tt0137523", true)

	if len(shows) == 0 {
		t.Fatal("expected TV items from fixture")
	}
	for _, m := range movies {
		if m.Name == "Memento.Mori.S03.1080p.AMZN.WEB-DL.DDP5.1.H.264-FLUX" {
			t.Fatal("TV torrent leaked into movie results")
		}
	}
	foundTV := false
	for _, s := range shows {
		if s.Seeders >= 0 && s.IndexerName == "FileList.io" {
			foundTV = true
			break
		}
	}
	if !foundTV {
		t.Fatal("expected FileList TV release in show mapping")
	}
}

func TestNormalizeIMDB(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"tt0137523":   "tt0137523",
		"1375664":     "tt1375664",
		"imdb:209144": "tt0209144",
	}
	for in, want := range cases {
		if got := prowlarr.NormalizeIMDB(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestMatchIndexerFilter(t *testing.T) {
	t.Parallel()
	if !prowlarr.MatchIndexerFilter("FileList.io", []string{"filelist"}) {
		t.Fatal("expected filelist to match FileList.io")
	}
	if prowlarr.MatchIndexerFilter("TorrentLeech", []string{"blutopia"}) {
		t.Fatal("expected no match")
	}
}

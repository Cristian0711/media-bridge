package blutopia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	idx "github.com/Cristian0711/media-bridge/backend/internal/indexer"
)

func TestFilterFreeleechMoviesOnly(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrents/filter" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("imdbId") != "1375666" {
			t.Fatalf("imdbId=%q", r.URL.Query().Get("imdbId"))
		}
		if r.URL.Query().Get("free") != "100" {
			t.Fatalf("free=%q", r.URL.Query().Get("free"))
		}
		_ = json.NewEncoder(w).Encode(filterResponse{
			Data: []torrentResource{
				{
					ID: "1",
					Attributes: torrentAttributes{
						Name:           "Inception 2010 1080p",
						Category:       "Movie",
						CategoryID:     1,
						Size:           1000,
						Seeders:        10,
						Freeleech:      "100%",
						DownloadLink:   "https://blutopia.cc/torrent/download/1.token",
					},
				},
				{
					ID: "2",
					Attributes: torrentAttributes{
						Name:         "Inception S01",
						Category:     "TV",
						CategoryID:   2,
						Size:         500,
						Seeders:      5,
						Freeleech:    "100%",
						DownloadLink: "https://blutopia.cc/torrent/download/2.token",
					},
				},
				{
					ID: "3",
					Attributes: torrentAttributes{
						Name:         "Not free",
						Category:     "Movie",
						CategoryID:   1,
						Freeleech:    "0%",
						DownloadLink: "https://blutopia.cc/torrent/download/3.token",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL})
	items, err := client.FilterFreeleech(context.Background(), "tt1375666")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 freeleech rows, got %d", len(items))
	}

	p := NewProvider(Config{BaseURL: srv.URL}, true)
	movies, err := p.SearchMovies(context.Background(), idx.SearchRequest{ImdbID: "tt1375666"})
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 {
		t.Fatalf("expected 1 movie, got %d", len(movies))
	}
	if !movies[0].Freeleech {
		t.Fatal("expected freeleech")
	}
	if movies[0].ID != "1" {
		t.Fatalf("expected torrent id 1, got %q", movies[0].ID)
	}
}

func TestNormalizeIMDbNumeric(t *testing.T) {
	t.Parallel()
	got, err := normalizeIMDbNumeric("tt1375666")
	if err != nil || got != "1375666" {
		t.Fatalf("got %q err %v", got, err)
	}
}

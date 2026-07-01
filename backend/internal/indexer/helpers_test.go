package indexer

import "testing"

func TestPickBestFreeleechMovie(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "a", Quality: "1080p", Seeders: 10, Freeleech: 0},
		{Name: "b", Quality: "1080p", Seeders: 5, Freeleech: 1},
		{Name: "c", Quality: "1080p", Seeders: 20, Freeleech: 1},
		{Name: "d", Quality: "720p", Seeders: 50, Freeleech: 1},
	}
	got, err := pickBestFreeleechMovie(movies, "1080p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "c" {
		t.Fatalf("expected c (20 seeders), got %s", got.Name)
	}
}

func TestCanonicalQualityFromLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"4K Remux", "4K Remux"},
		{"4k remux", "4K Remux"},
		{"1080p", "1080p"},
		{"1080p BluRay", "1080p BluRay"},
	}
	for _, tc := range cases {
		if got := canonicalQuality(tc.in); got != tc.want {
			t.Errorf("canonicalQuality(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPickBestFreeleechMovie4KRemuxLabel(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "a", Quality: "4K Remux", Seeders: 3, Freeleech: 1},
		{Name: "b", Quality: "4K Remux", Seeders: 10, Freeleech: 1},
	}
	got, err := pickBestFreeleechMovie(movies, "4K Remux")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "b" {
		t.Fatalf("expected b, got %s", got.Name)
	}
}

func TestPickBestFreeleechMovieNoFreeleech(t *testing.T) {
	t.Parallel()
	movies := []Movie{
		{Name: "a", Quality: "1080p", Seeders: 10, Freeleech: 0},
	}
	_, err := pickBestFreeleechMovie(movies, "1080p")
	if err == nil {
		t.Fatal("expected error")
	}
}

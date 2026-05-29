package indexer

import "testing"

func TestParseIDDigitsOnly(t *testing.T) {
	t.Parallel()

	got := parseID("12345")
	if got != 12345 {
		t.Fatalf("expected 12345, got %d", got)
	}
}

func TestParseIDSkipsNonDigits(t *testing.T) {
	t.Parallel()

	got := parseID("ab12cd34")
	if got != 1234 {
		t.Fatalf("expected 1234, got %d", got)
	}
}

func TestParseIDFromDownloadURL(t *testing.T) {
	t.Parallel()

	got := parseID("https://blutopia.cc/torrent/download/127673.d5c080356451401b6d009f9007835eb0")
	if got != 127673 {
		t.Fatalf("expected 127673, got %d", got)
	}
}

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

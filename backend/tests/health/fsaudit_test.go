package health_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/health"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAuditRoot_FlagsSingleLinkAndCountsHardlinked(t *testing.T) {
	root := t.TempDir()
	linkTarget := t.TempDir()

	// Hardlinked file (nlink == 2): library copy + a sibling link elsewhere.
	linked := filepath.Join(root, "Movie (tt1)", "movie.mkv")
	writeFile(t, linked)
	if err := os.Link(linked, filepath.Join(linkTarget, "movie.mkv")); err != nil {
		t.Fatal(err)
	}

	// Single-link file (nlink == 1): should be flagged.
	writeFile(t, filepath.Join(root, "Lonely (tt2)", "lonely.mkv"))

	res := health.AuditRoot(context.Background(), root, "library_movies", health.PathExclusions{})

	if res.FilesScanned != 2 {
		t.Fatalf("FilesScanned = %d, want 2", res.FilesScanned)
	}
	if res.FilesOK != 1 {
		t.Fatalf("FilesOK = %d, want 1", res.FilesOK)
	}
	if res.IssueCount != 1 {
		t.Fatalf("IssueCount = %d, want 1", res.IssueCount)
	}
	if len(res.Issues) != 1 || res.Issues[0].NLink != 1 {
		t.Fatalf("expected one issue with nlink=1, got %+v", res.Issues)
	}
}

// B4 regression: a single-link file under a removing destination must be
// excluded for library zones. The bug compared zone == "library" while callers
// pass "library_movies"/"library_shows", so the exclusion never fired.
func TestAuditRoot_RemovingDestExcludedForLibraryZone(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Removing Title (tt9)")
	writeFile(t, filepath.Join(dest, "video.mkv"))

	ex := health.PathExclusions{RemovingDest: []string{dest}}
	res := health.AuditRoot(context.Background(), root, "library_shows", ex)

	if res.IssueCount != 0 {
		t.Fatalf("IssueCount = %d, want 0 (file is under a removing dest)", res.IssueCount)
	}
	if res.FilesExcluded != 1 {
		t.Fatalf("FilesExcluded = %d, want 1", res.FilesExcluded)
	}
}

// The removing-dest exclusion is library-only: a downloads-zone file in the same
// situation should still be flagged.
func TestAuditRoot_RemovingDestNotAppliedToDownloadsZone(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "release")
	writeFile(t, filepath.Join(dest, "video.mkv"))

	ex := health.PathExclusions{RemovingDest: []string{dest}}
	res := health.AuditRoot(context.Background(), root, "downloads", ex)

	if res.IssueCount != 1 {
		t.Fatalf("IssueCount = %d, want 1 (downloads zone ignores removing-dest)", res.IssueCount)
	}
}

func TestAuditRoot_PrefixExcluded(t *testing.T) {
	root := t.TempDir()
	excludedDir := filepath.Join(root, "in-flight")
	writeFile(t, filepath.Join(excludedDir, "video.mkv"))

	ex := health.PathExclusions{Prefixes: []string{filepath.Clean(excludedDir)}}
	res := health.AuditRoot(context.Background(), root, "library_movies", ex)

	if res.FilesExcluded != 1 {
		t.Fatalf("FilesExcluded = %d, want 1", res.FilesExcluded)
	}
	if res.IssueCount != 0 {
		t.Fatalf("IssueCount = %d, want 0", res.IssueCount)
	}
}

func TestAuditRoot_MissingPathSkipped(t *testing.T) {
	res := health.AuditRoot(context.Background(), filepath.Join(t.TempDir(), "nope"), "downloads", health.PathExclusions{})
	if res.FilesSkipped != -1 {
		t.Fatalf("FilesSkipped = %d, want -1 (path not found sentinel)", res.FilesSkipped)
	}
}

func TestPathHasPrefix(t *testing.T) {
	prefixes := []string{"/mnt/a", "/mnt/b"}
	cases := []struct {
		path string
		want bool
	}{
		{"/mnt/a", true},
		{"/mnt/a/sub/file.mkv", true},
		{"/mnt/b/x", true},
		{"/mnt/abc/file.mkv", false}, // not a path-segment prefix
		{"/mnt/c", false},
	}
	for _, c := range cases {
		if got := health.PathHasPrefix(c.path, prefixes); got != c.want {
			t.Errorf("PathHasPrefix(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestFSResultToCheck(t *testing.T) {
	if got := health.FSResultToCheck("id", "n", health.FSAuditResult{FilesSkipped: -1}); got.Status != health.CheckSkip {
		t.Errorf("not-found should map to skip, got %q", got.Status)
	}
	if got := health.FSResultToCheck("id", "n", health.FSAuditResult{FilesScanned: 5}); got.Status != health.CheckOK {
		t.Errorf("clean scan should map to ok, got %q", got.Status)
	}
	if got := health.FSResultToCheck("id", "n", health.FSAuditResult{IssueCount: 2}); got.Status != health.CheckFail {
		t.Errorf("issues should map to fail, got %q", got.Status)
	}
	if got := health.FSResultToCheck("id", "n", health.FSAuditResult{Truncated: true}); got.Status != health.CheckWarn {
		t.Errorf("truncation without issues should map to warn, got %q", got.Status)
	}
}

func TestAggregateStatus(t *testing.T) {
	cases := []struct {
		name   string
		checks []health.Check
		want   string
	}{
		{"all ok", []health.Check{{Status: health.CheckOK}, {Status: health.CheckOK}}, health.StatusHealthy},
		{"warn", []health.Check{{Status: health.CheckOK}, {Status: health.CheckWarn}}, health.StatusDegraded},
		{"fail dominates warn", []health.Check{{Status: health.CheckWarn}, {Status: health.CheckFail}}, health.StatusUnhealthy},
		{"skip is healthy", []health.Check{{Status: health.CheckSkip}, {Status: health.CheckOK}}, health.StatusHealthy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := health.AggregateStatus(c.checks); got != c.want {
				t.Errorf("AggregateStatus = %q, want %q", got, c.want)
			}
		})
	}
}

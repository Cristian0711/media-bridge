package qbittorrent_test

import (
	"testing"

	qbit "github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

func TestTorrentTransferComplete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		progress float64
		state    string
		want     bool
	}{
		{"complete uploading", 1.0, "uploading", true},
		{"complete stalledUP", 1.0, "stalledUP", true},
		{"complete pausedUP", 1.0, "pausedUP", true},
		{"complete state casing/whitespace", 1.0, "  StalledUP ", true},
		{"full progress still downloading", 1.0, "downloading", false},
		{"full progress moving", 1.0, "moving", false},
		{"full progress checking prefix", 1.0, "checkingUP", false},
		{"full progress checkingResume", 1.0, "checkingResume", false},
		{"low progress complete state", 0.5, "uploading", false},
		{"just below threshold", 0.998, "uploading", false},
		{"at threshold", 0.999, "uploading", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := qbit.TorrentTransferComplete(qbit.Torrent{Progress: tc.progress, State: tc.state})
			if got != tc.want {
				t.Fatalf("TorrentTransferComplete(progress=%v, state=%q) = %v, want %v",
					tc.progress, tc.state, got, tc.want)
			}
		})
	}
}

func TestDownloadComplete(t *testing.T) {
	t.Parallel()

	if qbit.DownloadComplete(0.998) {
		t.Fatal("expected 0.998 to be incomplete")
	}
	if !qbit.DownloadComplete(0.999) {
		t.Fatal("expected 0.999 to be complete")
	}
	if !qbit.DownloadComplete(1.0) {
		t.Fatal("expected 1.0 to be complete")
	}
}

func TestFileDownloadComplete(t *testing.T) {
	t.Parallel()

	if qbit.FileDownloadComplete(0.5) {
		t.Fatal("expected 0.5 to be incomplete")
	}
	if !qbit.FileDownloadComplete(1.0) {
		t.Fatal("expected 1.0 to be complete")
	}
}

func TestFilesComplete(t *testing.T) {
	t.Parallel()

	all := func(string, int64) bool { return true }

	t.Run("empty list is never complete", func(t *testing.T) {
		t.Parallel()
		if qbit.FilesComplete(nil, all) {
			t.Fatal("empty file list should not be complete")
		}
	})

	t.Run("all files complete", func(t *testing.T) {
		t.Parallel()
		files := []qbit.TorrentFile{
			{Name: "a.mkv", Size: 10, Progress: 1.0},
			{Name: "b.mkv", Size: 20, Progress: 1.0},
		}
		if !qbit.FilesComplete(files, nil) {
			t.Fatal("expected complete with nil shouldCount counting all files")
		}
	})

	t.Run("one incomplete file fails", func(t *testing.T) {
		t.Parallel()
		files := []qbit.TorrentFile{
			{Name: "a.mkv", Size: 10, Progress: 1.0},
			{Name: "b.mkv", Size: 20, Progress: 0.4},
		}
		if qbit.FilesComplete(files, all) {
			t.Fatal("expected incomplete when a file has not finished")
		}
	})

	t.Run("shouldCount skips incomplete file", func(t *testing.T) {
		t.Parallel()
		files := []qbit.TorrentFile{
			{Name: "movie.mkv", Size: 10, Progress: 1.0},
			{Name: "sample.nfo", Size: 1, Progress: 0.1},
		}
		onlyVideo := func(name string, _ int64) bool { return name == "movie.mkv" }
		if !qbit.FilesComplete(files, onlyVideo) {
			t.Fatal("expected complete when the only counted file is finished")
		}
	})

	t.Run("filter excluding everything is not complete", func(t *testing.T) {
		t.Parallel()
		files := []qbit.TorrentFile{
			{Name: "a.mkv", Size: 10, Progress: 1.0},
		}
		none := func(string, int64) bool { return false }
		if qbit.FilesComplete(files, none) {
			t.Fatal("expected not complete when no files are counted")
		}
	})
}

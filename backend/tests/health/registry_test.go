package health_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/health"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

func issueKinds(issues []health.MediaTorrentIssue) map[string]int {
	out := map[string]int{}
	for _, i := range issues {
		out[i.Kind]++
	}
	return out
}

func TestCorrelateMediaTorrents_InSync(t *testing.T) {
	rows := []health.MediaTorrentRow{{ID: 1, Name: "A", Hash: "abc"}}
	torrents := map[string]qbittorrent.Torrent{"abc": {Name: "A.torrent"}}

	mediaIssues, orphanIssues := health.CorrelateMediaTorrents(rows, torrents, nil, false)
	if len(mediaIssues) != 0 || len(orphanIssues) != 0 {
		t.Fatalf("expected no issues, got media=%v orphan=%v", mediaIssues, orphanIssues)
	}
}

// Media row hash is stored uppercase but the torrent map is keyed by the
// normalized (lowercase) hash; they must still correlate.
func TestCorrelateMediaTorrents_NormalizesHashBeforeLookup(t *testing.T) {
	rows := []health.MediaTorrentRow{{ID: 1, Name: "A", Hash: "ABCDEF"}}
	torrents := map[string]qbittorrent.Torrent{qbittorrent.NormalizeHash("ABCDEF"): {Name: "A.torrent"}}

	mediaIssues, orphanIssues := health.CorrelateMediaTorrents(rows, torrents, nil, false)
	if len(mediaIssues) != 0 || len(orphanIssues) != 0 {
		t.Fatalf("uppercase media hash should match normalized torrent key, got media=%v orphan=%v", mediaIssues, orphanIssues)
	}
}

func TestCorrelateMediaTorrents_TorrentMissing(t *testing.T) {
	rows := []health.MediaTorrentRow{{ID: 1, Name: "A", Hash: "abc"}}
	mediaIssues, _ := health.CorrelateMediaTorrents(rows, map[string]qbittorrent.Torrent{}, nil, false)
	if issueKinds(mediaIssues)["torrent_missing"] != 1 {
		t.Fatalf("expected one torrent_missing issue, got %v", mediaIssues)
	}
}

// An in-flight media row whose torrent isn't in qBit yet must not be flagged.
func TestCorrelateMediaTorrents_InFlightSuppressesTorrentMissing(t *testing.T) {
	rows := []health.MediaTorrentRow{{ID: 7, Name: "A", Hash: "abc"}}
	inFlight := map[uint]struct{}{7: {}}
	mediaIssues, _ := health.CorrelateMediaTorrents(rows, map[string]qbittorrent.Torrent{}, inFlight, false)
	if len(mediaIssues) != 0 {
		t.Fatalf("in-flight row should suppress torrent_missing, got %v", mediaIssues)
	}
}

func TestCorrelateMediaTorrents_MissingHash(t *testing.T) {
	rows := []health.MediaTorrentRow{{ID: 1, Name: "A", Hash: ""}}
	mediaIssues, _ := health.CorrelateMediaTorrents(rows, map[string]qbittorrent.Torrent{}, nil, false)
	if issueKinds(mediaIssues)["missing_hash"] != 1 {
		t.Fatalf("expected one missing_hash issue, got %v", mediaIssues)
	}

	// in-flight suppresses missing_hash too
	inFlight := map[uint]struct{}{1: {}}
	mediaIssues, _ = health.CorrelateMediaTorrents(rows, map[string]qbittorrent.Torrent{}, inFlight, false)
	if len(mediaIssues) != 0 {
		t.Fatalf("in-flight row should suppress missing_hash, got %v", mediaIssues)
	}
}

func TestCorrelateMediaTorrents_OrphanTorrent(t *testing.T) {
	torrents := map[string]qbittorrent.Torrent{"orphan": {Name: "Ghost.torrent"}}
	_, orphanIssues := health.CorrelateMediaTorrents(nil, torrents, nil, false)
	if issueKinds(orphanIssues)["orphan_torrent"] != 1 {
		t.Fatalf("expected one orphan_torrent issue, got %v", orphanIssues)
	}

	// downloads-in-flight suppresses orphans (they may be transient)
	_, orphanIssues = health.CorrelateMediaTorrents(nil, torrents, nil, true)
	if len(orphanIssues) != 0 {
		t.Fatalf("downloads-in-flight should suppress orphans, got %v", orphanIssues)
	}
}

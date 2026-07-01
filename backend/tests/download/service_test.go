package download_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/download"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
	"github.com/Cristian0711/media-bridge/backend/tests/testhelpers"
)

type stubDownloader struct {
	payload        string
	err            error
	gotIndexerID   string
	gotDownloadURL string
}

func (s *stubDownloader) DownloadTorrent(_ context.Context, indexerID, downloadURL string) (string, error) {
	s.gotIndexerID = indexerID
	s.gotDownloadURL = downloadURL
	return s.payload, s.err
}

func okPayload() string { return base64.StdEncoding.EncodeToString([]byte("torrent-bytes")) }

func TestAdd_UsesConfiguredDownloadsPathAndNormalizesHash(t *testing.T) {
	dl := &stubDownloader{payload: okPayload()}
	var gotSavePath string
	qbit := testhelpers.StubQbit{
		AddFunc: func(_ context.Context, _ []byte, savePath, _ string) (*qbittorrent.AddTorrentResponse, error) {
			gotSavePath = savePath
			return &qbittorrent.AddTorrentResponse{Hash: "ABCDEF123", Size: 4096}, nil
		},
	}
	svc := download.NewService(dl, qbit, "/configured/downloads")

	res, err := svc.Add(context.Background(), download.RequestDetails{
		Type:        "movie_download",
		TorrentURL:  "https://filelist.io/download.php?id=1",
		TorrentName: "Film.torrent",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotSavePath != "/configured/downloads" {
		t.Errorf("save path = %q, want the injected downloads path", gotSavePath)
	}
	if res.TorrentHash == nil || *res.TorrentHash != qbittorrent.NormalizeHash("ABCDEF123") {
		t.Errorf("hash not normalized: got %v", res.TorrentHash)
	}
	if res.SavePath == nil || *res.SavePath != "/configured/downloads" {
		t.Errorf("result save path = %v, want injected downloads path (addResp.Path empty)", res.SavePath)
	}
	if res.SizeBytes == nil || *res.SizeBytes != 4096 {
		t.Errorf("size = %v, want 4096", res.SizeBytes)
	}
}

func TestAdd_RejectsEmptyHash(t *testing.T) {
	qbit := testhelpers.StubQbit{
		AddFunc: func(context.Context, []byte, string, string) (*qbittorrent.AddTorrentResponse, error) {
			return &qbittorrent.AddTorrentResponse{Hash: ""}, nil
		},
	}
	svc := download.NewService(&stubDownloader{payload: okPayload()}, qbit, "/dl")
	if _, err := svc.Add(context.Background(), download.RequestDetails{TorrentURL: "x"}); err == nil {
		t.Fatal("expected error on empty hash")
	}
}

func TestAdd_RejectsNilResponse(t *testing.T) {
	qbit := testhelpers.StubQbit{
		AddFunc: func(context.Context, []byte, string, string) (*qbittorrent.AddTorrentResponse, error) {
			return nil, nil
		},
	}
	svc := download.NewService(&stubDownloader{payload: okPayload()}, qbit, "/dl")
	if _, err := svc.Add(context.Background(), download.RequestDetails{TorrentURL: "x"}); err == nil {
		t.Fatal("expected error on nil response")
	}
}

func TestAdd_RejectsInvalidBase64(t *testing.T) {
	svc := download.NewService(&stubDownloader{payload: "not-base64!!!"}, testhelpers.StubQbit{}, "/dl")
	if _, err := svc.Add(context.Background(), download.RequestDetails{TorrentURL: "x"}); err == nil {
		t.Fatal("expected decode error")
	}
}

// All torrent downloads go through the Prowlarr provider regardless of URL host.
func TestAdd_UsesProwlarrIndexer(t *testing.T) {
	urls := []string{
		"http://127.0.0.1:9696/1/download?apikey=x&link=y&file=z.torrent",
		"https://filelist.io/download.php?id=9",
	}
	for _, u := range urls {
		dl := &stubDownloader{payload: okPayload()}
		qbit := testhelpers.StubQbit{
			AddFunc: func(context.Context, []byte, string, string) (*qbittorrent.AddTorrentResponse, error) {
				return &qbittorrent.AddTorrentResponse{Hash: "h"}, nil
			},
		}
		svc := download.NewService(dl, qbit, "/dl")
		if _, err := svc.Add(context.Background(), download.RequestDetails{TorrentURL: u}); err != nil {
			t.Fatalf("Add(%s): %v", u, err)
		}
		if dl.gotIndexerID != "prowlarr" {
			t.Errorf("url %s -> indexer %q, want prowlarr", u, dl.gotIndexerID)
		}
	}
}

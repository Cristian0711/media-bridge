package source_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	"github.com/Cristian0711/media-bridge/backend/internal/requests/source"
)

// stubRepo embeds requests.Repository so it satisfies the interface without
// implementing every method; only FindByID is overridden (the rest would panic
// if called, which they are not by the sources under test).
type stubRepo struct {
	requests.Repository
	req *requests.Request
	err error
}

func (s stubRepo) FindByID(context.Context, uint) (*requests.Request, error) {
	return s.req, s.err
}

func TestDownloadSourceFindByIDMapsFields(t *testing.T) {
	t.Parallel()
	repo := stubRepo{req: &requests.Request{
		ID: 3, RequestID: "req-3", MediaID: 9, Type: "movie_download",
		Name: "A Movie", IMDBID: "tt1", TMDBID: "t2", TVDBID: "t3",
		Season: 1, Episode: 2, PosterURL: "p", TorrentURL: "u",
		TorrentName: "n", Indexer: "FileList", Quality: "1080p",
		UserID: 4, Username: "alice",
	}}

	got, err := source.NewDownloadSource(repo).FindByID(context.Background(), 3)
	if err != nil {
		t.Fatalf("FindByID error = %v", err)
	}
	if got.RequestEntryID != 3 || got.RequestID != "req-3" || got.MediaID != 9 {
		t.Errorf("id fields wrong: %+v", got)
	}
	if got.Name != "A Movie" || got.Quality != "1080p" || got.Username != "alice" {
		t.Errorf("string fields wrong: %+v", got)
	}
	if got.Season != 1 || got.Episode != 2 {
		t.Errorf("season/episode wrong: %+v", got)
	}
}

func TestDownloadSourcePropagatesError(t *testing.T) {
	t.Parallel()
	repo := stubRepo{err: errors.New("not found")}
	if _, err := source.NewDownloadSource(repo).FindByID(context.Background(), 1); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestRemoveSourceFindByIDMapsFields(t *testing.T) {
	t.Parallel()
	repo := stubRepo{req: &requests.Request{
		ID: 5, RequestID: "req-5", MediaID: 11, Type: "movie_remove",
		UserID: 6, Username: "bob",
	}}

	got, err := source.NewRemoveSource(repo).FindByID(context.Background(), 5)
	if err != nil {
		t.Fatalf("FindByID error = %v", err)
	}
	if got.RequestEntryID != 5 || got.RequestID != "req-5" || got.MediaID != 11 {
		t.Errorf("id fields wrong: %+v", got)
	}
	if got.Type != "movie_remove" || got.Username != "bob" {
		t.Errorf("string fields wrong: %+v", got)
	}
}

func TestRemoveSourcePropagatesError(t *testing.T) {
	t.Parallel()
	repo := stubRepo{err: errors.New("boom")}
	if _, err := source.NewRemoveSource(repo).FindByID(context.Background(), 1); err == nil {
		t.Fatal("expected error to propagate")
	}
}

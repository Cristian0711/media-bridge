package media_test

import (
	"context"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"github.com/Cristian0711/media-bridge/backend/tests/testhelpers"
)

func TestDeleteMovieMediaCascade_RemovesMediaAndMovie(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateMedia(t, db)
	repo := media.NewRepository(db)
	ctx := context.Background()

	movie := &media.Movie{IMDBID: "tt1"}
	mediaRow := &media.Media{
		Type: media.MediaTypeMovie, Name: "Film", Path: "/dl", Indexer: "x", Quality: "1080p",
		UserID: 1, Username: "u",
	}
	if err := repo.CreateMovieWithMedia(ctx, movie, mediaRow); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.DeleteMovieMediaCascade(ctx, mediaRow.ID, movie.ID); err != nil {
		t.Fatalf("cascade: %v", err)
	}

	var mediaCount int64
	if err := db.Model(&media.Media{}).Count(&mediaCount).Error; err != nil {
		t.Fatalf("count media: %v", err)
	}
	var movieCount int64
	if err := db.Model(&media.Movie{}).Count(&movieCount).Error; err != nil {
		t.Fatalf("count movie: %v", err)
	}
	if mediaCount != 0 || movieCount != 0 {
		t.Fatalf("expected empty tables, media=%d movie=%d", mediaCount, movieCount)
	}
}

func TestDeleteMovieMediaCascade_KeepsMovieWhenOtherMediaReferences(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateMedia(t, db)
	repo := media.NewRepository(db)
	ctx := context.Background()

	movie := &media.Movie{IMDBID: "tt-shared"}
	rowA := &media.Media{
		Type: media.MediaTypeMovie, Name: "A", Path: "/a", Indexer: "x", Quality: "1080p",
		UserID: 1, Username: "u",
	}
	if err := repo.CreateMovieWithMedia(ctx, movie, rowA); err != nil {
		t.Fatal(err)
	}
	rowB := &media.Media{
		Type: media.MediaTypeMovie, Name: "B", Path: "/b", Indexer: "x", Quality: "720p",
		UserID: 1, Username: "u", MovieID: &movie.ID,
	}
	if err := db.Create(rowB).Error; err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteMovieMediaCascade(ctx, rowA.ID, movie.ID); err != nil {
		t.Fatal(err)
	}

	var movieCount int64
	if err := db.Model(&media.Movie{}).Count(&movieCount).Error; err != nil {
		t.Fatal(err)
	}
	if movieCount != 1 {
		t.Fatalf("movie row must survive while rowB references it, count=%d", movieCount)
	}
}

func TestDeleteShowMediaCascade_HardDeletesShowEntry(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateMedia(t, db)
	repo := media.NewRepository(db)
	ctx := context.Background()

	show := &media.Show{Name: "Show", IMDBID: "tt2"}
	entry := &media.ShowEntry{}
	mediaRow := &media.Media{
		Type: media.MediaTypeShowEpisode, Name: "Ep", Path: "/dl", Indexer: "x", Quality: "1080p",
		UserID: 1, Username: "u",
	}
	if err := repo.CreateShowEntryWithMedia(ctx, show, entry, mediaRow); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.DeleteShowMediaCascade(ctx, mediaRow.ID, entry.ID); err != nil {
		t.Fatalf("cascade: %v", err)
	}

	var entryCount int64
	if err := db.Unscoped().Model(&media.ShowEntry{}).Count(&entryCount).Error; err != nil {
		t.Fatalf("count entry: %v", err)
	}
	if entryCount != 0 {
		t.Fatalf("expected hard-deleted show_entry, count=%d", entryCount)
	}
}

func TestRemoveFromRequest_CascadeViaService(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateMedia(t, db)
	repo := media.NewRepository(db)
	svc := media.NewService(repo, sse.NoopPublisher{})
	ctx := context.Background()

	movie := &media.Movie{IMDBID: "tt-svc"}
	mediaRow := &media.Media{
		Type: media.MediaTypeMovie, Name: "Film", Path: "/dl", Indexer: "x", Quality: "1080p",
		UserID: 1, Username: "u",
	}
	if err := repo.CreateMovieWithMedia(ctx, movie, mediaRow); err != nil {
		t.Fatal(err)
	}

	if err := svc.RemoveFromRequest(ctx, media.CreateFromRequestInput{
		MediaID: mediaRow.ID,
		Type:    "movie_remove",
	}); err != nil {
		t.Fatalf("remove: %v", err)
	}

	var count int64
	if err := db.Model(&media.Media{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected media gone, count=%d", count)
	}
}

func TestUpdateLibraryPath_PersistsDestination(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateMedia(t, db)
	repo := media.NewRepository(db)
	svc := media.NewService(repo, sse.NoopPublisher{})
	ctx := context.Background()

	movie := &media.Movie{IMDBID: "tt-lib"}
	mediaRow := &media.Media{
		Type: media.MediaTypeMovie, Name: "Film", Path: "/dl", Indexer: "x", Quality: "1080p",
		UserID: 1, Username: "u",
	}
	if err := repo.CreateMovieWithMedia(ctx, movie, mediaRow); err != nil {
		t.Fatal(err)
	}

	libPath := "/movies/Film (tt-lib) (1080p)"
	if err := svc.UpdateLibraryPath(ctx, mediaRow.ID, libPath); err != nil {
		t.Fatal(err)
	}

	loaded, err := repo.FindByID(ctx, mediaRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LibraryPath != libPath {
		t.Fatalf("library_path = %q, want %q", loaded.LibraryPath, libPath)
	}
}

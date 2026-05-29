package requests_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/requests"
	"github.com/Cristian0711/media-bridge/backend/internal/sse"
	"github.com/Cristian0711/media-bridge/backend/tests/testhelpers"
	"gorm.io/gorm"
)

func newRequestsRepo(t *testing.T) (requests.Repository, *gorm.DB) {
	t.Helper()
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateRequests(t, db)
	return requests.NewRepository(db, sse.NoopPublisher{}), db
}

func TestCreateRemoveIfAbsent_DedupesConcurrentScope(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	first := &requests.Request{
		Type: "movie_remove", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 42,
		Indexer: "x", Quality: "1080p",
	}
	_, created, err := repo.CreateRemoveIfAbsent(ctx, first, 42, "movie_remove", nil)
	if err != nil || !created {
		t.Fatalf("first create: created=%v err=%v", created, err)
	}

	second := &requests.Request{
		Type: "movie_remove", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r2", MediaID: 42,
		Indexer: "x", Quality: "1080p",
	}
	_, created, err = repo.CreateRemoveIfAbsent(ctx, second, 42, "movie_remove", nil)
	if err != nil {
		t.Fatalf("second create err: %v", err)
	}
	if created {
		t.Fatal("expected duplicate remove to be rejected")
	}
}

func TestCreateRemoveIfAbsent_RollsBackWhenEnqueueFails(t *testing.T) {
	repo, db := newRequestsRepo(t)
	ctx := context.Background()

	entry := &requests.Request{
		Type: "movie_remove", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 7,
		Indexer: "x", Quality: "1080p",
	}
	_, created, err := repo.CreateRemoveIfAbsent(ctx, entry, 7, "movie_remove", func(tx *gorm.DB, e *requests.Request) error {
		return fmt.Errorf("simulated enqueue failure")
	})
	if err == nil {
		t.Fatal("expected enqueue error")
	}
	if created {
		t.Fatal("should not report created on rollback")
	}
	var count int64
	if err := db.Model(&requests.Request{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("poison-pill row must not persist, count=%d", count)
	}
}

func TestCreateRemoveIfAbsent_CommitsRequestAndQueueAtomically(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateRequests(t, db)
	testhelpers.CreateProcessingQueueTable(t, db)
	repo := requests.NewRepository(db, sse.NoopPublisher{})
	ctx := context.Background()

	entry := &requests.Request{
		Type: "movie_remove", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 9,
		Indexer: "x", Quality: "1080p",
	}
	_, created, err := repo.CreateRemoveIfAbsent(ctx, entry, 9, "movie_remove", testhelpers.RequestsEnqueueInTx(t))
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	if n := testhelpers.CountQueueJobsForRequest(t, db, entry.ID); n != 1 {
		t.Fatalf("expected 1 queue job, got %d", n)
	}
}

func TestCreateMovieDownloadIfAbsent_CommitsRequestAndQueueAtomically(t *testing.T) {
	db := testhelpers.OpenSQLite(t)
	testhelpers.MigrateRequests(t, db)
	testhelpers.CreateProcessingQueueTable(t, db)
	repo := requests.NewRepository(db, sse.NoopPublisher{})
	ctx := context.Background()

	entry := &requests.Request{
		Type: "movie_download", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "dl1",
		IMDBID: "tt123", Quality: "1080p", Indexer: "x",
	}
	_, created, err := repo.CreateMovieDownloadIfAbsent(ctx, entry, "tt123", "", "1080p", testhelpers.RequestsEnqueueInTx(t))
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	if n := testhelpers.CountQueueJobsForRequest(t, db, entry.ID); n != 1 {
		t.Fatalf("expected 1 queue job, got %d", n)
	}
}

func TestCreateMovieDownloadIfAbsent_DedupesInFlightDownload(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	first := &requests.Request{
		Type: "movie_download", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1",
		IMDBID: "tt999", Quality: "1080p", Indexer: "x",
	}
	_, created, err := repo.CreateMovieDownloadIfAbsent(ctx, first, "tt999", "", "1080p", nil)
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}

	second := &requests.Request{
		Type: "movie_download", Status: "pending", Name: "Film",
		UserID: 2, Username: "v", RequestID: "r2",
		IMDBID: "tt999", Quality: "1080p", Indexer: "x",
	}
	_, created, err = repo.CreateMovieDownloadIfAbsent(ctx, second, "tt999", "", "1080p", nil)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected duplicate download dedup")
	}
}

func TestCancelDownloadsByMediaID_IncludesDownloaded(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_download", Status: "downloaded", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 99,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatalf("create: %v", err)
	}

	n, err := repo.CancelDownloadsByMediaID(ctx, 99)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row cancelled, got %d", n)
	}
	updated, err := repo.FindByID(ctx, row.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if updated.Status != "cancelled" {
		t.Fatalf("expected cancelled, got %q", updated.Status)
	}
}

func TestCancelDownloadsByMediaID_SkipsAlreadyCancelled(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_download", Status: "cancelled", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 55,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatal(err)
	}
	n, err := repo.CancelDownloadsByMediaID(ctx, 55)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 updates, got %d", n)
	}
}

func TestMarkQueuedIfPending_GuardedTransition(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_download", Status: "pending", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1",
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatalf("create: %v", err)
	}

	ok, err := repo.MarkQueuedIfPending(ctx, row.ID)
	if err != nil || !ok {
		t.Fatalf("first mark: ok=%v err=%v", ok, err)
	}
	ok, err = repo.MarkQueuedIfPending(ctx, row.ID)
	if err != nil {
		t.Fatalf("second mark err: %v", err)
	}
	if ok {
		t.Fatal("expected guarded transition to reject second update")
	}
}

func TestMarkRemovingIfPending_DoesNotClobberRemoved(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_remove", Status: "removed", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 3,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatal(err)
	}
	ok, err := repo.MarkRemovingIfPending(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("must not regress removed → removing")
	}
}

func TestMarkDownloadedIfDownloading_RejectsCancelled(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_download", Status: "cancelled", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 8,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatal(err)
	}
	ok, err := repo.MarkDownloadedIfDownloading(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("watcher must not finalize cancelled download")
	}
}

func TestMarkRemovedIfRemoving_GuardedTransition(t *testing.T) {
	repo, _ := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_remove", Status: "removing", Name: "Film",
		UserID: 1, Username: "u", RequestID: "r1", MediaID: 11,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatal(err)
	}

	ok, err := repo.MarkRemovedIfRemoving(ctx, row.ID)
	if err != nil || !ok {
		t.Fatalf("first mark: ok=%v err=%v", ok, err)
	}
	ok, err = repo.MarkRemovedIfRemoving(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected guarded transition to reject second update")
	}
}

func TestListOrphanedPending_RespectsMinAge(t *testing.T) {
	repo, db := newRequestsRepo(t)
	ctx := context.Background()

	fresh := &requests.Request{
		Type: "movie_remove", Status: "pending", Name: "Fresh",
		UserID: 1, Username: "u", RequestID: "fresh", MediaID: 1,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, fresh); err != nil {
		t.Fatal(err)
	}

	stale := &requests.Request{
		Type: "movie_remove", Status: "pending", Name: "Stale",
		UserID: 1, Username: "u", RequestID: "stale", MediaID: 2,
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, stale); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * time.Minute)
	if err := db.Model(&requests.Request{}).Where("id = ?", stale.ID).
		Updates(map[string]any{"created_at": staleTime, "updated_at": staleTime}).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := repo.ListOrphanedPending(ctx, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != stale.ID {
		t.Fatalf("expected only stale row, got %+v", rows)
	}
}

func TestListStuckQueued_FiltersRowsWithDownloadJob(t *testing.T) {
	repo, db := newRequestsRepo(t)
	ctx := context.Background()

	row := &requests.Request{
		Type: "movie_download", Status: "queued", Name: "Film",
		UserID: 1, Username: "u", RequestID: "q1",
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * time.Minute)
	if err := db.Model(&requests.Request{}).Where("id = ?", row.ID).
		Update("updated_at", staleTime).Error; err != nil {
		t.Fatal(err)
	}

	hasJob := func(ctx context.Context, id uint) (bool, error) {
		return id == row.ID, nil
	}
	rows, err := repo.ListStuckQueued(ctx, time.Minute, hasJob)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no stuck rows when download job exists, got %d", len(rows))
	}

	hasJob = func(ctx context.Context, id uint) (bool, error) {
		return false, nil
	}
	rows, err = repo.ListStuckQueued(ctx, time.Minute, hasJob)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != row.ID {
		t.Fatalf("expected stuck row without job, got %+v", rows)
	}
}

func TestPurgeTerminalOlderThan_DeletesOldRows(t *testing.T) {
	repo, db := newRequestsRepo(t)
	ctx := context.Background()

	old := &requests.Request{
		Type: "movie_download", Status: "downloaded", Name: "Old",
		UserID: 1, Username: "u", RequestID: "old",
		Indexer: "x", Quality: "1080p",
	}
	if err := repo.Create(ctx, old); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	if err := db.Model(&requests.Request{}).Where("id = ?", old.ID).
		Update("updated_at", oldTime).Error; err != nil {
		t.Fatal(err)
	}

	n, err := repo.PurgeTerminalOlderThan(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 purged, got %d", n)
	}
}

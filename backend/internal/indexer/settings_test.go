package indexer

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSettingsRepo(t *testing.T) SettingsRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&IndexerSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewSettingsRepository(db)
}

func freeleechOnlyFor(t *testing.T, repo SettingsRepository, name string) bool {
	t.Helper()
	list, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range list {
		if s.IndexerName == name {
			return s.FreeleechOnly
		}
	}
	t.Fatalf("setting %q not found in %+v", name, list)
	return false
}

// TestSettingsUpsert_PersistsFalse guards the GORM default-tag zero-value bug:
// toggling an indexer to freeleech_only=false must actually persist as false on
// both the initial insert and a subsequent update, not silently revert to true.
func TestSettingsUpsert_PersistsFalse(t *testing.T) {
	t.Parallel()
	repo := newSettingsRepo(t)
	ctx := context.Background()

	// Initial insert with the zero value (false).
	if _, err := repo.Upsert(ctx, "FileList.io", false); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if freeleechOnlyFor(t, repo, "FileList.io") {
		t.Fatal("insert of freeleech_only=false persisted as true")
	}

	// Update to true.
	if _, err := repo.Upsert(ctx, "FileList.io", true); err != nil {
		t.Fatalf("upsert update->true: %v", err)
	}
	if !freeleechOnlyFor(t, repo, "FileList.io") {
		t.Fatal("update to freeleech_only=true did not persist")
	}

	// Update back to false (on-conflict path with zero value).
	if _, err := repo.Upsert(ctx, "FileList.io", false); err != nil {
		t.Fatalf("upsert update->false: %v", err)
	}
	if freeleechOnlyFor(t, repo, "FileList.io") {
		t.Fatal("update to freeleech_only=false silently reverted to true")
	}
}

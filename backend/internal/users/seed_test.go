package users_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedRuntimeInsertsRolesAndPromotesFirstUser(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&users.RoleRecord{}, &users.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := users.SeedRuntime(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var roleCount int64
	if err := db.Model(&users.RoleRecord{}).Count(&roleCount).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if roleCount != 2 {
		t.Fatalf("expected 2 roles, got %d", roleCount)
	}

	user := users.User{Username: "alice", Role: users.RoleUser, PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := users.SeedRuntime(db); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	var updated users.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if updated.Role != users.RoleAdmin {
		t.Fatalf("expected first user promoted to admin, got %q", updated.Role)
	}
}

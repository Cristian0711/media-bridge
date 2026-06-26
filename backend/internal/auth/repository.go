package auth

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	CreateKey(ctx context.Context, value string) (*Key, error)
	FindKey(ctx context.Context, value string) (*Key, error)
	ListKeys(ctx context.Context) ([]Key, error)
	ConsumeKey(ctx context.Context, value string) (bool, error)
	ReactivateKey(ctx context.Context, value string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateKey(ctx context.Context, value string) (*Key, error) {
	key := &Key{Value: value}
	return key, r.db.WithContext(ctx).Create(key).Error
}

func (r *repository) FindKey(ctx context.Context, value string) (*Key, error) {
	var key Key
	err := r.db.WithContext(ctx).
		Where("value = ?", value).
		First(&key).Error
	return &key, err
}

func (r *repository) ListKeys(ctx context.Context) ([]Key, error) {
	var keys []Key
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// ConsumeKey atomically claims a single-use key. The conditional UPDATE
// (is_active = true) is serialized by the database, so concurrent registrations
// racing on the same key cannot both observe it as active and consume it twice.
// It reports whether this caller won the claim (RowsAffected == 1).
func (r *repository) ConsumeKey(ctx context.Context, value string) (bool, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&Key{}).
		Where("value = ? AND is_active = ?", value, true).
		Updates(map[string]any{"is_active": false, "used_at": now})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ReactivateKey releases a previously claimed key. It is used to compensate when
// user creation fails after the key was consumed, so the invite stays valid.
func (r *repository) ReactivateKey(ctx context.Context, value string) error {
	return r.db.WithContext(ctx).
		Model(&Key{}).
		Where("value = ?", value).
		Update("is_active", true).Error
}

package auth

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	CreateKey(ctx context.Context, value string) (*Key, error)
	FindKey(ctx context.Context, value string) (*Key, error)
	DisableKey(ctx context.Context, value string) error
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

func (r *repository) DisableKey(ctx context.Context, value string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Key{}).
		Where("value = ?", value).
		Updates(map[string]any{"is_active": false, "used_at": now}).Error
}
package users

import (
	"context"

	"gorm.io/gorm"
)

type Repository interface {
	FindByID(ctx context.Context, id uint) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, user *User) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindByID(ctx context.Context, id uint) (*User, error) {
	var user User
	return &user, r.db.WithContext(ctx).First(&user, id).Error
}

func (r *repository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	return &user, r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
}

func (r *repository) Create(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

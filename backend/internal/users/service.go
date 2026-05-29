package users

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("user not found")

type Service interface {
	FindByID(ctx context.Context, id uint) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, input CreateInput) (*User, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) FindByID(ctx context.Context, id uint) (*User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return user, err
}

func (s *service) FindByUsername(ctx context.Context, username string) (*User, error) {
	user, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return user, err
}

func (s *service) Create(ctx context.Context, input CreateInput) (*User, error) {
	user := &User{
		Username:     input.Username,
		PasswordHash: input.PasswordHash,
	}
	return user, s.repo.Create(ctx, user)
}

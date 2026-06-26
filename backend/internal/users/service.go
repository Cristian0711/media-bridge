package users

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("user not found")

type Service interface {
	FindByID(ctx context.Context, id uint) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	Count(ctx context.Context) (int64, error)
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

func (s *service) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *service) Create(ctx context.Context, input CreateInput) (*User, error) {
	role := input.Role
	if role == "" {
		role = RoleUser
	}
	// Normalize username to lowercase so the case-sensitive unique index cannot
	// be bypassed by a caller that did not pre-normalize (e.g. a direct Create).
	user := &User{
		Username:     strings.ToLower(input.Username),
		Role:         role,
		PasswordHash: input.PasswordHash,
	}
	return user, s.repo.Create(ctx, user)
}

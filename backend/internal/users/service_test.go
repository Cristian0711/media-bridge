package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"gorm.io/gorm"
)

type usersRepoStub struct {
	findByIDFn       func(ctx context.Context, id uint) (*users.User, error)
	findByUsernameFn func(ctx context.Context, username string) (*users.User, error)
	countFn          func(ctx context.Context) (int64, error)
	createFn         func(ctx context.Context, user *users.User) error
}

func (r *usersRepoStub) FindByID(ctx context.Context, id uint) (*users.User, error) {
	return r.findByIDFn(ctx, id)
}
func (r *usersRepoStub) FindByUsername(ctx context.Context, username string) (*users.User, error) {
	return r.findByUsernameFn(ctx, username)
}
func (r *usersRepoStub) Count(ctx context.Context) (int64, error) {
	if r.countFn == nil {
		return 0, nil
	}
	return r.countFn(ctx)
}
func (r *usersRepoStub) Create(ctx context.Context, user *users.User) error {
	return r.createFn(ctx, user)
}

func TestFindByIDAndUsernameMapNotFound(t *testing.T) {
	t.Parallel()
	svc := users.NewService(&usersRepoStub{
		findByIDFn:       func(context.Context, uint) (*users.User, error) { return nil, gorm.ErrRecordNotFound },
		findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, gorm.ErrRecordNotFound },
		createFn:         func(context.Context, *users.User) error { return nil },
	})

	if _, err := svc.FindByID(context.Background(), 1); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for FindByID, got %v", err)
	}
	if _, err := svc.FindByUsername(context.Background(), "alice"); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for FindByUsername, got %v", err)
	}
}

func TestCreatePassesInput(t *testing.T) {
	t.Parallel()
	var captured *users.User
	svc := users.NewService(&usersRepoStub{
		findByIDFn:       func(context.Context, uint) (*users.User, error) { return nil, nil },
		findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, nil },
		createFn: func(_ context.Context, user *users.User) error {
			captured = user
			return nil
		},
	})

	resp, err := svc.Create(context.Background(), users.CreateInput{Username: "alice", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || captured == nil {
		t.Fatal("expected user to be created")
	}
	if captured.Username != "alice" || captured.PasswordHash != "hash" || captured.Role != users.RoleUser {
		t.Fatalf("unexpected user passed to repo: %+v", captured)
	}
}

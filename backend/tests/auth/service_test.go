package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/auth"
	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type authRepoStub struct {
	createKeyFn  func(ctx context.Context, value string) (*auth.Key, error)
	findKeyFn    func(ctx context.Context, value string) (*auth.Key, error)
	listKeysFn   func(ctx context.Context) ([]auth.Key, error)
	disableKeyFn func(ctx context.Context, value string) error
}

func (r *authRepoStub) CreateKey(ctx context.Context, value string) (*auth.Key, error) {
	return r.createKeyFn(ctx, value)
}
func (r *authRepoStub) FindKey(ctx context.Context, value string) (*auth.Key, error) {
	return r.findKeyFn(ctx, value)
}
func (r *authRepoStub) ListKeys(ctx context.Context) ([]auth.Key, error) {
	if r.listKeysFn == nil {
		return nil, nil
	}
	return r.listKeysFn(ctx)
}
func (r *authRepoStub) DisableKey(ctx context.Context, value string) error {
	return r.disableKeyFn(ctx, value)
}

type usersSvcStub struct {
	findByIDFn       func(ctx context.Context, id uint) (*users.User, error)
	findByUsernameFn func(ctx context.Context, username string) (*users.User, error)
	countFn          func(ctx context.Context) (int64, error)
	createFn         func(ctx context.Context, input users.CreateInput) (*users.User, error)
}

func (s *usersSvcStub) FindByID(ctx context.Context, id uint) (*users.User, error) {
	if s.findByIDFn == nil {
		return nil, users.ErrNotFound
	}
	return s.findByIDFn(ctx, id)
}
func (s *usersSvcStub) FindByUsername(ctx context.Context, username string) (*users.User, error) {
	if s.findByUsernameFn == nil {
		return nil, users.ErrNotFound
	}
	return s.findByUsernameFn(ctx, username)
}
func (s *usersSvcStub) Count(ctx context.Context) (int64, error) {
	if s.countFn == nil {
		return 0, nil
	}
	return s.countFn(ctx)
}
func (s *usersSvcStub) Create(ctx context.Context, input users.CreateInput) (*users.User, error) {
	if s.createFn == nil {
		return nil, errors.New("create not configured")
	}
	return s.createFn(ctx, input)
}

func TestRegisterPaths(t *testing.T) {
	t.Parallel()

	t.Run("invalid key when missing", func(t *testing.T) {
		t.Parallel()
		svc := auth.NewService(
			&authRepoStub{
				findKeyFn:    func(context.Context, string) (*auth.Key, error) { return nil, gorm.ErrRecordNotFound },
				disableKeyFn: func(context.Context, string) error { return nil },
				createKeyFn:  func(context.Context, string) (*auth.Key, error) { return nil, nil },
			},
			&usersSvcStub{},
			auth.NewJWTManager("secret"),
		)
		_, err := svc.Register(context.Background(), auth.RegisterRequest{Username: "alice", Password: "password", Key: "x"})
		if !errors.Is(err, auth.ErrKeyInvalid) {
			t.Fatalf("expected ErrKeyInvalid, got %v", err)
		}
	})

	t.Run("already exists from pre-check", func(t *testing.T) {
		t.Parallel()
		svc := auth.NewService(
			&authRepoStub{
				findKeyFn:    func(context.Context, string) (*auth.Key, error) { return &auth.Key{Value: "k", IsActive: true}, nil },
				disableKeyFn: func(context.Context, string) error { return nil },
				createKeyFn:  func(context.Context, string) (*auth.Key, error) { return nil, nil },
			},
			&usersSvcStub{
				findByUsernameFn: func(context.Context, string) (*users.User, error) { return &users.User{ID: 1}, nil },
			},
			auth.NewJWTManager("secret"),
		)
		_, err := svc.Register(context.Background(), auth.RegisterRequest{Username: "alice", Password: "password", Key: "k"})
		if !errors.Is(err, auth.ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})

	t.Run("maps postgres unique violation", func(t *testing.T) {
		t.Parallel()
		svc := auth.NewService(
			&authRepoStub{
				findKeyFn:    func(context.Context, string) (*auth.Key, error) { return &auth.Key{Value: "k", IsActive: true}, nil },
				disableKeyFn: func(context.Context, string) error { return nil },
				createKeyFn:  func(context.Context, string) (*auth.Key, error) { return nil, nil },
			},
			&usersSvcStub{
				findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, users.ErrNotFound },
				createFn: func(context.Context, users.CreateInput) (*users.User, error) {
					return nil, &pgconn.PgError{Code: "23505"}
				},
			},
			auth.NewJWTManager("secret"),
		)
		_, err := svc.Register(context.Background(), auth.RegisterRequest{Username: "alice", Password: "password", Key: "k"})
		if !errors.Is(err, auth.ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})
}

func TestRegisterSuccessNormalizesAndDisablesKey(t *testing.T) {
	t.Parallel()
	var created users.CreateInput
	var disabled string

	svc := auth.NewService(
		&authRepoStub{
			findKeyFn:    func(context.Context, string) (*auth.Key, error) { return &auth.Key{Value: "k", IsActive: true}, nil },
			disableKeyFn: func(_ context.Context, value string) error { disabled = value; return nil },
			createKeyFn:  func(context.Context, string) (*auth.Key, error) { return nil, nil },
		},
		&usersSvcStub{
			countFn:          func(context.Context) (int64, error) { return 1, nil },
			findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, users.ErrNotFound },
			createFn: func(_ context.Context, input users.CreateInput) (*users.User, error) {
				created = input
				return &users.User{ID: 7, Username: input.Username, Role: input.Role}, nil
			},
		},
		auth.NewJWTManager("secret"),
	)

	resp, err := svc.Register(context.Background(), auth.RegisterRequest{Username: "Alice", Password: "password", Key: "k"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Username != "alice" || created.Username != "alice" {
		t.Fatalf("expected normalized username alice, got resp=%q input=%q", resp.Username, created.Username)
	}
	if created.PasswordHash == "" {
		t.Fatal("expected password hash")
	}
	if disabled != "k" {
		t.Fatalf("expected disabled key k, got %q", disabled)
	}
}

func TestLoginAndValidateToken(t *testing.T) {
	t.Parallel()
	jwtm := auth.NewJWTManager("secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate hash: %v", err)
	}
	svc := auth.NewService(
		&authRepoStub{
			createKeyFn:  func(context.Context, string) (*auth.Key, error) { return nil, nil },
			findKeyFn:    func(context.Context, string) (*auth.Key, error) { return nil, nil },
			disableKeyFn: func(context.Context, string) error { return nil },
		},
		&usersSvcStub{
			findByUsernameFn: func(context.Context, string) (*users.User, error) {
				return &users.User{ID: 3, Username: "alice", Role: users.RoleUser, PasswordHash: string(hash)}, nil
			},
		},
		jwtm,
	)

	loginResp, err := svc.Login(context.Background(), auth.LoginRequest{Username: "alice", Password: "password"})
	if err != nil {
		t.Fatalf("expected login success, got %v", err)
	}
	validResp, err := svc.ValidateToken(context.Background(), loginResp.Token)
	if err != nil {
		t.Fatalf("expected validate success, got %v", err)
	}
	if !validResp.Valid || validResp.UserID != 3 {
		t.Fatalf("expected valid token for user 3, got %+v", validResp)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	t.Parallel()
	svc := auth.NewService(
		&authRepoStub{
			createKeyFn:  func(context.Context, string) (*auth.Key, error) { return nil, nil },
			findKeyFn:    func(context.Context, string) (*auth.Key, error) { return nil, nil },
			disableKeyFn: func(context.Context, string) error { return nil },
		},
		&usersSvcStub{
			findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, users.ErrNotFound },
		},
		auth.NewJWTManager("secret"),
	)
	_, err := svc.Login(context.Background(), auth.LoginRequest{Username: "missing", Password: "password"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRegisterFirstUserIsAdmin(t *testing.T) {
	t.Parallel()
	var created users.CreateInput
	svc := auth.NewService(
		&authRepoStub{
			findKeyFn:    func(context.Context, string) (*auth.Key, error) { return &auth.Key{Value: "k", IsActive: true}, nil },
			disableKeyFn: func(context.Context, string) error { return nil },
		},
		&usersSvcStub{
			countFn:          func(context.Context) (int64, error) { return 0, nil },
			findByUsernameFn: func(context.Context, string) (*users.User, error) { return nil, users.ErrNotFound },
			createFn: func(_ context.Context, input users.CreateInput) (*users.User, error) {
				created = input
				return &users.User{ID: 1, Username: input.Username, Role: input.Role}, nil
			},
		},
		auth.NewJWTManager("secret"),
	)
	resp, err := svc.Register(context.Background(), auth.RegisterRequest{Username: "admin", Password: "password", Key: "k"})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if created.Role != users.RoleAdmin || resp.Role != users.RoleAdmin {
		t.Fatalf("expected admin role, got created=%q resp=%q", created.Role, resp.Role)
	}
}

func TestGenerateKeyAndGetStatus(t *testing.T) {
	t.Parallel()
	svc := auth.NewService(
		&authRepoStub{
			createKeyFn: func(_ context.Context, value string) (*auth.Key, error) {
				return &auth.Key{Value: value, IsActive: true}, nil
			},
			findKeyFn: func(_ context.Context, value string) (*auth.Key, error) {
				if value == "missing" {
					return nil, gorm.ErrRecordNotFound
				}
				return &auth.Key{Value: value, IsActive: true}, nil
			},
			disableKeyFn: func(context.Context, string) error { return nil },
		},
		&usersSvcStub{},
		auth.NewJWTManager("secret"),
	)

	keyResp, err := svc.GenerateKey(context.Background())
	if err != nil || keyResp.Key == "" {
		t.Fatalf("expected generated key, got resp=%+v err=%v", keyResp, err)
	}
	_, err = svc.GetKeyStatus(context.Background(), "missing")
	if !errors.Is(err, auth.ErrKeyInvalid) {
		t.Fatalf("expected ErrKeyInvalid, got %v", err)
	}
}

func TestListKeysReturnsAll(t *testing.T) {
	t.Parallel()
	svc := auth.NewService(
		&authRepoStub{
			listKeysFn: func(context.Context) ([]auth.Key, error) {
				return []auth.Key{
					{Value: "a", IsActive: true},
					{Value: "b", IsActive: false},
				}, nil
			},
		},
		&usersSvcStub{},
		auth.NewJWTManager("secret"),
	)
	resp, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(resp.Keys) != 2 || resp.Keys[0].Status != "available" || resp.Keys[1].Status != "used" {
		t.Fatalf("unexpected keys: %+v", resp.Keys)
	}
}

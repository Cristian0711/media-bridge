package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/Cristian0711/media-bridge/backend/internal/users"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrKeyInvalid         = errors.New("key is invalid or already used")
)

type Service interface {
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	ValidateToken(ctx context.Context, token string) (*ValidateResponse, error)
	GenerateKey(ctx context.Context) (*GenerateKeyResponse, error)
	GetKeyStatus(ctx context.Context, value string) (*KeyStatusResponse, error)
}

type service struct {
	repo    Repository
	userSvc users.Service
	jwt     *JWTManager
}

func NewService(repo Repository, userSvc users.Service, jwt *JWTManager) Service {
	return &service{repo: repo, userSvc: userSvc, jwt: jwt}
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.userSvc.FindByUsername(ctx, strings.ToLower(req.Username))
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	token, err := s.jwt.Generate(user.ID, user.Username)
	if err != nil {
		return nil, err
	}
	return &LoginResponse{Token: token}, nil
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	username := strings.ToLower(req.Username)

	key, err := s.repo.FindKey(ctx, req.Key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyInvalid
		}
		return nil, err
	}
	if !key.IsActive {
		return nil, ErrKeyInvalid
	}

	_, err = s.userSvc.FindByUsername(ctx, username)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	if !errors.Is(err, users.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user, err := s.userSvc.Create(ctx, users.CreateInput{
		Username:     username,
		PasswordHash: string(hash),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	// Key is consumed after successful registration.
	if err := s.repo.DisableKey(ctx, req.Key); err != nil {
		return nil, err
	}

	return &RegisterResponse{ID: user.ID, Username: user.Username}, nil
}

func (s *service) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	claims, err := s.jwt.Parse(token)
	if err != nil {
		return &ValidateResponse{Valid: false}, nil
	}

	return &ValidateResponse{
		Valid:    true,
		UserID:   claims.UserID,
		Username: claims.Username,
	}, nil
}

func (s *service) GenerateKey(ctx context.Context) (*GenerateKeyResponse, error) {
	key, err := s.repo.CreateKey(ctx, uuid.New().String())
	if err != nil {
		return nil, err
	}
	return &GenerateKeyResponse{Key: key.Value}, nil
}

func (s *service) GetKeyStatus(ctx context.Context, value string) (*KeyStatusResponse, error) {
	key, err := s.repo.FindKey(ctx, value)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyInvalid
		}
		return nil, err
	}
	return &KeyStatusResponse{Value: key.Value, IsActive: key.IsActive}, nil
}

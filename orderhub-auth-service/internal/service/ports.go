package service

import (
	"context"
	"time"

	"auth-service/internal/models"
	repo "auth-service/internal/repository"

	"github.com/google/uuid"
)

type UserRepo interface {
	Create(ctx context.Context, u *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdatePassword(ctx context.Context, user *models.User) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type RefreshRepo interface {
	Create(ctx context.Context, t *models.RefreshToken) error
	RevokeByHash(ctx context.Context, userID uuid.UUID, hash string) (bool, error)
	RevokeAll(ctx context.Context, userID uuid.UUID) (int64, error)
	Touch(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error
	IsActive(ctx context.Context, userID uuid.UUID, hash string, now time.Time) (bool, error)
	GetByHash(ctx context.Context, userID uuid.UUID, hash string) (*models.RefreshToken, error)
	GetByHashOnly(ctx context.Context, hash string) (*models.RefreshToken, error)
	IsActiveByHash(ctx context.Context, hash string, now time.Time) (bool, error)
	RevokeByHashOnly(ctx context.Context, hash string) (bool, error)
}

type JWKRepo interface {
	ListPublic(ctx context.Context) ([]PublicJWK, error)
}

// PublicJWK — алиас репозиторного типа, чтобы не дублировать структуру
// и чтобы repository.JWKRepo удовлетворял интерфейсу JWKRepo сервиса.
type PublicJWK = repo.PublicJWK

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type Claims struct {
	UserID uuid.UUID
	Role   string
	Exp    time.Time
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshOpaque    string // выдаём клиенту
	RefreshExpiresAt time.Time
	RefreshHash      string // сохраняем в БД
}

type TokenProvider interface {
	SignAccess(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (token string, exp time.Time, err error)
	NewRefresh(ctx context.Context, sub uuid.UUID, ttl time.Duration) (opaque string, hash string, exp time.Time, err error)
	ParseAndValidateAccess(ctx context.Context, token string) (*Claims, error)
	// JWKS нужен только при RSA, при HS можно вернуть пусто
	ListPublicJWK(ctx context.Context) ([]PublicJWK, error)
}

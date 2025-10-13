package service

import (
	"context"
	"time"

	"auth-service/internal/models"
	"auth-service/internal/producer"
	repo "auth-service/internal/repository"

	"github.com/google/uuid"
)

type UserRepo interface {
	Create(ctx context.Context, u *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdatePassword(ctx context.Context, user *models.User) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateIsEmailVerified(ctx context.Context, user *models.User) error
}

type RefreshRepo interface {
	Create(ctx context.Context, t *models.RefreshToken) error
	RevokeAll(ctx context.Context, userID uuid.UUID) (int64, error)
	Touch(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error
	GetByHashOnly(ctx context.Context, hash string) (*models.RefreshToken, error)
	IsActiveByHash(ctx context.Context, hash string, now time.Time) (bool, error)
	RevokeByHashOnly(ctx context.Context, hash string) (bool, error)
	HasActiveBySession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error)
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

type SessionRepo interface {
	Create(ctx context.Context, s *models.UserSession) error
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
	Revoke(ctx context.Context, id uuid.UUID) (bool, error)
	RevokeAllByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListActiveByUser(ctx context.Context, userID uuid.UUID, since time.Time) ([]models.UserSession, error)
}

type PasswordResetRepo interface {
	Create(ctx context.Context, t *models.PasswordResetToken) error
	GetValidByHash(ctx context.Context, codeHash string, now time.Time) (*models.PasswordResetToken, error)
	Consume(ctx context.Context, id string) (bool, error)
	DeleteAllForUser(ctx context.Context, userID string) (int64, error)
	FindLatestByUser(ctx context.Context, userID uuid.UUID) (*models.PasswordResetToken, error)
}

type EmailVerificationRepo interface {
	Create(ctx context.Context, v *models.EmailVerification) error
	GetValidByHash(ctx context.Context, codeHash string, now time.Time) (*models.EmailVerification, error)
	Consume(ctx context.Context, id string) (bool, error)
	DeleteAllForUser(ctx context.Context, userID string) (int64, error)
	FindLatestByUser(ctx context.Context, userID uuid.UUID) (*models.EmailVerification, error)
}

type CacheClient interface {
	// Rate limiting
	SetRateLimit(ctx context.Context, key string, ttl time.Duration) error
	CheckRateLimit(ctx context.Context, key string) (bool, error)

	// JWK кэширование
	SetJWK(ctx context.Context, kid string, jwkData []byte, ttl time.Duration) error
	GetJWK(ctx context.Context, kid string) ([]byte, error)

	// Blacklist токенов
	BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)

	// Общие методы
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
}

type EmailProducer interface {
	SendEmail(ctx context.Context, key string, msg producer.EmailMessage) error
}

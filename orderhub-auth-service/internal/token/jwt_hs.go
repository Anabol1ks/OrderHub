package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"auth-service/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type HSProvider struct {
	accessSecret  []byte
	refreshSecret []byte // используется только для HMAC подписи opaque? — нет, opaque у нас случайный, секрет не нужен
	issuer        string
	audience      string
	now           func() time.Time
}

func NewHSProvider(accessSecret, refreshSecret, issuer, audience string) *HSProvider {
	return &HSProvider{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		issuer:        issuer,
		audience:      audience,
		now:           time.Now,
	}
}

type customClaims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (p *HSProvider) SignAccess(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (string, time.Time, error) {
	now := p.now()
	exp := now.Add(ttl)

	claims := customClaims{
		Sub:  sub.String(),
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    p.issuer,
			Subject:   sub.String(),
			Audience:  []string{p.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(p.accessSecret)
	return signed, exp, err
}

// Refresh: opaque (случайная строка), hash = sha256(opaque) base64url
func (p *HSProvider) NewRefresh(ctx context.Context, sub uuid.UUID, ttl time.Duration) (opaque string, hash string, exp time.Time, err error) {
	exp = p.now().Add(ttl)
	opaque, err = randomOpaque(32) // 32 байта -> 43-44 base64url символа
	if err != nil {
		return "", "", time.Time{}, err
	}
	sum := sha256.Sum256([]byte(opaque))
	hash = base64.RawURLEncoding.EncodeToString(sum[:])
	return opaque, hash, exp, nil
}

func (p *HSProvider) ParseAndValidateAccess(ctx context.Context, token string) (*service.Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &customClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return p.accessSecret, nil
	}, jwt.WithAudience(p.audience), jwt.WithIssuer(p.issuer))
	if err != nil {
		return nil, err
	}
	cc, ok := parsed.Claims.(*customClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	uid, err := uuid.Parse(cc.Sub)
	if err != nil {
		return nil, err
	}
	return &service.Claims{UserID: uid, Role: cc.Role, Exp: cc.ExpiresAt.Time}, nil
}

func (p *HSProvider) ListPublicJWK(ctx context.Context) ([]service.PublicJWK, error) {
	// HS не поддерживает JWKS, вернём пусто
	return []service.PublicJWK{}, nil
}

func randomOpaque(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

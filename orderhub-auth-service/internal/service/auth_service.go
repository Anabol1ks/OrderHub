package service

import (
	"auth-service/internal/models"
	"auth-service/internal/util"
	"context"
	"time"

	"github.com/google/uuid"
)

type AuthService struct {
	users   UserRepo
	refresh RefreshRepo
	jwks    JWKRepo // может быть nil при HS256
	hasher  PasswordHasher
	tokens  TokenProvider

	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

type ClientMeta struct {
	ClientID  *string
	IP        *string
	UserAgent *string
}

func NewAuthService(
	users UserRepo,
	refresh RefreshRepo,
	jwks JWKRepo,
	hasher PasswordHasher,
	tokens TokenProvider,
	accessTTL, refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:   users,
		refresh: refresh,
		jwks:    jwks,
		hasher:  hasher,
		tokens:  tokens,

		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, role string) (*models.User, error) {
	exists, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyExists
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return nil, err
	}

	u := &models.User{
		ID:              uuid.New(),
		Email:           email,
		Password:        hash,
		Role:            "ROLE_CUSTOMER",
		IsEmailVerified: false,
	}

	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string, meta ClientMeta) (uuid.UUID, string, TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, "", TokenPair{}, ErrNotFound
	}

	if user == nil || !s.hasher.Compare(user.Password, password) {
		return uuid.Nil, "", TokenPair{}, ErrInvalidCredentials
	}

	access, aexp, err := s.tokens.SignAccess(ctx, user.ID, string(user.Role), s.accessTTL)
	if err != nil {
		return uuid.Nil, "", TokenPair{}, err
	}

	opaque, hash, rexp, err := s.tokens.NewRefresh(ctx, user.ID, s.refreshTTL)
	if err != nil {
		return uuid.Nil, "", TokenPair{}, err
	}

	rt := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ClientID:  meta.ClientID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: rexp,
	}

	if err := s.refresh.Create(ctx, rt); err != nil {
		return uuid.Nil, "", TokenPair{}, err
	}

	pair := TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  aexp,
		RefreshOpaque:    opaque,
		RefreshExpiresAt: rexp,
		RefreshHash:      hash,
	}
	return user.ID, string(user.Role), pair, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshOpaqueHash string, meta ClientMeta) (TokenPair, error) {
	hash := util.Sha256Base64URL(refreshOpaqueHash)
	now := s.now()
	active, err := s.refresh.IsActiveByHash(ctx, hash, now)
	if err != nil {
		return TokenPair{}, err
	}
	if !active {
		return TokenPair{}, ErrTokenExpired
	}
	rt, err := s.refresh.GetByHashOnly(ctx, hash)
	if err != nil || rt == nil {
		if err == nil {
			err = ErrNotFound
		}
		return TokenPair{}, err
	}

	user, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil || user == nil {
		if err == nil {
			err = ErrNotFound
		}
		return TokenPair{}, err
	}

	if _, err := s.refresh.RevokeByHashOnly(ctx, hash); err != nil {
		return TokenPair{}, err
	}

	access, aexp, err := s.tokens.SignAccess(ctx, user.ID, string(user.Role), s.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}

	opaqueNew, hashNew, rexp, err := s.tokens.NewRefresh(ctx, user.ID, s.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	newRt := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashNew,
		ClientID:  meta.ClientID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: rexp,
		Revoked:   false,
		CreatedAt: now,
	}
	if err := s.refresh.Create(ctx, newRt); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  aexp,
		RefreshOpaque:    opaqueNew,
		RefreshExpiresAt: rexp,
		RefreshHash:      hashNew,
	}, nil
}

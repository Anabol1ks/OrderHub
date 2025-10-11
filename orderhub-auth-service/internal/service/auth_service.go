package service

import (
	"auth-service/internal/models"
	"auth-service/internal/util"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nanorand/nanorand"
	"go.uber.org/zap"
)

type AuthService struct {
	users             UserRepo
	refresh           RefreshRepo
	jwks              JWKRepo // может быть nil при HS256
	hasher            PasswordHasher
	tokens            TokenProvider
	sessions          SessionRepo
	passwordReset     PasswordResetRepo
	emailVerification EmailVerificationRepo

	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time

	log *zap.Logger
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
	sessions SessionRepo,
	passwordReset PasswordResetRepo,
	emailVerification EmailVerificationRepo,
	accessTTL, refreshTTL time.Duration,
	log *zap.Logger,
) *AuthService {
	return &AuthService{
		users:             users,
		refresh:           refresh,
		jwks:              jwks,
		hasher:            hasher,
		tokens:            tokens,
		sessions:          sessions,
		passwordReset:     passwordReset,
		emailVerification: emailVerification,

		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
		log:        log,
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

	rng, err := nanorand.Gen(10)
	if err != nil {
		return nil, err
	}

	codeHash := util.Sha256Base64URL(rng)

	emailVer := models.EmailVerification{
		UserID:    u.ID,
		Email:     u.Email,
		CodeHash:  codeHash,
		ExpiresAt: s.now().Add(24 * time.Hour),
	}

	if err := s.emailVerification.Create(ctx, &emailVer); err != nil {
		return nil, err
	}

	// TODO: ОТПРАВКА УВЕДОМЛЕНИЯ О ПОДТВЕРЖДЕНИИ EMAIL
	s.log.Info("Код подтверждения почты", zap.String("code", rng))

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
	var clientID string
	if meta.ClientID != nil {
		clientID = *meta.ClientID
	} else {
		clientID = "" // или сгенерировать uuid.NewString()
	}

	session := &models.UserSession{
		UserID:     user.ID,
		ClientID:   clientID,
		IP:         meta.IP,
		UserAgent:  meta.UserAgent,
		CreatedAt:  s.now(),
		LastSeenAt: s.now(),
		Revoked:    false,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return uuid.Nil, "", TokenPair{}, err
	}

	rt := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ClientID:  meta.ClientID,
		SessionID: &session.ID,
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
		SessionID: rt.SessionID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: rexp,
		Revoked:   false,
		CreatedAt: now,
	}
	if rt.SessionID != nil {
		_ = s.sessions.Touch(ctx, *rt.SessionID, now)
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

func (s *AuthService) Logout(ctx context.Context, opaque string) error {
	if opaque == "" {
		return errors.New("empty refresh token")
	}
	hash := util.Sha256Base64URL(opaque)

	rt, err := s.refresh.GetByHashOnly(ctx, hash)
	if err != nil || rt == nil {
		if err == nil { // не найден
			return ErrTokenNotFoundOrRevoked
		}
		return err
	}

	// Ревокуем сам refresh
	if ok, err := s.refresh.RevokeByHashOnly(ctx, hash); err != nil {
		return err
	} else if !ok {
		return ErrTokenNotFoundOrRevoked
	}

	if rt.SessionID != nil {
		stillActive, err := s.refresh.HasActiveBySession(ctx, *rt.SessionID, s.now())
		if err != nil {
			return err
		}
		if !stillActive {
			_, _ = s.sessions.Revoke(ctx, *rt.SessionID)
		}
	}
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context) (int64, error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok || userID == uuid.Nil {
		return 0, errors.New("unauthenticated: user id not found in context")
	}
	affected, err := s.refresh.RevokeAll(ctx, userID)
	if err != nil {
		return 0, err
	}
	if s.sessions != nil {
		_, _ = s.sessions.RevokeAllByUser(ctx, userID)
	}
	return affected, nil
}

func (s *AuthService) GetJwks(ctx context.Context) ([]PublicJWK, error) {
	// если используешь RSAProvider — просто читай из repo
	return s.jwks.ListPublic(ctx)
}

func (s *AuthService) Introspect(ctx context.Context, access string) (bool, uuid.UUID, string, time.Time, error) {
	claims, err := s.tokens.ParseAndValidateAccess(ctx, access)
	if err != nil {
		// недействителен: active=false
		return false, uuid.Nil, "", time.Time{}, nil
	}
	return true, claims.UserID, claims.Role, claims.Exp, nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return ErrNotFound
	}

	latest, err := s.passwordReset.FindLatestByUser(ctx, u.ID)
	if err == nil && latest != nil {
		cooldownDuration := time.Minute
		if s.now().Sub(latest.CreatedAt) < cooldownDuration {
			return ErrTooManyRequests
		}
	}

	rng, err := nanorand.Gen(6)
	if err != nil {
		return err
	}

	codeHash := util.Sha256Base64URL(rng)
	// TODO: ВРЕМЕННЫЙ ВЫВОД, УБРАТЬ ПОСЛЕ СЕРВИСА УВЕДОМЛЕНИЙ
	s.log.Info("Код сброса пароля: ", zap.String("code", rng))

	expiresAt := s.now().Add(1 * time.Hour)

	passwordReset := &models.PasswordResetToken{
		UserID:    u.ID,
		CodeHash:  codeHash,
		Email:     email,
		ExpiresAt: expiresAt,
		Consumed:  false,
	}

	if err := s.passwordReset.Create(ctx, passwordReset); err != nil {
		return err
	}

	// TODO: ТУТ НАДО БУДЕТ ДОБАВИТЬ ОТПРАВКУ EMAIL С КОДОМ

	return nil
}

func (s *AuthService) ConfirmPasswordReset(ctx context.Context, code, newPassword string) error {
	codeHash := util.Sha256Base64URL(code)

	passwordReset, err := s.passwordReset.GetValidByHash(ctx, codeHash, s.now())
	if err != nil {
		return ErrInvalidOrExpiredCode
	}

	user, err := s.users.GetByID(ctx, passwordReset.UserID)
	if err != nil || user == nil {
		return ErrNotFound
	}

	newPasswordHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}

	user.Password = newPasswordHash
	if err := s.users.UpdatePassword(ctx, user); err != nil {
		return err
	}

	if _, err := s.passwordReset.Consume(ctx, passwordReset.ID.String()); err != nil {
		s.log.Info("Failed to consume password reset token: ", zap.Error(err))
	}

	if _, err := s.refresh.RevokeAll(ctx, user.ID); err != nil {
		s.log.Info("Failed to revoke refresh tokens: ", zap.Error(err))
	}

	if _, err := s.sessions.RevokeAllByUser(ctx, user.ID); err != nil {
		s.log.Info("Failed to revoke session tokens: ", zap.Error(err))
	}

	if _, err := s.passwordReset.DeleteAllForUser(ctx, user.ID.String()); err != nil {
		s.log.Info("Failed to delete password reset tokens: ", zap.Error(err))
	}

	return nil
}

func (s *AuthService) RequestEmailVerification(ctx context.Context) error {
	userID, ok := UserIDFromContext(ctx)
	if !ok || userID == uuid.Nil {
		return errors.New("unauthenticated: user id not found in context")
	}

	// Получаем пользователя по ID из контекста
	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u == nil {
		return ErrNotFound
	}

	// Если email уже подтверждён
	if u.IsEmailVerified {
		return ErrEmailAlreadyVerified
	}

	latest, err := s.emailVerification.FindLatestByUser(ctx, u.ID)
	if err == nil && latest != nil {
		cooldownDuration := time.Minute
		if s.now().Sub(latest.CreatedAt) < cooldownDuration {
			return ErrTooManyRequests
		}
	}

	rng, err := nanorand.Gen(10)
	if err != nil {
		return err
	}

	codeHash := util.Sha256Base64URL(rng)
	s.log.Info("Код подтверждения почты", zap.String("code", rng))

	expiresAt := s.now().Add(24 * time.Hour)

	emailVerification := &models.EmailVerification{
		UserID:    u.ID,
		CodeHash:  codeHash,
		Email:     u.Email, // берём email пользователя
		ExpiresAt: expiresAt,
		Consumed:  false,
		CreatedAt: s.now(),
	}

	if err := s.emailVerification.Create(ctx, emailVerification); err != nil {
		return err
	}

	// TODO: отправка через Kafka

	return nil
}

func (s *AuthService) RequestEmailVerificationByEmail(ctx context.Context, email string) error {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return ErrNotFound
	}

	if u.IsEmailVerified {
		return ErrEmailAlreadyVerified
	}

	latest, err := s.emailVerification.FindLatestByUser(ctx, u.ID)
	if err == nil && latest != nil {
		if !latest.Consumed && latest.ExpiresAt.After(s.now()) {
			return ErrEmailVerificationInProgress
		}

		cooldownDuration := time.Minute
		if s.now().Sub(latest.CreatedAt) < cooldownDuration {
			return ErrTooManyRequests
		}
	}

	rng, err := nanorand.Gen(10)
	if err != nil {
		return err
	}

	codeHash := util.Sha256Base64URL(rng)
	s.log.Info("Код подтверждения почты", zap.String("code", rng))

	expiresAt := s.now().Add(24 * time.Hour)

	emailVerification := &models.EmailVerification{
		UserID:    u.ID,
		CodeHash:  codeHash,
		Email:     email,
		ExpiresAt: expiresAt,
		Consumed:  false,
		CreatedAt: s.now(),
	}

	if err := s.emailVerification.Create(ctx, emailVerification); err != nil {
		return err
	}

	// TODO: ТУТ НАДО БУДЕТ ДОБАВИТЬ ОТПРАВКУ EMAIL С КОДОМ

	return nil
}

func (s *AuthService) ConfirmEmailVerificationRequest(ctx context.Context, code string) error {
	codeHash := util.Sha256Base64URL(code)

	emailVer, err := s.emailVerification.GetValidByHash(ctx, codeHash, s.now())
	if err != nil {
		return ErrInvalidOrExpiredCode
	}

	user, err := s.users.GetByID(ctx, emailVer.UserID)
	if err != nil || user == nil {
		return ErrNotFound
	}

	user.IsEmailVerified = true
	if err := s.users.UpdateIsEmailVerified(ctx, user); err != nil {
		return err
	}

	if _, err := s.emailVerification.Consume(ctx, emailVer.ID.String()); err != nil {
		s.log.Info("Failed to consume password reset token", zap.Error(err))
	}

	return nil
}

type ctxKey string

const (
	ctxUserIDKey ctxKey = "auth.user_id"
	ctxRoleKey   ctxKey = "auth.role"
)

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxUserIDKey, id)
}
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v := ctx.Value(ctxUserIDKey)
	if v == nil {
		return uuid.Nil, false
	}
	id, _ := v.(uuid.UUID)
	return id, id != uuid.Nil
}

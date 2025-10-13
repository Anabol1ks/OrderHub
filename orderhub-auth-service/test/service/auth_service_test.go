package service_test

import (
	"auth-service/internal/models"
	"auth-service/internal/producer"
	"auth-service/internal/service"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Моки для всех зависимостей AuthService

// MockUserRepo
type MockUserRepo struct {
	CreateFunc                func(ctx context.Context, u *models.User) error
	GetByEmailFunc            func(ctx context.Context, email string) (*models.User, error)
	GetByIDFunc               func(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdatePasswordFunc        func(ctx context.Context, user *models.User) error
	ExistsByEmailFunc         func(ctx context.Context, email string) (bool, error)
	UpdateIsEmailVerifiedFunc func(ctx context.Context, user *models.User) error
}

func (m *MockUserRepo) Create(ctx context.Context, u *models.User) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, u)
	}
	return nil
}

func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *MockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockUserRepo) UpdatePassword(ctx context.Context, user *models.User) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, user)
	}
	return nil
}

func (m *MockUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if m.ExistsByEmailFunc != nil {
		return m.ExistsByEmailFunc(ctx, email)
	}
	return false, nil
}

func (m *MockUserRepo) UpdateIsEmailVerified(ctx context.Context, user *models.User) error {
	if m.UpdateIsEmailVerifiedFunc != nil {
		return m.UpdateIsEmailVerifiedFunc(ctx, user)
	}
	return nil
}

// MockRefreshRepo
type MockRefreshRepo struct {
	CreateFunc             func(ctx context.Context, t *models.RefreshToken) error
	RevokeAllFunc          func(ctx context.Context, userID uuid.UUID) (int64, error)
	TouchFunc              func(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error
	GetByHashOnlyFunc      func(ctx context.Context, hash string) (*models.RefreshToken, error)
	IsActiveByHashFunc     func(ctx context.Context, hash string, now time.Time) (bool, error)
	RevokeByHashOnlyFunc   func(ctx context.Context, hash string) (bool, error)
	HasActiveBySessionFunc func(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error)
}

func (m *MockRefreshRepo) Create(ctx context.Context, t *models.RefreshToken) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, t)
	}
	return nil
}

func (m *MockRefreshRepo) RevokeAll(ctx context.Context, userID uuid.UUID) (int64, error) {
	if m.RevokeAllFunc != nil {
		return m.RevokeAllFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockRefreshRepo) Touch(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error {
	if m.TouchFunc != nil {
		return m.TouchFunc(ctx, userID, hash, at)
	}
	return nil
}

func (m *MockRefreshRepo) GetByHashOnly(ctx context.Context, hash string) (*models.RefreshToken, error) {
	if m.GetByHashOnlyFunc != nil {
		return m.GetByHashOnlyFunc(ctx, hash)
	}
	return nil, nil
}

func (m *MockRefreshRepo) IsActiveByHash(ctx context.Context, hash string, now time.Time) (bool, error) {
	if m.IsActiveByHashFunc != nil {
		return m.IsActiveByHashFunc(ctx, hash, now)
	}
	return false, nil
}

func (m *MockRefreshRepo) RevokeByHashOnly(ctx context.Context, hash string) (bool, error) {
	if m.RevokeByHashOnlyFunc != nil {
		return m.RevokeByHashOnlyFunc(ctx, hash)
	}
	return false, nil
}

func (m *MockRefreshRepo) HasActiveBySession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	if m.HasActiveBySessionFunc != nil {
		return m.HasActiveBySessionFunc(ctx, sessionID, now)
	}
	return false, nil
}

// MockJWKRepo
type MockJWKRepo struct {
	ListPublicFunc func(ctx context.Context) ([]service.PublicJWK, error)
}

func (m *MockJWKRepo) ListPublic(ctx context.Context) ([]service.PublicJWK, error) {
	if m.ListPublicFunc != nil {
		return m.ListPublicFunc(ctx)
	}
	return []service.PublicJWK{}, nil
}

// MockPasswordHasher
type MockPasswordHasher struct {
	HashFunc    func(password string) (string, error)
	CompareFunc func(hash, password string) bool
}

func (m *MockPasswordHasher) Hash(password string) (string, error) {
	if m.HashFunc != nil {
		return m.HashFunc(password)
	}
	return "hashed_" + password, nil
}

func (m *MockPasswordHasher) Compare(hash, password string) bool {
	if m.CompareFunc != nil {
		return m.CompareFunc(hash, password)
	}
	return hash == "hashed_"+password
}

// MockTokenProvider
type MockTokenProvider struct {
	SignAccessFunc             func(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (string, time.Time, error)
	NewRefreshFunc             func(ctx context.Context, sub uuid.UUID, ttl time.Duration) (string, string, time.Time, error)
	ParseAndValidateAccessFunc func(ctx context.Context, token string) (*service.Claims, error)
	ListPublicJWKFunc          func(ctx context.Context) ([]service.PublicJWK, error)
}

func (m *MockTokenProvider) SignAccess(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (string, time.Time, error) {
	if m.SignAccessFunc != nil {
		return m.SignAccessFunc(ctx, sub, role, ttl)
	}
	exp := time.Now().Add(ttl)
	return "access_token", exp, nil
}

func (m *MockTokenProvider) NewRefresh(ctx context.Context, sub uuid.UUID, ttl time.Duration) (string, string, time.Time, error) {
	if m.NewRefreshFunc != nil {
		return m.NewRefreshFunc(ctx, sub, ttl)
	}
	exp := time.Now().Add(ttl)
	return "refresh_opaque", "refresh_hash", exp, nil
}

func (m *MockTokenProvider) ParseAndValidateAccess(ctx context.Context, token string) (*service.Claims, error) {
	if m.ParseAndValidateAccessFunc != nil {
		return m.ParseAndValidateAccessFunc(ctx, token)
	}
	return &service.Claims{
		UserID: uuid.New(),
		Role:   "ROLE_CUSTOMER",
		Exp:    time.Now().Add(time.Hour),
	}, nil
}

func (m *MockTokenProvider) ListPublicJWK(ctx context.Context) ([]service.PublicJWK, error) {
	if m.ListPublicJWKFunc != nil {
		return m.ListPublicJWKFunc(ctx)
	}
	return []service.PublicJWK{}, nil
}

// MockSessionRepo
type MockSessionRepo struct {
	CreateFunc           func(ctx context.Context, s *models.UserSession) error
	TouchFunc            func(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeFunc           func(ctx context.Context, id uuid.UUID) (bool, error)
	RevokeAllByUserFunc  func(ctx context.Context, userID uuid.UUID) (int64, error)
	ListActiveByUserFunc func(ctx context.Context, userID uuid.UUID, since time.Time) ([]models.UserSession, error)
}

func (m *MockSessionRepo) Create(ctx context.Context, s *models.UserSession) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, s)
	}
	return nil
}

func (m *MockSessionRepo) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	if m.TouchFunc != nil {
		return m.TouchFunc(ctx, id, at)
	}
	return nil
}

func (m *MockSessionRepo) Revoke(ctx context.Context, id uuid.UUID) (bool, error) {
	if m.RevokeFunc != nil {
		return m.RevokeFunc(ctx, id)
	}
	return true, nil
}

func (m *MockSessionRepo) RevokeAllByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	if m.RevokeAllByUserFunc != nil {
		return m.RevokeAllByUserFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockSessionRepo) ListActiveByUser(ctx context.Context, userID uuid.UUID, since time.Time) ([]models.UserSession, error) {
	if m.ListActiveByUserFunc != nil {
		return m.ListActiveByUserFunc(ctx, userID, since)
	}
	return []models.UserSession{}, nil
}

// MockPasswordResetRepo
type MockPasswordResetRepo struct {
	CreateFunc           func(ctx context.Context, t *models.PasswordResetToken) error
	GetValidByHashFunc   func(ctx context.Context, codeHash string, now time.Time) (*models.PasswordResetToken, error)
	ConsumeFunc          func(ctx context.Context, id string) (bool, error)
	DeleteAllForUserFunc func(ctx context.Context, userID string) (int64, error)
	FindLatestByUserFunc func(ctx context.Context, userID uuid.UUID) (*models.PasswordResetToken, error)
}

func (m *MockPasswordResetRepo) Create(ctx context.Context, t *models.PasswordResetToken) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, t)
	}
	return nil
}

func (m *MockPasswordResetRepo) GetValidByHash(ctx context.Context, codeHash string, now time.Time) (*models.PasswordResetToken, error) {
	if m.GetValidByHashFunc != nil {
		return m.GetValidByHashFunc(ctx, codeHash, now)
	}
	return nil, nil
}

func (m *MockPasswordResetRepo) Consume(ctx context.Context, id string) (bool, error) {
	if m.ConsumeFunc != nil {
		return m.ConsumeFunc(ctx, id)
	}
	return true, nil
}

func (m *MockPasswordResetRepo) DeleteAllForUser(ctx context.Context, userID string) (int64, error) {
	if m.DeleteAllForUserFunc != nil {
		return m.DeleteAllForUserFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockPasswordResetRepo) FindLatestByUser(ctx context.Context, userID uuid.UUID) (*models.PasswordResetToken, error) {
	if m.FindLatestByUserFunc != nil {
		return m.FindLatestByUserFunc(ctx, userID)
	}
	return nil, nil
}

// MockEmailVerificationRepo
type MockEmailVerificationRepo struct {
	CreateFunc           func(ctx context.Context, v *models.EmailVerification) error
	GetValidByHashFunc   func(ctx context.Context, codeHash string, now time.Time) (*models.EmailVerification, error)
	ConsumeFunc          func(ctx context.Context, id string) (bool, error)
	DeleteAllForUserFunc func(ctx context.Context, userID string) (int64, error)
	FindLatestByUserFunc func(ctx context.Context, userID uuid.UUID) (*models.EmailVerification, error)
}

func (m *MockEmailVerificationRepo) Create(ctx context.Context, v *models.EmailVerification) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, v)
	}
	return nil
}

func (m *MockEmailVerificationRepo) GetValidByHash(ctx context.Context, codeHash string, now time.Time) (*models.EmailVerification, error) {
	if m.GetValidByHashFunc != nil {
		return m.GetValidByHashFunc(ctx, codeHash, now)
	}
	return nil, nil
}

func (m *MockEmailVerificationRepo) Consume(ctx context.Context, id string) (bool, error) {
	if m.ConsumeFunc != nil {
		return m.ConsumeFunc(ctx, id)
	}
	return true, nil
}

func (m *MockEmailVerificationRepo) DeleteAllForUser(ctx context.Context, userID string) (int64, error) {
	if m.DeleteAllForUserFunc != nil {
		return m.DeleteAllForUserFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockEmailVerificationRepo) FindLatestByUser(ctx context.Context, userID uuid.UUID) (*models.EmailVerification, error) {
	if m.FindLatestByUserFunc != nil {
		return m.FindLatestByUserFunc(ctx, userID)
	}
	return nil, nil
}

// MockCacheClient
type MockCacheClient struct {
	SetRateLimitFunc       func(ctx context.Context, key string, ttl time.Duration) error
	CheckRateLimitFunc     func(ctx context.Context, key string) (bool, error)
	SetJWKFunc             func(ctx context.Context, kid string, jwkData []byte, ttl time.Duration) error
	GetJWKFunc             func(ctx context.Context, kid string) ([]byte, error)
	BlacklistTokenFunc     func(ctx context.Context, jti string, ttl time.Duration) error
	IsTokenBlacklistedFunc func(ctx context.Context, jti string) (bool, error)
	SetFunc                func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	GetFunc                func(ctx context.Context, key string) (string, error)
	DelFunc                func(ctx context.Context, keys ...string) error
}

// MockEmailProducer
type MockEmailProducer struct {
	SendEmailFunc func(ctx context.Context, to string, message producer.EmailMessage) error
}

func (m *MockEmailProducer) SendEmail(ctx context.Context, to string, message producer.EmailMessage) error {
	if m.SendEmailFunc != nil {
		return m.SendEmailFunc(ctx, to, message)
	}
	return nil
}

func (m *MockCacheClient) SetRateLimit(ctx context.Context, key string, ttl time.Duration) error {
	if m.SetRateLimitFunc != nil {
		return m.SetRateLimitFunc(ctx, key, ttl)
	}
	return nil
}

func (m *MockCacheClient) CheckRateLimit(ctx context.Context, key string) (bool, error) {
	if m.CheckRateLimitFunc != nil {
		return m.CheckRateLimitFunc(ctx, key)
	}
	return false, nil
}

func (m *MockCacheClient) SetJWK(ctx context.Context, kid string, jwkData []byte, ttl time.Duration) error {
	if m.SetJWKFunc != nil {
		return m.SetJWKFunc(ctx, kid, jwkData, ttl)
	}
	return nil
}

func (m *MockCacheClient) GetJWK(ctx context.Context, kid string) ([]byte, error) {
	if m.GetJWKFunc != nil {
		return m.GetJWKFunc(ctx, kid)
	}
	return nil, nil
}

func (m *MockCacheClient) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	if m.BlacklistTokenFunc != nil {
		return m.BlacklistTokenFunc(ctx, jti, ttl)
	}
	return nil
}

func (m *MockCacheClient) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	if m.IsTokenBlacklistedFunc != nil {
		return m.IsTokenBlacklistedFunc(ctx, jti)
	}
	return false, nil
}

func (m *MockCacheClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, ttl)
	}
	return nil
}

func (m *MockCacheClient) Get(ctx context.Context, key string) (string, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return "", nil
}

func (m *MockCacheClient) Del(ctx context.Context, keys ...string) error {
	if m.DelFunc != nil {
		return m.DelFunc(ctx, keys...)
	}
	return nil
}

// Вспомогательная функция для создания тестового AuthService
func createTestAuthService(
	userRepo *MockUserRepo,
	refreshRepo *MockRefreshRepo,
	jwkRepo *MockJWKRepo,
	hasher *MockPasswordHasher,
	tokens *MockTokenProvider,
	sessions *MockSessionRepo,
	passwordReset *MockPasswordResetRepo,
	emailVerification *MockEmailVerificationRepo,
	cache *MockCacheClient,
	emailProducer *MockEmailProducer,
) *service.AuthService {
	return service.NewAuthService(
		userRepo,
		refreshRepo,
		jwkRepo,
		hasher,
		tokens,
		sessions,
		passwordReset,
		emailVerification,
		cache,
		emailProducer,
		time.Hour,    // accessTTL
		24*time.Hour, // refreshTTL
		zap.NewNop(), // logger
	)
}

// Теперь начинаем писать тесты

func TestAuthService_Register_Success(t *testing.T) {
	userRepo := &MockUserRepo{}
	emailVerificationRepo := &MockEmailVerificationRepo{}
	hasher := &MockPasswordHasher{}

	// Настраиваем моки
	userRepo.ExistsByEmailFunc = func(ctx context.Context, email string) (bool, error) {
		return false, nil // email не существует
	}

	userRepo.CreateFunc = func(ctx context.Context, u *models.User) error {
		if u.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", u.Email)
		}
		if u.Password != "hashed_password123" {
			t.Errorf("Expected hashed password, got %s", u.Password)
		}
		return nil
	}

	emailVerificationRepo.CreateFunc = func(ctx context.Context, v *models.EmailVerification) error {
		if v.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", v.Email)
		}
		return nil
	}

	authService := createTestAuthService(
		userRepo, nil, nil, hasher, nil, nil, nil, emailVerificationRepo, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	user, err := authService.Register(ctx, "test@example.com", "password123", "ROLE_CUSTOMER")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", user.Email)
	}

	if user.Role != "ROLE_CUSTOMER" {
		t.Errorf("Expected role ROLE_CUSTOMER, got %s", user.Role)
	}

	if user.IsEmailVerified {
		t.Error("Expected IsEmailVerified to be false")
	}
}

func TestAuthService_Register_EmailExists(t *testing.T) {
	userRepo := &MockUserRepo{}

	// Email уже существует
	userRepo.ExistsByEmailFunc = func(ctx context.Context, email string) (bool, error) {
		return true, nil
	}

	authService := createTestAuthService(
		userRepo, nil, nil, nil, nil, nil, nil, nil, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	_, err := authService.Register(ctx, "test@example.com", "password123", "ROLE_CUSTOMER")

	if err == nil {
		t.Fatal("Expected error for existing email, got nil")
	}

	if err != service.ErrEmailExists {
		t.Errorf("Expected ErrEmailExists, got %v", err)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	userRepo := &MockUserRepo{}
	tokens := &MockTokenProvider{}
	sessions := &MockSessionRepo{}
	refreshRepo := &MockRefreshRepo{}
	hasher := &MockPasswordHasher{}

	userID := uuid.New()

	// Настраиваем моки
	userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{
			ID:       userID,
			Email:    "test@example.com",
			Password: "hashed_password123",
			Role:     "ROLE_CUSTOMER",
		}, nil
	}

	tokens.SignAccessFunc = func(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (string, time.Time, error) {
		exp := time.Now().Add(ttl)
		return "access_token", exp, nil
	}

	tokens.NewRefreshFunc = func(ctx context.Context, sub uuid.UUID, ttl time.Duration) (string, string, time.Time, error) {
		exp := time.Now().Add(ttl)
		return "refresh_opaque", "refresh_hash", exp, nil
	}

	sessions.CreateFunc = func(ctx context.Context, s *models.UserSession) error {
		if s.UserID != userID {
			t.Errorf("Expected userID %v, got %v", userID, s.UserID)
		}
		return nil
	}

	refreshRepo.CreateFunc = func(ctx context.Context, token *models.RefreshToken) error {
		if token.UserID != userID {
			t.Errorf("Expected userID %v, got %v", userID, token.UserID)
		}
		return nil
	}

	authService := createTestAuthService(
		userRepo, refreshRepo, nil, hasher, tokens, sessions, nil, nil, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	clientMeta := service.ClientMeta{
		IP:        stringPtr("127.0.0.1"),
		UserAgent: stringPtr("test-agent"),
	}

	loggedUserID, role, tokenPair, err := authService.Login(ctx, "test@example.com", "password123", clientMeta)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loggedUserID != userID {
		t.Errorf("Expected userID %v, got %v", userID, loggedUserID)
	}

	if role != "ROLE_CUSTOMER" {
		t.Errorf("Expected role ROLE_CUSTOMER, got %s", role)
	}

	if tokenPair.AccessToken != "access_token" {
		t.Errorf("Expected access_token, got %s", tokenPair.AccessToken)
	}
}

func TestAuthService_Login_InvalidCredentials(t *testing.T) {
	userRepo := &MockUserRepo{}
	hasher := &MockPasswordHasher{}

	// Пользователь не найден
	userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
		return nil, nil
	}

	authService := createTestAuthService(
		userRepo, nil, nil, hasher, nil, nil, nil, nil, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	clientMeta := service.ClientMeta{}

	_, _, _, err := authService.Login(ctx, "test@example.com", "password123", clientMeta)

	if err == nil {
		t.Fatal("Expected error for invalid credentials, got nil")
	}

	if err != service.ErrInvalidCredentials {
		t.Errorf("Expected ErrInvalidCredentials, got %v", err)
	}
}

// Вспомогательная функция
func stringPtr(s string) *string {
	return &s
}

func TestAuthService_Refresh_Success(t *testing.T) {
	refreshRepo := &MockRefreshRepo{}
	tokens := &MockTokenProvider{}
	sessions := &MockSessionRepo{}
	cache := &MockCacheClient{}
	userRepo := &MockUserRepo{}

	userID := uuid.New()
	sessionID := uuid.New()

	// Настраиваем моки
	refreshRepo.IsActiveByHashFunc = func(ctx context.Context, hash string, now time.Time) (bool, error) {
		return true, nil // Токен активен
	}

	refreshRepo.GetByHashOnlyFunc = func(ctx context.Context, hash string) (*models.RefreshToken, error) {
		return &models.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			SessionID: &sessionID,
			TokenHash: "refresh_hash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Revoked:   false,
		}, nil
	}

	userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
		return &models.User{
			ID:   userID,
			Role: "ROLE_CUSTOMER",
		}, nil
	}

	refreshRepo.TouchFunc = func(ctx context.Context, uid uuid.UUID, hash string, at time.Time) error {
		if uid != userID {
			t.Errorf("Expected userID %v, got %v", userID, uid)
		}
		return nil
	}

	sessions.TouchFunc = func(ctx context.Context, id uuid.UUID, at time.Time) error {
		if id != sessionID {
			t.Errorf("Expected sessionID %v, got %v", sessionID, id)
		}
		return nil
	}

	tokens.SignAccessFunc = func(ctx context.Context, sub uuid.UUID, role string, ttl time.Duration) (string, time.Time, error) {
		if sub != userID {
			t.Errorf("Expected userID %v, got %v", userID, sub)
		}
		exp := time.Now().Add(ttl)
		return "new_access_token", exp, nil
	}

	tokens.NewRefreshFunc = func(ctx context.Context, sub uuid.UUID, ttl time.Duration) (string, string, time.Time, error) {
		exp := time.Now().Add(ttl)
		return "new_refresh_opaque", "new_refresh_hash", exp, nil
	}

	refreshRepo.RevokeByHashOnlyFunc = func(ctx context.Context, hash string) (bool, error) {
		return true, nil
	}

	refreshRepo.CreateFunc = func(ctx context.Context, token *models.RefreshToken) error {
		if token.UserID != userID {
			t.Errorf("Expected userID %v, got %v", userID, token.UserID)
		}
		return nil
	}

	authService := createTestAuthService(
		userRepo, refreshRepo, nil, nil, tokens, sessions, nil, nil, cache, &MockEmailProducer{},
	)

	ctx := context.Background()
	clientMeta := service.ClientMeta{
		IP:        stringPtr("127.0.0.1"),
		UserAgent: stringPtr("test-agent"),
	}

	tokenPair, err := authService.Refresh(ctx, "refresh_hash", clientMeta)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tokenPair.AccessToken != "new_access_token" {
		t.Errorf("Expected new_access_token, got %s", tokenPair.AccessToken)
	}

	if tokenPair.RefreshOpaque != "new_refresh_opaque" {
		t.Errorf("Expected new_refresh_opaque, got %s", tokenPair.RefreshOpaque)
	}
}

func TestAuthService_Refresh_TokenNotFound(t *testing.T) {
	refreshRepo := &MockRefreshRepo{}

	// Токен не активен
	refreshRepo.IsActiveByHashFunc = func(ctx context.Context, hash string, now time.Time) (bool, error) {
		return false, nil // Токен не активен
	}

	authService := createTestAuthService(
		nil, refreshRepo, nil, nil, nil, nil, nil, nil, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	clientMeta := service.ClientMeta{}

	_, err := authService.Refresh(ctx, "invalid_hash", clientMeta)

	if err == nil {
		t.Fatal("Expected error for invalid refresh token, got nil")
	}

	if err != service.ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}
}

func TestAuthService_LogoutWithAccessToken_Success(t *testing.T) {
	refreshRepo := &MockRefreshRepo{}
	tokens := &MockTokenProvider{}
	sessions := &MockSessionRepo{}
	cache := &MockCacheClient{}

	userID := uuid.New()
	sessionID := uuid.New()

	// Настраиваем моки
	tokens.ParseAndValidateAccessFunc = func(ctx context.Context, token string) (*service.Claims, error) {
		return &service.Claims{
			UserID: userID,
			Role:   "ROLE_CUSTOMER",
			Exp:    time.Now().Add(time.Hour),
		}, nil
	}

	refreshRepo.GetByHashOnlyFunc = func(ctx context.Context, hash string) (*models.RefreshToken, error) {
		return &models.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			SessionID: &sessionID,
			TokenHash: "refresh_hash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Revoked:   false,
		}, nil
	}

	refreshRepo.TouchFunc = func(ctx context.Context, uid uuid.UUID, hash string, at time.Time) error {
		return nil
	}

	refreshRepo.RevokeByHashOnlyFunc = func(ctx context.Context, hash string) (bool, error) {
		return true, nil
	}

	sessions.RevokeFunc = func(ctx context.Context, id uuid.UUID) (bool, error) {
		if id != sessionID {
			t.Errorf("Expected sessionID %v, got %v", sessionID, id)
		}
		return true, nil
	}

	cache.BlacklistTokenFunc = func(ctx context.Context, jti string, ttl time.Duration) error {
		return nil
	}

	authService := createTestAuthService(
		nil, refreshRepo, nil, nil, tokens, sessions, nil, nil, cache, &MockEmailProducer{},
	)

	ctx := context.Background()
	err := authService.LogoutWithAccessToken(ctx, "refresh_opaque", "access_token")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAuthService_GetJwks_Success(t *testing.T) {
	jwkRepo := &MockJWKRepo{}

	// Настраиваем мок
	jwkRepo.ListPublicFunc = func(ctx context.Context) ([]service.PublicJWK, error) {
		return []service.PublicJWK{
			{
				KID: "key1",
				Alg: "RS256",
				Kty: "RSA",
				Use: "sig",
				N:   "test_n",
				E:   "AQAB",
			},
		}, nil
	}

	authService := createTestAuthService(
		nil, nil, jwkRepo, nil, nil, nil, nil, nil, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	jwks, err := authService.GetJwks(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(jwks) != 1 {
		t.Errorf("Expected 1 JWK, got %d", len(jwks))
	}

	if jwks[0].KID != "key1" {
		t.Errorf("Expected KID key1, got %s", jwks[0].KID)
	}
}

func TestAuthService_RequestPasswordReset_Success(t *testing.T) {
	userRepo := &MockUserRepo{}
	passwordResetRepo := &MockPasswordResetRepo{}
	cache := &MockCacheClient{}

	userID := uuid.New()

	// Настраиваем моки
	cache.CheckRateLimitFunc = func(ctx context.Context, key string) (bool, error) {
		return false, nil // Нет rate limiting
	}

	userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
		return &models.User{
			ID:    userID,
			Email: "test@example.com",
		}, nil
	}

	passwordResetRepo.FindLatestByUserFunc = func(ctx context.Context, uid uuid.UUID) (*models.PasswordResetToken, error) {
		// Возвращаем nil, что означает что активных запросов нет
		return nil, nil
	}

	passwordResetRepo.CreateFunc = func(ctx context.Context, token *models.PasswordResetToken) error {
		if token.UserID != userID {
			t.Errorf("Expected userID %v, got %v", userID, token.UserID)
		}
		if token.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", token.Email)
		}
		return nil
	}

	cache.SetRateLimitFunc = func(ctx context.Context, key string, ttl time.Duration) error {
		return nil
	}

	authService := createTestAuthService(
		userRepo, nil, nil, nil, nil, nil, passwordResetRepo, nil, cache, &MockEmailProducer{},
	)

	ctx := context.Background()
	err := authService.RequestPasswordReset(ctx, "test@example.com")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAuthService_RequestPasswordReset_UserNotFound(t *testing.T) {
	userRepo := &MockUserRepo{}
	cache := &MockCacheClient{}
	passwordResetRepo := &MockPasswordResetRepo{}

	// Настраиваем моки
	cache.CheckRateLimitFunc = func(ctx context.Context, key string) (bool, error) {
		return false, nil // Нет rate limiting
	}

	// Пользователь не найден - GetByEmail возвращает error
	userRepo.GetByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
		return nil, errors.New("user not found")
	}

	authService := createTestAuthService(
		userRepo, nil, nil, nil, nil, nil, passwordResetRepo, nil, cache, &MockEmailProducer{},
	)

	ctx := context.Background()
	err := authService.RequestPasswordReset(ctx, "test@example.com")

	if err == nil {
		t.Fatal("Expected error for user not found, got nil")
	}

	if err != service.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestAuthService_ConfirmPasswordReset_Success(t *testing.T) {
	passwordResetRepo := &MockPasswordResetRepo{}
	userRepo := &MockUserRepo{}
	hasher := &MockPasswordHasher{}
	refreshRepo := &MockRefreshRepo{}
	sessions := &MockSessionRepo{}

	userID := uuid.New()
	tokenID := uuid.New()

	// Настраиваем моки
	passwordResetRepo.GetValidByHashFunc = func(ctx context.Context, codeHash string, now time.Time) (*models.PasswordResetToken, error) {
		return &models.PasswordResetToken{
			ID:        tokenID,
			UserID:    userID,
			Email:     "test@example.com",
			CodeHash:  codeHash,
			ExpiresAt: time.Now().Add(6 * time.Hour),
			Consumed:  false,
		}, nil
	}

	userRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
		return &models.User{
			ID:    userID,
			Email: "test@example.com",
		}, nil
	}

	userRepo.UpdatePasswordFunc = func(ctx context.Context, user *models.User) error {
		if user.ID != userID {
			t.Errorf("Expected userID %v, got %v", userID, user.ID)
		}
		if user.Password != "hashed_newpassword123" {
			t.Errorf("Expected hashed password, got %s", user.Password)
		}
		return nil
	}

	passwordResetRepo.ConsumeFunc = func(ctx context.Context, id string) (bool, error) {
		if id != tokenID.String() {
			t.Errorf("Expected tokenID %s, got %s", tokenID.String(), id)
		}
		return true, nil
	}

	refreshRepo.RevokeAllFunc = func(ctx context.Context, uid uuid.UUID) (int64, error) {
		if uid != userID {
			t.Errorf("Expected userID %v, got %v", userID, uid)
		}
		return 2, nil // Ревокнули 2 токена
	}

	sessions.RevokeAllByUserFunc = func(ctx context.Context, uid uuid.UUID) (int64, error) {
		if uid != userID {
			t.Errorf("Expected userID %v, got %v", userID, uid)
		}
		return 1, nil // Ревокнули 1 сессию
	}

	authService := createTestAuthService(
		userRepo, refreshRepo, nil, hasher, nil, sessions, passwordResetRepo, nil, nil, &MockEmailProducer{},
	)

	ctx := context.Background()
	err := authService.ConfirmPasswordReset(ctx, "resetcode123", "newpassword123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAuthService_LogoutAll_Success(t *testing.T) {
	refreshRepo := &MockRefreshRepo{}
	sessions := &MockSessionRepo{}

	userID := uuid.New()
	ctx := service.WithUserID(context.Background(), userID)

	// Настраиваем моки
	refreshRepo.RevokeAllFunc = func(ctx context.Context, uid uuid.UUID) (int64, error) {
		if uid != userID {
			t.Errorf("Expected userID %v, got %v", userID, uid)
		}
		return 3, nil // Ревокнули 3 токена
	}

	sessions.RevokeAllByUserFunc = func(ctx context.Context, uid uuid.UUID) (int64, error) {
		if uid != userID {
			t.Errorf("Expected userID %v, got %v", userID, uid)
		}
		return 2, nil // Ревокнули 2 сессии
	}

	authService := createTestAuthService(
		nil, refreshRepo, nil, nil, nil, sessions, nil, nil, nil, &MockEmailProducer{},
	)

	count, err := authService.LogoutAll(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestAuthService_Introspect_Success(t *testing.T) {
	tokens := &MockTokenProvider{}
	cache := &MockCacheClient{}

	userID := uuid.New()

	// Настраиваем моки
	tokens.ParseAndValidateAccessFunc = func(ctx context.Context, token string) (*service.Claims, error) {
		return &service.Claims{
			UserID: userID,
			Role:   "ROLE_CUSTOMER",
			Exp:    time.Now().Add(time.Hour),
		}, nil
	}

	cache.IsTokenBlacklistedFunc = func(ctx context.Context, jti string) (bool, error) {
		return false, nil // Токен не в блэклисте
	}

	authService := createTestAuthService(
		nil, nil, nil, nil, tokens, nil, nil, nil, cache, &MockEmailProducer{},
	)

	ctx := context.Background()
	valid, resultUserID, role, exp, err := authService.Introspect(ctx, "access_token")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !valid {
		t.Error("Expected token to be valid")
	}

	if resultUserID != userID {
		t.Errorf("Expected userID %v, got %v", userID, resultUserID)
	}

	if role != "ROLE_CUSTOMER" {
		t.Errorf("Expected role ROLE_CUSTOMER, got %s", role)
	}

	if exp.Before(time.Now()) {
		t.Error("Expected expiration time to be in the future")
	}
}

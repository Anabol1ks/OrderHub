package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"auth-service/internal/migrate"
	"auth-service/internal/models"
	"auth-service/internal/repository"

	"orderhub-utils-go/testutil"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestUserRepo(t *testing.T) {
	db := testutil.SetupTestPostgres(t)

	// Запускаем миграцию явно в тесте
	if err := migrate.MigrateAuthDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	repo := repository.NewUserRepo(db)

	ctx := context.Background()

	user := models.User{
		Email:    "test@example.com",
		Password: "password",
	}
	if err := repo.Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// проверка на уникальность
	if err := repo.Create(ctx, &user); err == nil {
		t.Fatal("expected unique constraint error, got nil")
	}

	_, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get user by ID: %v", err)
	}

	_, err = repo.GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}

	user.Password = "newpassword"

	err = repo.UpdatePassword(ctx, &user)
	if err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	// Проверяем что пароль обновился
	updatedUser, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get updated user: %v", err)
	}
	if updatedUser.Password != "newpassword" {
		t.Fatalf("password not updated: got %s, want newpassword", updatedUser.Password)
	}

	// Тестируем ExistsByEmail
	if exists, err := repo.ExistsByEmail(ctx, user.Email); err != nil {
		t.Fatalf("failed to check email existence: %v", err)
	} else if !exists {
		t.Fatal("expected email to exist")
	}

	if exists, err := repo.ExistsByEmail(ctx, "nonexistent@example.com"); err != nil {
		t.Fatalf("failed to check nonexistent email: %v", err)
	} else if exists {
		t.Fatal("expected email to not exist")
	}

	// Тестируем UpdateIsEmailVerified
	user.IsEmailVerified = true
	if err := repo.UpdateIsEmailVerified(ctx, &user); err != nil {
		t.Fatalf("failed to update email verification status: %v", err)
	}

	// Проверяем что статус обновился
	verifiedUser, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get verified user: %v", err)
	}
	if !verifiedUser.IsEmailVerified {
		t.Fatal("expected user to be email verified")
	}

	// Тестируем case-insensitive поиск по email
	_, err = repo.GetByEmail(ctx, strings.ToUpper(user.Email))
	if err != nil {
		t.Fatalf("failed to get user by email (case insensitive): %v", err)
	}
}

func TestRefreshRepo(t *testing.T) {
	db := testutil.SetupTestPostgres(t)

	// Запускаем миграцию явно в тесте
	if err := migrate.MigrateAuthDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	repo := repository.NewRefreshRepo(db)
	userRepo := repository.NewUserRepo(db)

	ctx := context.Background()

	u_id := uuid.New()
	user := models.User{
		ID:       u_id,
		Email:    "test@example.com",
		Password: "password",
	}
	if err := userRepo.Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	r := models.RefreshToken{
		UserID:    u_id,
		TokenHash: "hash_123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, &r); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	// Проверяем IsActiveByHash
	if active, err := repo.IsActiveByHash(ctx, "hash_123", time.Now()); err != nil {
		t.Fatalf("failed to check if refresh token is active: %v", err)
	} else if !active {
		t.Fatal("expected token to be active")
	}

	// Проверяем Touch - убеждаемся что обновляется last_used_at
	beforeTouch := time.Now()
	time.Sleep(100 * time.Millisecond) // Небольшая задержка
	touchTime := time.Now()

	if err := repo.Touch(ctx, u_id, "hash_123", touchTime); err != nil {
		t.Fatalf("failed to touch refresh token: %v", err)
	}

	// Проверяем что last_used_at обновился
	if token, err := repo.GetByHashOnly(ctx, "hash_123"); err != nil {
		t.Fatalf("failed to get refresh token after touch: %v", err)
	} else {
		if token.LastUsedAt == nil {
			t.Fatal("expected last_used_at to be set after touch")
		}
		if token.LastUsedAt.Before(beforeTouch) {
			t.Fatalf("last_used_at not updated properly: got %v, should be after %v",
				token.LastUsedAt, beforeTouch)
		}
	}

	// Проверяем GetByHashOnly
	if token, err := repo.GetByHashOnly(ctx, "hash_123"); err != nil {
		t.Fatalf("failed to get refresh token: %v", err)
	} else if token.TokenHash != "hash_123" {
		t.Fatalf("token hash mismatch: got %s, want %s", token.TokenHash, "hash_123")
	}

	// Проверяем RevokeByHashOnly
	if revoked, err := repo.RevokeByHashOnly(ctx, "hash_123"); err != nil {
		t.Fatalf("failed to revoke refresh token: %v", err)
	} else if !revoked {
		t.Fatal("expected token to be revoked")
	}

	// Проверяем, что токен больше не активен
	if active, err := repo.IsActiveByHash(ctx, "hash_123", time.Now()); err != nil {
		t.Fatalf("failed to check if refresh token is active: %v", err)
	} else if active {
		t.Fatal("expected token to be inactive after revoke")
	}

	// Создаём ещё несколько токенов для тестирования RevokeAll
	sessionID := uuid.New()
	r1 := models.RefreshToken{
		ID:        uuid.New(),
		UserID:    u_id,
		SessionID: &sessionID,
		TokenHash: "hash_456",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := repo.Create(ctx, &r1); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	r2 := models.RefreshToken{
		ID:        uuid.New(),
		UserID:    u_id,
		SessionID: &sessionID,
		TokenHash: "hash_789",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := repo.Create(ctx, &r2); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	// Проверяем HasActiveBySession
	if hasActive, err := repo.HasActiveBySession(ctx, *r1.SessionID, time.Now()); err != nil {
		t.Fatalf("failed to check HasActiveBySession: %v", err)
	} else if !hasActive {
		t.Fatal("expected to have active refresh tokens for session")
	}

	// Проверяем RevokeAll
	if count, err := repo.RevokeAll(ctx, u_id); err != nil {
		t.Fatalf("failed to revoke all refresh tokens: %v", err)
	} else if count != 2 {
		t.Fatalf("expected to revoke 2 tokens, got %d", count)
	}

	// Проверяем, что сессия больше не имеет активных токенов
	if hasActive, err := repo.HasActiveBySession(ctx, *r1.SessionID, time.Now()); err != nil {
		t.Fatalf("failed to check HasActiveBySession after revoke: %v", err)
	} else if hasActive {
		t.Fatal("expected no active refresh tokens for session after revoke all")
	}

	// Проверяем, что токены больше не активны
	if active, err := repo.IsActiveByHash(ctx, "hash_456", time.Now()); err != nil {
		t.Fatalf("failed to check if refresh token is active: %v", err)
	} else if active {
		t.Fatal("expected token to be inactive after revoke all")
	}

	if active, err := repo.IsActiveByHash(ctx, "hash_789", time.Now()); err != nil {
		t.Fatalf("failed to check if refresh token is active: %v", err)
	} else if active {
		t.Fatal("expected token to be inactive after revoke all")
	}
}

func TestPasswordResetRepo(t *testing.T) {
	db := testutil.SetupTestPostgres(t)

	// Запускаем миграцию явно в тесте
	if err := migrate.MigrateAuthDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()

	repo := repository.New(db)

	u := models.User{
		Email:    "test@example.com",
		Password: "password",
	}

	if err := repo.Users.Create(ctx, &u); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	pr := models.PasswordResetToken{
		UserID:    u.ID,
		CodeHash:  "reset_token_hash",
		ExpiresAt: time.Now().Add(6 * time.Hour),
		Email:     u.Email,
	}

	if err := repo.PasswordReset.Create(ctx, &pr); err != nil {
		t.Fatalf("failed to create password reset token: %v", err)
	}

	if getPr, err := repo.PasswordReset.GetValidByHash(ctx, pr.CodeHash, time.Now()); err != nil {
		t.Fatalf("failed to get valid password reset token: %v", err)
	} else {
		// Проверяем ключевые поля — даты сравниваем с небольшим допуском
		if getPr.UserID != pr.UserID {
			t.Fatalf("retrieved password reset token user id mismatch: got %v want %v", getPr.UserID, pr.UserID)
		}
		if getPr.CodeHash != pr.CodeHash {
			t.Fatalf("retrieved password reset token code hash mismatch: got %v want %v", getPr.CodeHash, pr.CodeHash)
		}
		if getPr.Email != pr.Email {
			t.Fatalf("retrieved password reset token email mismatch: got %v want %v", getPr.Email, pr.Email)
		}
		// допускаем небольшую погрешность времени (1 секунда)
		if !getPr.ExpiresAt.Equal(pr.ExpiresAt) {
			diff := getPr.ExpiresAt.Sub(pr.ExpiresAt)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				t.Fatalf("retrieved password reset token expires_at mismatch: got %v want %v (diff %v)", getPr.ExpiresAt, pr.ExpiresAt, diff)
			}
		}
	}

	pr2 := models.PasswordResetToken{
		UserID:    u.ID,
		CodeHash:  "reset_token_hash2",
		ExpiresAt: time.Now().Add(6 * time.Hour),
		Email:     u.Email,
	}

	if err := repo.PasswordReset.Create(ctx, &pr2); err != nil {
		t.Fatalf("failed to create password reset token: %v", err)
	}

	if _, err := repo.PasswordReset.Consume(ctx, pr2.ID.String()); err != nil {
		t.Fatalf("failed to consume password reset token: %v", err)
	}

	if c, err := repo.PasswordReset.DeleteAllForUser(ctx, u.ID.String()); err != nil {
		t.Fatalf("failed to delete all password reset tokens: %v", err)
	} else {
		// Проверяем, что количество удалённых токенов соответствует ожидаемому
		if c != 2 {
			t.Fatalf("expected to delete 2 password reset tokens, got %d", c)
		}
	}

	// Тест FindLatestByUser
	pr3 := models.PasswordResetToken{
		UserID:    u.ID,
		CodeHash:  "reset_token_hash3",
		ExpiresAt: time.Now().Add(6 * time.Hour),
		Email:     u.Email,
		CreatedAt: time.Now(),
	}
	if err := repo.PasswordReset.Create(ctx, &pr3); err != nil {
		t.Fatalf("failed to create password reset token: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Небольшая задержка

	pr4 := models.PasswordResetToken{
		UserID:    u.ID,
		CodeHash:  "reset_token_hash4",
		ExpiresAt: time.Now().Add(6 * time.Hour),
		Email:     u.Email,
		CreatedAt: time.Now(),
	}
	if err := repo.PasswordReset.Create(ctx, &pr4); err != nil {
		t.Fatalf("failed to create password reset token: %v", err)
	}

	// FindLatestByUser должен вернуть последний созданный
	if latest, err := repo.PasswordReset.FindLatestByUser(ctx, u.ID); err != nil {
		t.Fatalf("failed to find latest password reset token: %v", err)
	} else if latest.CodeHash != "reset_token_hash4" {
		t.Fatalf("expected latest code hash to be reset_token_hash4, got %s", latest.CodeHash)
	}
}

func TestJWKRepo(t *testing.T) {
	db := testutil.SetupTestPostgres(t)
	if err := migrate.MigrateAuthDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	repo := repository.NewJWKRepo(db)
	ctx := context.Background()

	k1 := models.JwkKey{KID: "k1", N: "n1", E: "e1", PrivPEM: []byte("priv1"), Active: true}
	if err := repo.Create(ctx, &k1); err != nil {
		t.Fatalf("create k1: %v", err)
	}

	// Active retrieval
	if active, err := repo.GetActive(ctx); err != nil {
		t.Fatalf("GetActive err: %v", err)
	} else if active.KID != "k1" {
		t.Fatalf("expected active k1 got %s", active.KID)
	}

	k2 := models.JwkKey{KID: "k2", N: "n2", E: "e2", PrivPEM: []byte("priv2"), Active: false}
	if err := repo.Create(ctx, &k2); err != nil {
		t.Fatalf("create k2: %v", err)
	}

	// SetActive should switch
	if err := repo.SetActive(ctx, "k2"); err != nil {
		t.Fatalf("SetActive k2: %v", err)
	}
	if active, err := repo.GetActive(ctx); err != nil {
		t.Fatalf("GetActive err: %v", err)
	} else if active.KID != "k2" {
		t.Fatalf("expected active k2 got %s", active.KID)
	}

	// GetByKID
	if by, err := repo.GetByKID(ctx, "k1"); err != nil {
		t.Fatalf("GetByKID k1: %v", err)
	} else if by.KID != "k1" {
		t.Fatalf("GetByKID mismatch got %s", by.KID)
	}

	// ListPublic should contain both
	pubs, err := repo.ListPublic(ctx)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	if len(pubs) != 2 {
		t.Fatalf("expected 2 public keys, got %d", len(pubs))
	}

	// Проверяем что активный ключ теперь k2
	if active, err := repo.GetActive(ctx); err != nil {
		t.Fatalf("get active after switch: %v", err)
	} else if active.KID != "k2" {
		t.Fatalf("expected active key to be k2, got %s", active.KID)
	}

	// Тестируем создание ключа с дублирующимся KID (должно быть ошибка)
	k3 := models.JwkKey{KID: "k2", N: "n3", E: "e3", PrivPEM: []byte("priv3"), Active: false}
	if err := repo.Create(ctx, &k3); err == nil {
		t.Fatal("expected error when creating key with duplicate KID")
	}

	// Тестируем SetActive с несуществующим KID
	if err := repo.SetActive(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error when setting nonexistent key as active")
	}
}

func TestSessionRepo(t *testing.T) {
	db := testutil.SetupTestPostgres(t)
	if err := migrate.MigrateAuthDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	ctx := context.Background()
	userRepo := repository.NewUserRepo(db)
	srepo := repository.NewSessionRepo(db)

	u := models.User{Email: "sess@example.com", Password: "pwd"}
	if err := userRepo.Create(ctx, &u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	s1 := models.UserSession{UserID: u.ID, ClientID: "c1"}
	if err := srepo.Create(ctx, &s1); err != nil {
		t.Fatalf("create session1: %v", err)
	}

	// Touch updates last_seen_at
	before := s1.LastSeenAt
	newTime := time.Now().Add(time.Second)
	if err := srepo.Touch(ctx, s1.ID, newTime); err != nil {
		t.Fatalf("touch: %v", err)
	}
	// reload
	list, err := srepo.ListActiveByUser(ctx, u.ID, time.Time{})
	if err != nil {
		t.Fatalf("list after touch: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session got %d", len(list))
	}
	if !list[0].LastSeenAt.After(before) {
		t.Fatalf("LastSeenAt not updated")
	}

	// Add second session, filter by since
	s2 := models.UserSession{UserID: u.ID, ClientID: "c2"}
	if err := srepo.Create(ctx, &s2); err != nil {
		t.Fatalf("create session2: %v", err)
	}

	since := time.Now().Add(-1 * time.Minute)
	activeList, err := srepo.ListActiveByUser(ctx, u.ID, since)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(activeList) != 2 {
		t.Fatalf("expected 2 active sessions got %d", len(activeList))
	}

	// Revoke first
	if ok, err := srepo.Revoke(ctx, s1.ID); err != nil || !ok {
		t.Fatalf("revoke s1 failed: %v ok=%v", err, ok)
	}
	activeList, _ = srepo.ListActiveByUser(ctx, u.ID, since)
	if len(activeList) != 1 {
		t.Fatalf("expected 1 active after revoke got %d", len(activeList))
	}

	// RevokeAllByUser
	if c, err := srepo.RevokeAllByUser(ctx, u.ID); err != nil {
		t.Fatalf("revoke all: %v", err)
	} else if c == 0 {
		t.Fatalf("expected revoke count >0")
	}
	activeList, _ = srepo.ListActiveByUser(ctx, u.ID, since)
	if len(activeList) != 0 {
		t.Fatalf("expected 0 active after revoke all got %d", len(activeList))
	}
}

func TestEmailVerificationRepo(t *testing.T) {
	db := testutil.SetupTestPostgres(t)
	if err := migrate.MigrateAuthDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	ctx := context.Background()
	userRepo := repository.NewUserRepo(db)
	erepo := repository.NewEmailVerificationRepo(db)

	u := models.User{Email: "verify@example.com", Password: "pwd"}
	if err := userRepo.Create(ctx, &u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	ev := models.EmailVerification{UserID: u.ID, Email: u.Email, CodeHash: "code1", ExpiresAt: time.Now().Add(10 * time.Minute)}
	if err := erepo.Create(ctx, &ev); err != nil {
		t.Fatalf("create ev: %v", err)
	}

	if got, err := erepo.GetValidByHash(ctx, "code1", time.Now()); err != nil {
		t.Fatalf("GetValid: %v", err)
	} else if got.CodeHash != ev.CodeHash {
		t.Fatalf("GetValid mismatch")
	}

	// Consume
	if ok, err := erepo.Consume(ctx, ev.ID.String()); err != nil || !ok {
		t.Fatalf("consume failed: %v ok=%v", err, ok)
	}
	if got, err := erepo.GetValidByHash(ctx, "code1", time.Now()); err == nil || got != nil {
		t.Fatalf("expected not found after consume")
	}

	// Expired token
	expired := models.EmailVerification{UserID: u.ID, Email: u.Email, CodeHash: "code2", ExpiresAt: time.Now().Add(-1 * time.Minute)}
	if err := erepo.Create(ctx, &expired); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if got, err := erepo.GetValidByHash(ctx, "code2", time.Now()); err == nil || got != nil {
		t.Fatalf("expected not found for expired token")
	}

	// DeleteAllForUser
	if c, err := erepo.DeleteAllForUser(ctx, u.ID.String()); err != nil {
		t.Fatalf("delete all: %v", err)
	} else if c == 0 {
		t.Fatalf("expected delete count >0")
	}

	// Тест FindLatestByUser
	ev3 := models.EmailVerification{
		UserID:    u.ID,
		Email:     u.Email,
		CodeHash:  "verify_hash3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := erepo.Create(ctx, &ev3); err != nil {
		t.Fatalf("failed to create email verification: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Небольшая задержка

	ev4 := models.EmailVerification{
		UserID:    u.ID,
		Email:     u.Email,
		CodeHash:  "verify_hash4",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := erepo.Create(ctx, &ev4); err != nil {
		t.Fatalf("failed to create email verification: %v", err)
	}

	// FindLatestByUser должен вернуть последний созданный
	if latest, err := erepo.FindLatestByUser(ctx, u.ID); err != nil {
		t.Fatalf("failed to find latest email verification: %v", err)
	} else if latest.CodeHash != "verify_hash4" {
		t.Fatalf("expected latest code hash to be verify_hash4, got %s", latest.CodeHash)
	}
}

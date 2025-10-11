package repository_test

import (
	"context"
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
		t.Fatalf("failed to update user password: %v", err)
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
	}

	if err := repo.Create(ctx, &r); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	if _, err := repo.IsActive(ctx, u_id, "hash_123", time.Now()); err != nil {
		t.Fatalf("failed to check if refresh token is active: %v", err)
	}

	if err := repo.Touch(ctx, u_id, "hash_123", time.Now()); err != nil {
		t.Fatalf("failed to touch refresh token: %v", err)
	}

	if _, err := repo.RevokeByHash(ctx, u_id, "hash_123"); err != nil {
		t.Fatalf("failed to revoke refresh token: %v", err)
	}

	// Проверяем, что токен больше не существует
	if _, err := repo.GetByHash(ctx, u_id, "hash_123"); err == nil {
		t.Fatal("expected not found error, got nil")
	}

	// ещё раз создаём несколько токенов
	r.TokenHash = "hash_456"
	r.ID = uuid.New()
	if err := repo.Create(ctx, &r); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	r.TokenHash = "hash_789"
	r.ID = uuid.New()
	if err := repo.Create(ctx, &r); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	if _, err := repo.RevokeAll(ctx, u_id); err != nil {
		t.Fatalf("failed to revoke all refresh tokens: %v", err)
	}

	// Проверяем, что токены больше не существуют
	if _, err := repo.GetByHash(ctx, u_id, "hash_456"); err == nil {
		t.Fatal("expected not found error, got nil")
	}

	if _, err := repo.GetByHash(ctx, u_id, "hash_789"); err == nil {
		t.Fatal("expected not found error, got nil")
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
		t.Fatalf("expected 2 public keys got %d", len(pubs))
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
}

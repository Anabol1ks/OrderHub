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

	if getPr, err := repo.PasswordReset.GetValidByHash(ctx, u.ID.String(), pr.CodeHash, time.Now()); err != nil {
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

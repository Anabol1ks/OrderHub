package cleanup

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CleanupService struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewCleanupService(db *gorm.DB, log *zap.Logger) *CleanupService {
	return &CleanupService{
		db:  db,
		log: log,
	}
}

// CleanupExpiredTokens удаляет истёкшие refresh токены, password reset и email verification токены
func (c *CleanupService) CleanupExpiredTokens(ctx context.Context) error {
	now := time.Now()

	// Удаляем истёкшие refresh токены
	result := c.db.WithContext(ctx).
		Exec("DELETE FROM refresh_tokens WHERE expires_at < ?", now)
	if result.Error != nil {
		c.log.Error("failed to cleanup expired refresh tokens", zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected > 0 {
		c.log.Info("cleaned up expired refresh tokens", zap.Int64("count", result.RowsAffected))
	}

	// Удаляем истёкшие password reset токены
	result = c.db.WithContext(ctx).
		Exec("DELETE FROM password_reset_tokens WHERE expires_at < ?", now)
	if result.Error != nil {
		c.log.Error("failed to cleanup expired password reset tokens", zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected > 0 {
		c.log.Info("cleaned up expired password reset tokens", zap.Int64("count", result.RowsAffected))
	}

	// Удаляем истёкшие email verification токены
	result = c.db.WithContext(ctx).
		Exec("DELETE FROM email_verifications WHERE expires_at < ?", now)
	if result.Error != nil {
		c.log.Error("failed to cleanup expired email verification tokens", zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected > 0 {
		c.log.Info("cleaned up expired email verification tokens", zap.Int64("count", result.RowsAffected))
	}

	return nil
}

// CleanupOrphanedSessions удаляет сессии, у которых нет активных refresh токенов
func (c *CleanupService) CleanupOrphanedSessions(ctx context.Context) error {
	now := time.Now()

	// Находим сессии без активных refresh токенов
	query := `
		DELETE FROM user_sessions 
		WHERE id NOT IN (
			SELECT DISTINCT session_id 
			FROM refresh_tokens 
			WHERE session_id IS NOT NULL 
			AND expires_at > ? 
			AND revoked = false
		)
	`

	result := c.db.WithContext(ctx).Exec(query, now)
	if result.Error != nil {
		c.log.Error("failed to cleanup orphaned sessions", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected > 0 {
		c.log.Info("cleaned up orphaned sessions", zap.Int64("count", result.RowsAffected))
	}

	return nil
}

// CleanupOldSessions удаляет старые неактивные сессии (старше 30 дней)
func (c *CleanupService) CleanupOldSessions(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -30) // 30 дней назад

	result := c.db.WithContext(ctx).
		Exec("DELETE FROM user_sessions WHERE last_seen_at < ?", cutoff)
	if result.Error != nil {
		c.log.Error("failed to cleanup old sessions", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected > 0 {
		c.log.Info("cleaned up old sessions", zap.Int64("count", result.RowsAffected))
	}

	return nil
}

// CleanupConsumedTokens удаляет уже использованные (consumed) токены старше 24 часов
func (c *CleanupService) CleanupConsumedTokens(ctx context.Context) error {
	cutoff := time.Now().Add(-24 * time.Hour)

	// Удаляем использованные password reset токены
	result := c.db.WithContext(ctx).
		Exec("DELETE FROM password_reset_tokens WHERE consumed = true AND created_at < ?", cutoff)
	if result.Error != nil {
		c.log.Error("failed to cleanup consumed password reset tokens", zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected > 0 {
		c.log.Info("cleaned up consumed password reset tokens", zap.Int64("count", result.RowsAffected))
	}

	// Удаляем использованные email verification токены
	result = c.db.WithContext(ctx).
		Exec("DELETE FROM email_verifications WHERE consumed = true AND created_at < ?", cutoff)
	if result.Error != nil {
		c.log.Error("failed to cleanup consumed email verification tokens", zap.Error(result.Error))
		return result.Error
	}
	if result.RowsAffected > 0 {
		c.log.Info("cleaned up consumed email verification tokens", zap.Int64("count", result.RowsAffected))
	}

	return nil
}

// RunFullCleanup выполняет все задачи очистки
func (c *CleanupService) RunFullCleanup(ctx context.Context) error {
	c.log.Info("starting full cleanup")

	if err := c.CleanupExpiredTokens(ctx); err != nil {
		return err
	}

	if err := c.CleanupConsumedTokens(ctx); err != nil {
		return err
	}

	if err := c.CleanupOrphanedSessions(ctx); err != nil {
		return err
	}

	if err := c.CleanupOldSessions(ctx); err != nil {
		return err
	}

	c.log.Info("full cleanup completed")
	return nil
}

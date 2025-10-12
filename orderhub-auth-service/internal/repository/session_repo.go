package repository

import (
	"auth-service/internal/models"
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionRepo interface {
	Create(ctx context.Context, s *models.UserSession) error
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
	Revoke(ctx context.Context, id uuid.UUID) (bool, error)
	RevokeAllByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListActiveByUser(ctx context.Context, userID uuid.UUID, since time.Time) ([]models.UserSession, error)
}

type sessionRepo struct{ db *gorm.DB }

func NewSessionRepo(db *gorm.DB) SessionRepo { return &sessionRepo{db: db} }

func (r *sessionRepo) Create(ctx context.Context, s *models.UserSession) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *sessionRepo) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&models.UserSession{}).
		Where("id = ?", id).Update("last_seen_at", at).Error
}

func (r *sessionRepo) Revoke(ctx context.Context, id uuid.UUID) (bool, error) {
	res := r.db.WithContext(ctx).Model(&models.UserSession{}).
		Where("id = ? AND revoked = false", id).Update("revoked", true)
	return res.RowsAffected > 0, res.Error
}

func (r *sessionRepo) RevokeAllByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Model(&models.UserSession{}).
		Where("user_id = ? AND revoked = false", userID).Update("revoked", true)
	return res.RowsAffected, res.Error
}

func (r *sessionRepo) ListActiveByUser(ctx context.Context, userID uuid.UUID, since time.Time) ([]models.UserSession, error) {
	var list []models.UserSession
	err := r.db.WithContext(ctx).Where("user_id = ? AND revoked = false AND last_seen_at >= ?", userID, since).
		Order("last_seen_at DESC").Find(&list).Error
	return list, err
}

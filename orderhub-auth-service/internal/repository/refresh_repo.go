package repository

import (
	"auth-service/internal/models"
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RefreshRepo interface {
	Create(ctx context.Context, t *models.RefreshToken) error
	RevokeAll(ctx context.Context, userID uuid.UUID) (int64, error)
	Touch(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error
	GetByHashOnly(ctx context.Context, hash string) (*models.RefreshToken, error)
	IsActiveByHash(ctx context.Context, hash string, now time.Time) (bool, error)
	RevokeByHashOnly(ctx context.Context, hash string) (bool, error)
	HasActiveBySession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error)
}

type refreshRepo struct{ db *gorm.DB }

func NewRefreshRepo(db *gorm.DB) RefreshRepo {
	return &refreshRepo{db: db}
}

func (r *refreshRepo) Create(ctx context.Context, t *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *refreshRepo) RevokeAll(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Model(&models.RefreshToken{}).Where("user_id = ? AND revoked = false", userID).Update("revoked", true)
	return res.RowsAffected, res.Error
}

func (r *refreshRepo) Touch(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id=? AND token_hash=? AND revoked = false", userID, hash).
		Update("last_used_at", at).Error
}

func (r *refreshRepo) GetByHashOnly(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("token_hash=? AND revoked=false", hash).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *refreshRepo) IsActiveByHash(ctx context.Context, hash string, now time.Time) (bool, error) {
	var cnt int64
	err := r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("token_hash=? AND revoked=false AND expires_at>?", hash, now).
		Count(&cnt).Error
	return cnt > 0, err
}

func (r *refreshRepo) RevokeByHashOnly(ctx context.Context, hash string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("token_hash=? AND revoked=false", hash).
		Update("revoked", true)
	return res.RowsAffected > 0, res.Error
}

func (r *refreshRepo) HasActiveBySession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	var cnt int64
	err := r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("session_id = ? AND revoked = false AND expires_at > ?", sessionID, now).
		Count(&cnt).Error
	return cnt > 0, err
}

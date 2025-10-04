package repository

import (
	"auth-service/internal/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type EmailVerificationRepo interface {
	Create(ctx context.Context, v *models.EmailVerification) error
	GetValidByHash(ctx context.Context, userID string, codeHash string, now time.Time) (*models.EmailVerification, error)
	Consume(ctx context.Context, id string) (bool, error)
	DeleteAllForUser(ctx context.Context, userID string) (int64, error)
}

type emailVerificationRepo struct{ db *gorm.DB }

func NewEmailVerificationRepo(db *gorm.DB) EmailVerificationRepo {
	return &emailVerificationRepo{db: db}
}

func (r *emailVerificationRepo) Create(ctx context.Context, v *models.EmailVerification) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *emailVerificationRepo) GetValidByHash(ctx context.Context, userID string, codeHash string, now time.Time) (*models.EmailVerification, error) {
	var ev models.EmailVerification
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND code_hash = ? AND consumed = false AND expires_at > ?", userID, codeHash, now).
		First(&ev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &ev, nil
}

func (r *emailVerificationRepo) Consume(ctx context.Context, id string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&models.EmailVerification{}).
		Where("id = ? AND consumed = false", id).
		Update("consumed", true)
	return res.RowsAffected > 0, res.Error
}

func (r *emailVerificationRepo) DeleteAllForUser(ctx context.Context, userID string) (int64, error) {
	res := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Delete(&models.EmailVerification{})
	return res.RowsAffected, res.Error
}

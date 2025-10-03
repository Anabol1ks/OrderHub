package repository

import (
	"auth-service/internal/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("not found")

type PasswordResetRepo interface {
	Create(ctx context.Context, t *models.PasswordResetToken) error
	GetValidByHash(ctx context.Context, userID, codeHash string, now time.Time) (*models.PasswordResetToken, error)
	Consume(ctx context.Context, id string) (bool, error)
	DeleteAllForUser(ctx context.Context, userID string) (int64, error)
}

type passwordResetRepo struct {
	db *gorm.DB
}

func NewPasswordResetRepo(db *gorm.DB) PasswordResetRepo {
	return &passwordResetRepo{db: db}
}

func (r *passwordResetRepo) Create(ctx context.Context, t *models.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *passwordResetRepo) GetValidByHash(ctx context.Context, userID, codeHash string, now time.Time) (*models.PasswordResetToken, error) {
	var pr models.PasswordResetToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND code_hash = ? AND consumed = false AND expires_at > ?", userID, codeHash, now).First(&pr).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &pr, nil
}

func (r *passwordResetRepo) Consume(ctx context.Context, id string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Where("id = ? AND consumed = false", id).
		Update("consumed", true)
	return res.RowsAffected > 0, res.Error
}

func (r *passwordResetRepo) DeleteAllForUser(ctx context.Context, userID string) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.PasswordResetToken{})
	return res.RowsAffected, res.Error
}

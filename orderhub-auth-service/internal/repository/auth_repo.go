package repository

import (
	"auth-service/internal/models"
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepo interface {
	Create(ctx context.Context, u *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdatePassword(ctx context.Context, user *models.User) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateIsEmailVerified(ctx context.Context, user *models.User) error
}

type userRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) UserRepo { return &userRepo{db: db} }

func (r *userRepo) Create(ctx context.Context, u *models.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("lower(email) = lower(?)", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{"password": user.Password}).
		Error
}

func (r *userRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("lower(email) = lower(?)", email).
		Count(&count).Error
	return count > 0, err
}

func (r *userRepo) UpdateIsEmailVerified(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{"is_email_verified": user.IsEmailVerified}).
		Error
}

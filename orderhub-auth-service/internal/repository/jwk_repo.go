package repository

import (
	"auth-service/internal/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

type PublicJWK struct {
	KID string `gorm:"column:kid"`
	Alg string `gorm:"column:alg"`
	Kty string `gorm:"column:kty"`
	Use string `gorm:"column:use"`
	N   string `gorm:"column:n"`
	E   string `gorm:"column:e"`
}

type JWKRepo interface {
	Create(ctx context.Context, key *models.JwkKey) error
	GetActive(ctx context.Context) (*models.JwkKey, error)
	GetByKID(ctx context.Context, kid string) (*models.JwkKey, error)
	SetActive(ctx context.Context, kid string) error
	ListPublic(ctx context.Context) ([]PublicJWK, error)
}

type jwkRepo struct{ db *gorm.DB }

func NewJWKRepo(db *gorm.DB) JWKRepo { return &jwkRepo{db: db} }

func (r *jwkRepo) Create(ctx context.Context, key *models.JwkKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *jwkRepo) GetActive(ctx context.Context) (*models.JwkKey, error) {
	var k models.JwkKey
	if err := r.db.WithContext(ctx).
		Where("active = true").First(&k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *jwkRepo) GetByKID(ctx context.Context, kid string) (*models.JwkKey, error) {
	var k models.JwkKey
	if err := r.db.WithContext(ctx).
		Where("kid = ?", kid).First(&k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

// Делает выбранный ключ активным, снимая active со старого. Оборачиваем в транзакцию.
func (r *jwkRepo) SetActive(ctx context.Context, kid string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Снять флаг со всех активных (на случай гонки или миграций)
		if err := tx.Model(&models.JwkKey{}).
			Where("active = true").
			Update("active", false).Error; err != nil {
			return err
		}
		// Включить нужный
		res := tx.Model(&models.JwkKey{}).
			Where("kid = ?", kid).
			Update("active", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *jwkRepo) ListPublic(ctx context.Context) ([]PublicJWK, error) {
	var rows []PublicJWK
	// Можно отдавать все ключи (активные и недавно ротированные), чтобы валидация старых access ещё работала
	err := r.db.WithContext(ctx).
		Model(&models.JwkKey{}).
		Select("kid, alg, kty, use, n, e").
		Find(&rows).Error
	return rows, err
}

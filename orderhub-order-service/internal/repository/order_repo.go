package repository

import (
	"context"
	"errors"
	"order-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderListFilter struct {
	UserID *uuid.UUID
	Status *models.OrderStatus
	Limit  int
	Offset int
}

type OrderRepo interface {
	Create(ctx context.Context, o *models.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Order, error)
	GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.OrderStatus, reason *string) error
	UpdateTotals(ctx context.Context, id uuid.UUID, totalCents int64, currencyCode string) error
	List(ctx context.Context, f OrderListFilter) ([]*models.Order, int64, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)

	WithTx(ctx context.Context, fn func(txRepo OrderRepo, txItems OrderItemRepo) error) error
}

type orderRepo struct{ db *gorm.DB }

func NewOrderRepo(db *gorm.DB) OrderRepo { return &orderRepo{db: db} }

func (r *orderRepo) Create(ctx context.Context, o *models.Order) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *orderRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Order, error) {
	var ord models.Order
	err := r.db.WithContext(ctx).Preload("Items").First(&ord, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &ord, err
}

func (r *orderRepo) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Order, error) {
	var ord models.Order
	err := r.db.WithContext(ctx).Preload("Items").First(&ord, "id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &ord, err
}

func (r *orderRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status models.OrderStatus, reason *string) error {
	upd := map[string]any{"status": status}
	if reason != nil {
		upd["cancel_reason"] = reason
	}

	return r.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", id).Updates(upd).Error
}

func (r *orderRepo) UpdateTotals(ctx context.Context, id uuid.UUID, totalCents int64, currencyCode string) error {
	return r.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", id).Updates(map[string]any{
		"total_price_cents": totalCents,
		"currency_code":     currencyCode,
	}).Error
}

func (r *orderRepo) List(ctx context.Context, f OrderListFilter) ([]*models.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Order{})

	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if f.Limit <= 0 {
		f.Limit = 20
	}

	if f.Offset < 0 {
		f.Offset = 0
	}

	var list []*models.Order
	err := q.Order("created_at DESC").Limit(f.Limit).Offset(f.Offset).Preload("Items").Find(&list).Error
	return list, total, err
}

func (r *orderRepo) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var cnt int64
	err := r.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", id).Count(&cnt).Error
	return cnt > 0, err
}

func (r *orderRepo) WithTx(ctx context.Context, fn func(txRepo OrderRepo, txItems OrderItemRepo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&orderRepo{db: tx}, &orderItemRepo{db: tx})
	})
}

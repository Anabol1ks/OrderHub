package repository

import (
	"context"
	"errors"
	"order-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderItemRepo interface {
	BulkCreate(ctx context.Context, items []models.OrderItem) error
	GetByOrderID(ctx context.Context, orderID uuid.UUID) ([]models.OrderItem, error)
	SumByOrder(ctx context.Context, orderID uuid.UUID) (totalCents int64, currencyCode string, err error)
	DeleteByOrderID(ctx context.Context, orderID uuid.UUID) (int64, error)
}

type orderItemRepo struct{ db *gorm.DB }

func NewOrderItemRepo(db *gorm.DB) OrderItemRepo { return &orderItemRepo{db: db} }

func (r *orderItemRepo) BulkCreate(ctx context.Context, items []models.OrderItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *orderItemRepo) GetByOrderID(ctx context.Context, orderID uuid.UUID) ([]models.OrderItem, error) {
	var rows []models.OrderItem
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at ASC").Find(&rows).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return rows, err
}

func (r *orderItemRepo) SumByOrder(ctx context.Context, orderID uuid.UUID) (int64, string, error) {
	type aggRow struct {
		TotalCents   int64
		CurrencyCode string
	}

	var res aggRow
	err := r.db.WithContext(ctx).Model(&models.OrderItem{}).Select("COALESCE(SUM(line_total_cents),0) AS total_cents, MIN(currency_code) AS currency_code").Where("order_id = ?", orderID).Scan(&res).Error
	return res.TotalCents, res.CurrencyCode, err
}

func (r *orderItemRepo) DeleteByOrderID(ctx context.Context, orderID uuid.UUID) (int64, error) {
	tx := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Delete(&models.OrderItem{})
	return tx.RowsAffected, tx.Error
}

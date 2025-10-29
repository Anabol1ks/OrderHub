package repository

import (
	"context"
	"errors"
	"inventory-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReservationRepo interface {
	// Upsert «ожидаемую» запись (для идемпотентности)
	UpsertPending(ctx context.Context, orderID, productID uuid.UUID, qty int32) error
	// Маркировки статуса
	MarkReserved(ctx context.Context, orderID, productID uuid.UUID) (bool, error)
	MarkReleased(ctx context.Context, orderID, productID uuid.UUID) (bool, error)
	MarkFailed(ctx context.Context, orderID, productID uuid.UUID) (bool, error)

	// Массовые операции по order_id
	ListByOrder(ctx context.Context, orderID uuid.UUID) ([]models.Reservation, error)
	ReleaseByOrder(ctx context.Context, orderID uuid.UUID) (int64, error)
	ConfirmByOrder(ctx context.Context, orderID uuid.UUID) (int64, error)
	Exists(ctx context.Context, orderID, productID uuid.UUID) (bool, error)
}

type reservationRepo struct{ db *gorm.DB }

func NewReservationRepo(db *gorm.DB) ReservationRepo { return &reservationRepo{db: db} }

func (r *reservationRepo) UpsertPending(ctx context.Context, orderID, productID uuid.UUID, qty int32) error {
	rec := models.Reservation{
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  qty,
		Status:    models.ReservationPending,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "order_id"}, {Name: "product_id"}},
			DoUpdates: clause.Assignments(map[string]any{"quantity": qty, "status": models.ReservationPending}),
		}).
		Create(&rec).Error
}

func (r *reservationRepo) MarkReserved(ctx context.Context, orderID, productID uuid.UUID) (bool, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Where("order_id = ? AND product_id = ?", orderID, productID).
		Update("status", models.ReservationReserved)
	return tx.RowsAffected > 0, tx.Error
}

func (r *reservationRepo) MarkReleased(ctx context.Context, orderID, productID uuid.UUID) (bool, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Where("order_id = ? AND product_id = ?", orderID, productID).
		Update("status", models.ReservationReleased)
	return tx.RowsAffected > 0, tx.Error
}

func (r *reservationRepo) MarkFailed(ctx context.Context, orderID, productID uuid.UUID) (bool, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Where("order_id = ? AND product_id = ?", orderID, productID).
		Update("status", models.ReservationFailed)
	return tx.RowsAffected > 0, tx.Error
}

func (r *reservationRepo) ListByOrder(ctx context.Context, orderID uuid.UUID) ([]models.Reservation, error) {
	var list []models.Reservation
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("created_at ASC").
		Find(&list).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return list, err
}

func (r *reservationRepo) ReleaseByOrder(ctx context.Context, orderID uuid.UUID) (int64, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Where("order_id = ?", orderID).
		Update("status", models.ReservationReleased)
	return tx.RowsAffected, tx.Error
}

func (r *reservationRepo) ConfirmByOrder(ctx context.Context, orderID uuid.UUID) (int64, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Where("order_id = ?", orderID).
		Update("status", models.ReservationReserved)
	return tx.RowsAffected, tx.Error
}

func (r *reservationRepo) Exists(ctx context.Context, orderID, productID uuid.UUID) (bool, error) {
	var cnt int64
	err := r.db.WithContext(ctx).
		Model(&models.Reservation{}).
		Where("order_id = ? AND product_id = ?", orderID, productID).
		Count(&cnt).Error
	return cnt > 0, err
}

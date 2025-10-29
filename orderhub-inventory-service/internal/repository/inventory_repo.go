package repository

import (
	"context"
	"errors"
	"inventory-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InventoryRepo interface {
	Get(ctx context.Context, productID uuid.UUID) (*models.Inventory, error)
	SetAvailable(ctx context.Context, productID uuid.UUID, available int32) error
	AdjustAvailable(ctx context.Context, productID uuid.UUID, delta int32) (bool, error)

	// Резервирование на складе (атомарно):
	// TryReserve: if available >= qty then available -= qty; reserved += qty
	TryReserve(ctx context.Context, productID uuid.UUID, qty int32) (bool, error)
	// Release: reserved -= qty; available += qty (предполагаем reserved >= qty)
	Release(ctx context.Context, productID uuid.UUID, qty int32) (bool, error)
	// Confirm: reserved -= qty (списываем резерв окончательно)
	Confirm(ctx context.Context, productID uuid.UUID, qty int32) (bool, error)
}

type inventoryRepo struct{ db *gorm.DB }

func NewInventoryRepo(db *gorm.DB) InventoryRepo { return &inventoryRepo{db: db} }

func (r *inventoryRepo) Get(ctx context.Context, productID uuid.UUID) (*models.Inventory, error) {
	var inv models.Inventory
	err := r.db.WithContext(ctx).First(&inv, "product_id = ?", productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &inv, err
}

func (r *inventoryRepo) SetAvailable(ctx context.Context, productID uuid.UUID, available int32) error {
	return r.db.WithContext(ctx).Model(&models.Inventory{}).Where("product_id = ?", productID).Update("available", available).Error
}

func (r *inventoryRepo) AdjustAvailable(ctx context.Context, productID uuid.UUID, delta int32) (bool, error) {
	tx := r.db.WithContext(ctx).Exec(`
UPDATE inventories
SET available = available + @delta,
    updated_at = now()
WHERE product_id = @pid
  AND available + @delta >= 0
`, map[string]any{
		"pid":   productID,
		"delta": delta,
	})
	return tx.RowsAffected > 0, tx.Error
}

func (r *inventoryRepo) TryReserve(ctx context.Context, productID uuid.UUID, qty int32) (bool, error) {
	// атомарно: available -= qty, reserved += qty, если хватает
	tx := r.db.WithContext(ctx).Exec(`
UPDATE inventories
SET available = available - @q,
    reserved  = reserved  + @q,
    updated_at = now()
WHERE product_id = @pid
  AND available >= @q
`, map[string]any{
		"pid": productID,
		"q":   qty,
	})
	return tx.RowsAffected > 0, tx.Error
}

func (r *inventoryRepo) Release(ctx context.Context, productID uuid.UUID, qty int32) (bool, error) {
	// reserved -= qty, available += qty (если хватает резерва)
	tx := r.db.WithContext(ctx).Exec(`
UPDATE inventories
SET reserved  = reserved  - @q,
    available = available + @q,
    updated_at = now()
WHERE product_id = @pid
  AND reserved >= @q
`, map[string]any{
		"pid": productID,
		"q":   qty,
	})
	return tx.RowsAffected > 0, tx.Error
}

func (r *inventoryRepo) Confirm(ctx context.Context, productID uuid.UUID, qty int32) (bool, error) {
	// списываем резерв окончательно: reserved -= qty
	tx := r.db.WithContext(ctx).Exec(`
UPDATE inventories
SET reserved  = reserved  - @q,
    updated_at = now()
WHERE product_id = @pid
  AND reserved >= @q
`, map[string]any{
		"pid": productID,
		"q":   qty,
	})
	return tx.RowsAffected > 0, tx.Error
}

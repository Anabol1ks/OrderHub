package service

import (
	"context"

	"inventory-service/internal/models"

	"github.com/google/uuid"
)

const currencyRUB = "RUB"

type ProductInput struct {
	VendorID     uuid.UUID
	SKU          string
	Name         string
	Description  string
	PriceCents   int64
	CurrencyCode string // ожидаем "RUB"
	IsActive     bool
}

type ProductPatch struct {
	SKU          *string
	Name         *string
	Description  *string
	PriceCents   *int64
	CurrencyCode *string // если передали — должен быть "RUB"
	IsActive     *bool
}

type ProductListFilter struct {
	VendorID   *uuid.UUID
	Query      string
	OnlyActive *bool
	Limit      int
	Offset     int
}

type ReserveItem struct {
	ProductID uuid.UUID
	Quantity  uint32
}
type ReserveOkItem struct {
	ProductID uuid.UUID
	Quantity  uint32
}
type ReserveFailedItem struct {
	ProductID uuid.UUID
	Requested uint32
	Reason    string
}
type ReserveResult struct {
	OK     []ReserveOkItem
	Failed []ReserveFailedItem
}

type InventoryService interface {
	// catalog
	CreateProduct(ctx context.Context, in ProductInput) (*models.Product, error)
	UpdateProduct(ctx context.Context, productID uuid.UUID, patch ProductPatch) (*models.Product, error)
	GetProduct(ctx context.Context, productID uuid.UUID) (*models.Product, error)
	ListProducts(ctx context.Context, f ProductListFilter) ([]models.Product, int64, error)
	DeleteProduct(ctx context.Context, productID uuid.UUID) (bool, error)
	BatchGetProducts(ctx context.Context, ids []uuid.UUID) ([]models.Product, error)

	// stock
	GetStock(ctx context.Context, productID uuid.UUID) (*models.Inventory, error)
	SetStock(ctx context.Context, productID uuid.UUID, available int32) (*models.Inventory, error)
	AdjustStock(ctx context.Context, productID uuid.UUID, delta int32) (*models.Inventory, error)

	// reservations (saga)
	Reserve(ctx context.Context, orderID uuid.UUID, items []ReserveItem) (ReserveResult, error)
	Release(ctx context.Context, orderID uuid.UUID) (int64, error)
	Confirm(ctx context.Context, orderID uuid.UUID) (int64, error)
}

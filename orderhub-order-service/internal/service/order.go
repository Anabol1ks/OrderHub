package service

import (
	"context"

	"order-service/internal/models"

	"github.com/google/uuid"
)

type CreateOrderItem struct {
	ProductID uuid.UUID
	Quantity  uint32
}

type CreateOrderInput struct {
	Items   []CreateOrderItem
	Comment string
}

type ListFilter struct {
	UserID *uuid.UUID
	Status *models.OrderStatus
	Limit  int
	Offset int
}

type OrderService interface {
	CreateOrder(ctx context.Context, in CreateOrderInput) (*models.Order, error)
	GetOrder(ctx context.Context, id uuid.UUID) (*models.Order, error)
	ListOrders(ctx context.Context, f ListFilter) ([]models.Order, int64, error)
	CancelOrder(ctx context.Context, id uuid.UUID, reason *string) (*models.Order, error)
}

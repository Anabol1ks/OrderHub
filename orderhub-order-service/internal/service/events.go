package service

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OrderItemEvent struct {
	ProductID  uuid.UUID `json:"product_id"`
	Quantity   uint32    `json:"quantity"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	LineTotal  int64     `json:"line_total_cents"`
}

type OrderCreatedEvent struct {
	OrderID    uuid.UUID        `json:"order_id"`
	UserID     uuid.UUID        `json:"user_id"`
	Items      []OrderItemEvent `json:"items"`
	TotalCents int64            `json:"total_cents"`
	Currency   string           `json:"currency"`
	CreatedAt  time.Time        `json:"created_at"`
}

type OrderCancelledEvent struct {
	OrderID     uuid.UUID `json:"order_id"`
	UserID      uuid.UUID `json:"user_id"`
	Reason      string    `json:"reason,omitempty"`
	CancelledAt time.Time `json:"cancelled_at"`
}

type EventBus interface {
	PublishOrderCreated(ctx context.Context, e OrderCreatedEvent) error
	PublishOrderCancelled(ctx context.Context, e OrderCancelledEvent) error
}

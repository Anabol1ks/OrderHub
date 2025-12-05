package models

import (
	"time"

	"github.com/google/uuid"
)

// Статус заказа — строковый тип (как Role у тебя в auth)
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "ORDER_STATUS_PENDING"
	OrderStatusConfirmed OrderStatus = "ORDER_STATUS_CONFIRMED"
	OrderStatusCancelled OrderStatus = "ORDER_STATUS_CANCELLED"
)

type Order struct {
	ID              uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID          uuid.UUID   `gorm:"type:uuid;not null;index"`
	Status          OrderStatus `gorm:"type:text;not null;default:'ORDER_STATUS_PENDING';index"`
	TotalPriceCents int64       `gorm:"not null;default:0"`
	CurrencyCode    string      `gorm:"type:char(3);not null"`
	CancelReason    *string     `gorm:"type:text"`

	CreatedAt time.Time `gorm:"not null;default:now();index"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`

	Items []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"` // каскад на позиции
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID        uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_order_items_order_product"` // композитный UNIQUE
	ProductID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_order_items_order_product"`
	Quantity       uint32    `gorm:"type:int;not null"` // CHECK добавим в миграции
	UnitPriceCents int64     `gorm:"not null"`
	LineTotalCents int64     `gorm:"not null"`
	CurrencyCode   string    `gorm:"type:char(3);not null"`

	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (OrderItem) TableName() string { return "order_items" }

package models

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	VendorID     uuid.UUID `gorm:"type:uuid;not null;index"`
	SKU          string    `gorm:"type:text;not null"`
	Name         string    `gorm:"type:text;not null"`
	Description  string    `gorm:"type:text"`
	PriceCents   int64     `gorm:"not null;default:0"`
	CurrencyCode string    `gorm:"type:char(3);not null;default:'RUB'"` // всегда RUB
	IsActive     bool      `gorm:"not null;default:true"`

	CreatedAt time.Time `gorm:"not null;default:now();index"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (Product) TableName() string {
	return "products"
}

type Inventory struct {
	ProductID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Available int32     `gorm:"not null;default:0"`
	Reserved  int32     `gorm:"not null;default:0"`

	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (Inventory) TableName() string {
	return "inventories"
}

type ReservationStatus string

const (
	ReservationPending  ReservationStatus = "PENDING"
	ReservationReserved ReservationStatus = "RESERVED"
	ReservationReleased ReservationStatus = "RELEASED"
	ReservationFailed   ReservationStatus = "FAILED"
)

type Reservation struct {
	ID        uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID   uuid.UUID         `gorm:"type:uuid;not null;index;uniqueIndex:ux_reservations_order_product"`
	ProductID uuid.UUID         `gorm:"type:uuid;not null;index;uniqueIndex:ux_reservations_order_product"`
	Quantity  int32             `gorm:"not null"`
	Status    ReservationStatus `gorm:"type:text;not null;default:'PENDING';index"`

	CreatedAt time.Time `gorm:"not null;default:now();index"`
}

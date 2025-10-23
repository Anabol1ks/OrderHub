package service

import (
	"context"

	"github.com/google/uuid"
)

type Price struct {
	UnitPriceCents int64
	CurrencyCode   string
}

type PricingProvider interface {
	GetPrice(ctx context.Context, productID uuid.UUID) (Price, error)
}

type StaticPricing struct{}

func (StaticPricing) GetPrice(ctx context.Context, productID uuid.UUID) (Price, error) {
	return Price{UnitPriceCents: 10_00, CurrencyCode: "RUB"}, nil // 10₽
}

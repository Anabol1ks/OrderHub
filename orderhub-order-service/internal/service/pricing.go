package service

import (
	"context"
	"fmt"

	commonv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/common/v1"
	inventoryv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/inventory/v1"
	"github.com/google/uuid"
)

type Price struct {
	UnitPriceCents int64
	CurrencyCode   string
}

type PricingProvider interface {
	GetPrice(ctx context.Context, productID uuid.UUID) (Price, error)
	GetPrices(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]Price, error)
}

// InventoryPricingClient wraps Inventory gRPC client for pricing
type InventoryPricingClient struct {
	client inventoryv1.InventoryServiceClient
}

func NewInventoryPricingClient(client inventoryv1.InventoryServiceClient) PricingProvider {
	return &InventoryPricingClient{client: client}
}

func (p *InventoryPricingClient) GetPrice(ctx context.Context, productID uuid.UUID) (Price, error) {
	resp, err := p.client.GetProduct(ctx, &inventoryv1.GetProductRequest{
		ProductId: &commonv1.UUID{Value: productID.String()},
	})
	if err != nil {
		return Price{}, fmt.Errorf("failed to get product from inventory: %w", err)
	}

	product := resp.GetProduct()
	if product == nil {
		return Price{}, fmt.Errorf("product not found: %s", productID)
	}

	if !product.GetIsActive() {
		return Price{}, fmt.Errorf("product is not active: %s", productID)
	}

	return Price{
		UnitPriceCents: product.GetPriceCents(),
		CurrencyCode:   product.GetCurrencyCode(),
	}, nil
}

func (p *InventoryPricingClient) GetPrices(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]Price, error) {
	if len(productIDs) == 0 {
		return map[uuid.UUID]Price{}, nil
	}

	protoIDs := make([]*commonv1.UUID, 0, len(productIDs))
	for _, id := range productIDs {
		protoIDs = append(protoIDs, &commonv1.UUID{Value: id.String()})
	}

	resp, err := p.client.BatchGetProducts(ctx, &inventoryv1.BatchGetProductsRequest{
		ProductIds: protoIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to batch get products from inventory: %w", err)
	}

	prices := make(map[uuid.UUID]Price, len(resp.GetProducts()))
	for _, product := range resp.GetProducts() {
		if product == nil {
			continue
		}

		pid, err := uuid.Parse(product.GetId().GetValue())
		if err != nil {
			continue
		}

		if !product.GetIsActive() {
			continue // пропускаем неактивные товары
		}

		prices[pid] = Price{
			UnitPriceCents: product.GetPriceCents(),
			CurrencyCode:   product.GetCurrencyCode(),
		}
	}

	return prices, nil
}

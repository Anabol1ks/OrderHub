package service

import (
	"context"
	"inventory-service/internal/models"
	"inventory-service/internal/repository"
	"strings"
	"time"

	"github.com/google/uuid"
)

type inventoryService struct {
	repo *repository.Repository
	now  func() time.Time
}

func NewInventoryService(repo *repository.Repository) *inventoryService {
	return &inventoryService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *inventoryService) requireAuth(ctx context.Context) (uuid.UUID, Role, error) {
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, "", ErrUnauthorized
	}

	role, ok := RoleFromContext(ctx)
	if !ok {
		return uuid.Nil, "", ErrUnauthorized
	}

	return uid, role, nil
}

func mustRub(code string) bool {
	return code == currencyRUB
}

func (s *inventoryService) CreateProduct(ctx context.Context, in ProductInput) (*models.Product, error) {
	reqUser, role, err := s.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if role != RoleAdmin && role != RoleVendor {
		return nil, ErrForbidden
	}

	if role == RoleVendor && in.VendorID != reqUser {
		return nil, ErrForbidden
	}

	if !mustRub(in.CurrencyCode) {
		return nil, ErrCurrencyNotRUB
	}

	now := s.now()
	p := &models.Product{
		VendorID:     in.VendorID,
		SKU:          strings.TrimSpace(in.SKU),
		Name:         strings.TrimSpace(in.Name),
		Description:  strings.TrimSpace(in.Description),
		PriceCents:   in.PriceCents,
		CurrencyCode: currencyRUB,
		IsActive:     in.IsActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = s.repo.WithTx(func(tx *repository.Repository) error {
		if existing, err := tx.Products.GetByVendorAndSKU(ctx, p.VendorID, p.SKU); err != nil {
			return err
		} else if existing != nil {
			return ErrSKUAlreadyExists
		}
		if err := tx.Products.Create(ctx, p); err != nil {
			return err
		}
		// 1:1 строка в inventories
		return tx.Products.EnsureInventoryRow(ctx, p.ID)
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (s *inventoryService) UpdateProduct(ctx context.Context, productID uuid.UUID, patch ProductPatch) (*models.Product, error) {
	reqUser, role, err := s.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	p, err := s.repo.Products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if p == nil {
		return nil, ErrProductNotFound
	}

	if role != RoleAdmin && !(role == RoleVendor && p.VendorID == reqUser) {
		return nil, ErrForbidden
	}

	fields := map[string]any{}

	if patch.SKU != nil {
		fields["sku"] = strings.TrimSpace(*patch.SKU)
	}

	if patch.Name != nil {
		fields["name"] = strings.TrimSpace(*patch.Name)
	}

	if patch.Description != nil {
		fields["description"] = strings.TrimSpace(*patch.Description)
	}

	if patch.CurrencyCode != nil {
		if !mustRub(*patch.CurrencyCode) {
			return nil, ErrCurrencyNotRUB
		}
		fields["currency_code"] = currencyRUB
	}

	if patch.IsActive != nil {
		fields["is_active"] = *patch.IsActive
	}

	if len(fields) == 0 {
		return p, nil
	}

	fields["updated_at"] = s.now()

	if v, ok := fields["sku"]; ok {
		newSKU := v.(string)
		if existing, err := s.repo.Products.GetByVendorAndSKU(ctx, p.VendorID, newSKU); err != nil {
			return nil, err
		} else if existing != nil && existing.ID != p.ID {
			return nil, ErrSKUAlreadyExists
		}
	}

	if err := s.repo.Products.UpdateFields(ctx, productID, fields); err != nil {
		return nil, err
	}

	return s.repo.Products.GetByID(ctx, productID)
}

func (s *inventoryService) GetProduct(ctx context.Context, productID uuid.UUID) (*models.Product, error) {
	p, err := s.repo.Products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProductNotFound
	}
	return p, nil
}

func (s *inventoryService) ListProducts(ctx context.Context, f ProductListFilter) ([]models.Product, int64, error) {
	return s.repo.Products.List(ctx, repository.ProductListFilter{
		VendorID:   f.VendorID,
		Query:      f.Query,
		OnlyActive: f.OnlyActive,
		Limit:      f.Limit,
		Offset:     f.Offset,
	})
}

func (s *inventoryService) DeleteProduct(ctx context.Context, productID uuid.UUID) (bool, error) {
	reqUser, role, err := s.requireAuth(ctx)
	if err != nil {
		return false, err
	}
	p, err := s.repo.Products.GetByID(ctx, productID)
	if err != nil {
		return false, err
	}
	if p == nil {
		return false, ErrProductNotFound
	}
	if role != RoleAdmin && !(role == RoleVendor && p.VendorID == reqUser) {
		return false, ErrForbidden
	}

	inv, err := s.repo.Inventories.Get(ctx, productID)
	if err != nil {
		return false, err
	}
	if inv != nil && inv.Reserved > 0 {
		return false, ErrCannotDeleteProductWithReservations
	}

	ok, err := s.repo.Products.Delete(ctx, productID)
	return ok, err
}

func (s *inventoryService) BatchGetProducts(ctx context.Context, ids []uuid.UUID) ([]models.Product, error) {
	return s.repo.Products.BatchGetByIDs(ctx, ids)
}

func (s *inventoryService) GetStock(ctx context.Context, productID uuid.UUID) (*models.Inventory, error) {
	inv, err := s.repo.Inventories.Get(ctx, productID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInventoryNotFound
	}
	return inv, nil
}

func (s *inventoryService) SetStock(ctx context.Context, productID uuid.UUID, available int32) (*models.Inventory, error) {
	reqUser, role, err := s.requireAuth(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.repo.Products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProductNotFound
	}

	if role != RoleAdmin && !(role == RoleVendor && p.VendorID == reqUser) {
		return nil, ErrForbidden
	}

	if err := s.repo.Inventories.SetAvailable(ctx, productID, available); err != nil {
		return nil, err
	}

	return s.repo.Inventories.Get(ctx, productID)
}

func (s *inventoryService) AdjustStock(ctx context.Context, productID uuid.UUID, delta int32) (*models.Inventory, error) {
	reqUser, role, err := s.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	p, err := s.repo.Products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProductNotFound
	}
	if role != RoleAdmin && !(role == RoleVendor && p.VendorID == reqUser) {
		return nil, ErrForbidden
	}

	_, err = s.repo.Inventories.AdjustAvailable(ctx, productID, delta)
	if err != nil {
		return nil, err
	}

	return s.repo.Inventories.Get(ctx, productID)
}

func (s *inventoryService) Reserve(ctx context.Context, orderID uuid.UUID, items []ReserveItem) (ReserveResult, error) {
	if len(items) == 0 {
		return ReserveResult{}, nil
	}

	// Проверяем, существует ли уже резервация для этого заказа
	existing, err := s.repo.Reservations.ListByOrder(ctx, orderID)
	if err != nil {
		return ReserveResult{}, err
	}
	if len(existing) > 0 {
		// Заказ уже имеет резервации - не допускаем повторную резервацию
		return ReserveResult{}, ErrReservationExists
	}

	res := ReserveResult{
		OK:     make([]ReserveOkItem, 0, len(items)),
		Failed: make([]ReserveFailedItem, 0),
	}

	now := s.now()

	err = s.repo.WithTx(func(tx *repository.Repository) error {
		for _, it := range items {
			if it.Quantity == 0 {
				return ErrInvalidQuantity
			}

			p, err := s.repo.Products.GetByID(ctx, it.ProductID)
			if err != nil {
				return err
			}
			if p == nil {
				res.Failed = append(res.Failed, ReserveFailedItem{
					ProductID: it.ProductID,
					Requested: it.Quantity,
					Reason:    "not found",
				})
				continue
			}
			if !p.IsActive {
				res.Failed = append(res.Failed, ReserveFailedItem{
					ProductID: it.ProductID, Requested: it.Quantity, Reason: "inactive",
				})
				continue
			}
			if p.CurrencyCode != currencyRUB {
				return ErrCurrencyNotRUB
			}
			if err := tx.Reservations.UpsertPending(ctx, orderID, it.ProductID, int32(it.Quantity)); err != nil {
				return err
			}

			// try reserve
			ok, err := tx.Inventories.TryReserve(ctx, it.ProductID, int32(it.Quantity))
			if err != nil {
				return err
			}
			if ok {
				if _, err := tx.Reservations.MarkReserved(ctx, orderID, it.ProductID); err != nil {
					return err
				}
				res.OK = append(res.OK, ReserveOkItem{ProductID: it.ProductID, Quantity: it.Quantity})
			} else {
				if _, err := tx.Reservations.MarkFailed(ctx, orderID, it.ProductID); err != nil {
					return err
				}
				res.Failed = append(res.Failed, ReserveFailedItem{
					ProductID: it.ProductID, Requested: it.Quantity, Reason: "out_of_stock",
				})
			}
		}

		_ = now // зарезервировано для возможного аудита/логов
		return nil
	})
	if err != nil {
		return ReserveResult{}, err
	}
	return res, nil
}

func (s *inventoryService) Release(ctx context.Context, orderID uuid.UUID) (int64, error) {
	var releasedTotal int64

	err := s.repo.WithTx(func(tx *repository.Repository) error {
		rows, err := tx.Reservations.ListByOrder(ctx, orderID)
		if err != nil {
			return err
		}

		for _, r := range rows {
			if r.Status == models.ReservationReserved {
				ok, err := tx.Inventories.Release(ctx, r.ProductID, r.Quantity)
				if err != nil {
					return err
				}
				if ok {
					releasedTotal++
				}
			}
			if _, err := tx.Reservations.MarkReleased(ctx, orderID, r.ProductID); err != nil {
				return err
			}
		}
		return nil
	})
	return releasedTotal, err
}

func (s *inventoryService) Confirm(ctx context.Context, orderID uuid.UUID) (int64, error) {
	var confirmedTotal int64 = 0
	err := s.repo.WithTx(func(tx *repository.Repository) error {
		rows, err := tx.Reservations.ListByOrder(ctx, orderID)
		if err != nil {
			return nil
		}
		for _, r := range rows {
			if r.Status == models.ReservationReserved {
				ok, err := tx.Inventories.Confirm(ctx, r.ID, r.Quantity)
				if err != nil {
					return err
				}
				if ok {
					confirmedTotal++
				}
			}
		}
		return nil
	})

	return confirmedTotal, err
}

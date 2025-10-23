package service

import (
	"context"
	"order-service/internal/models"
	"order-service/internal/repository"
	"time"

	"github.com/google/uuid"
)

const currencyRUB = "RUB"

type orderService struct {
	repo    *repository.Repository
	pricing PricingProvider
	events  EventBus
	now     func() time.Time
}

func NewOrderService(repo *repository.Repository, pricing PricingProvider, events EventBus) OrderService {
	return &orderService{
		repo:    repo,
		pricing: pricing,
		events:  events,
		now:     time.Now,
	}
}

func requireAuth(ctx context.Context) (uuid.UUID, Role, error) {
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, "", ErrUnauthorized
	}
	role, _ := RoleFromContext(ctx) // если нет — считаем customer по умолчанию
	return uid, role, nil
}

func toOrderStatus(s models.OrderStatus) string { return string(s) }

func (s *orderService) CreateOrder(ctx context.Context, in CreateOrderInput) (*models.Order, error) {
	userID, _, err := requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if len(in.Items) == 0 {
		return nil, ErrEmptyItems
	}

	var (
		order    *models.Order
		now      = s.now()
		itemsDB  []models.OrderItem
		total    int64
		currency = currencyRUB
	)

	err = s.repo.Orders.WithTx(ctx, func(or repository.OrderRepo, ir repository.OrderItemRepo) error {
		for _, it := range in.Items {
			if it.Quantity == 0 {
				return ErrQuantityInvalid
			}

			price, err := s.pricing.GetPrice(ctx, it.ProductID)
			if err != nil {
				return err
			}

			if price.CurrencyCode != currencyRUB {
				return ErrCurrencyMismatch
			}

			line := int64(it.Quantity) * price.UnitPriceCents
			total += line

			itemsDB = append(itemsDB, models.OrderItem{
				ProductID:      it.ProductID,
				Quantity:       it.Quantity,
				UnitPriceCents: price.UnitPriceCents,
				LineTotalCents: line,
				CurrencyCode:   currencyRUB,
				CreatedAt:      now,
			})
		}

		order = &models.Order{
			UserID:          userID,
			Status:          models.OrderStatusPending,
			TotalPriceCents: total,
			CurrencyCode:    currencyRUB,
			CancelReason:    nil,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := s.repo.Orders.Create(ctx, order); err != nil {
			return err
		}

		for i := range itemsDB {
			itemsDB[i].OrderID = order.ID
		}

		if err := ir.BulkCreate(ctx, itemsDB); err != nil {
			return err
		}

		if err := or.UpdateTotals(ctx, order.ID, total, currency); err != nil {
			return err
		}

		ordWith, err := or.GetByID(ctx, order.ID)
		if err != nil {
			return err
		}
		order = ordWith

		return nil
	})

	if err != nil {
		return nil, err
	}

	if s.events != nil {
		evItems := make([]OrderItemEvent, 0, len(itemsDB))
		for _, it := range itemsDB {
			evItems = append(evItems, OrderItemEvent{
				ProductID:  it.ProductID,
				Quantity:   it.Quantity,
				PriceCents: it.UnitPriceCents,
				Currency:   it.CurrencyCode,
				LineTotal:  it.LineTotalCents,
			})
		}
		_ = s.events.PublishOrderCreated(ctx, OrderCreatedEvent{
			OrderID:    order.ID,
			UserID:     order.UserID,
			Items:      evItems,
			TotalCents: order.TotalPriceCents,
			Currency:   order.CurrencyCode,
			CreatedAt:  order.CreatedAt,
		})
	}

	return order, nil
}

func (s *orderService) GetOrder(ctx context.Context, id uuid.UUID) (*models.Order, error) {
	userID, role, err := requireAuth(ctx)
	if err != nil {
		return nil, err
	}
	isAdmin := role == RoleAdmin

	var ord *models.Order
	if isAdmin {
		ord, err = s.repo.Orders.GetByID(ctx, id)
	} else {
		ord, err = s.repo.Orders.GetByIDForUser(ctx, id, userID)
	}
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrOrderNotFound
	}
	return ord, nil
}

func (s *orderService) ListOrders(ctx context.Context, f ListFilter) ([]models.Order, int64, error) {
	userID, role, err := requireAuth(ctx)
	if err != nil {
		return nil, 0, err
	}
	isAdmin := role == RoleAdmin

	if !isAdmin {
		f.UserID = &userID
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	ordersPtr, total, err := s.repo.Orders.List(ctx, repository.OrderListFilter{
		UserID: f.UserID,
		Status: f.Status,
		Limit:  f.Limit,
		Offset: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}

	orders := make([]models.Order, len(ordersPtr))
	for i, o := range ordersPtr {
		orders[i] = *o
	}
	return orders, total, nil
}

func (s *orderService) CancelOrder(ctx context.Context, id uuid.UUID, reason *string) (*models.Order, error) {
	userID, role, err := requireAuth(ctx)
	if err != nil {
		return nil, err
	}
	isAdmin := role == RoleAdmin

	ord, err := s.repo.Orders.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrOrderNotFound
	}
	if !isAdmin && ord.UserID != userID {
		return nil, ErrForbidden
	}
	switch ord.Status {
	case models.OrderStatusCancelled:
		return ord, ErrAlreadyCancelled
	case models.OrderStatusConfirmed:
		// допустимо отменять подтверждённый? На MVP — да, запускаем компенсацию
	default:
		// pending — ок
	}

	// меняем статус
	if err := s.repo.Orders.UpdateStatus(ctx, id, models.OrderStatusCancelled, reason); err != nil {
		return nil, err
	}
	ord, err = s.repo.Orders.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// событие компенсации
	if s.events != nil {
		_ = s.events.PublishOrderCancelled(ctx, OrderCancelledEvent{
			OrderID:     ord.ID,
			UserID:      ord.UserID,
			Reason:      s.sanitizeReason(reason),
			CancelledAt: s.now(),
		})
	}

	return ord, nil
}

func (s *orderService) sanitizeReason(reason *string) string {
	if reason == nil {
		return ""
	}
	r := *reason
	if len(r) > 500 {
		r = r[:500]
	}
	return r
}

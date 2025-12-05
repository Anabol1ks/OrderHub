package repository_test

import (
	"context"
	"testing"

	"order-service/internal/migrate"
	"order-service/internal/models"
	"order-service/internal/repository"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/testutil"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.SetupTestPostgres(t)
	if err := migrate.MigrateOrderDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestOrderRepo_CRUD_And_List(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewOrderRepo(db)

	ctx := context.Background()

	userID := uuid.New()
	ord := &models.Order{UserID: userID, CurrencyCode: "USD"}
	if err := repo.Create(ctx, ord); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if ok, err := repo.Exists(ctx, ord.ID); err != nil || !ok {
		t.Fatalf("Exists: ok=%v err=%v", ok, err)
	}

	get, err := repo.GetByID(ctx, ord.ID)
	if err != nil || get == nil {
		t.Fatalf("GetByID: %v %v", get, err)
	}

	getUser, err := repo.GetByIDForUser(ctx, ord.ID, userID)
	if err != nil || getUser == nil {
		t.Fatalf("GetByIDForUser: %v %v", getUser, err)
	}

	// UpdateTotals
	if err := repo.UpdateTotals(ctx, ord.ID, 12345, "USD"); err != nil {
		t.Fatalf("UpdateTotals: %v", err)
	}
	got, _ := repo.GetByID(ctx, ord.ID)
	if got.TotalPriceCents != 12345 || got.CurrencyCode != "USD" {
		t.Fatalf("UpdateTotals mismatch: %+v", got)
	}

	// UpdateStatus with reason
	reason := "cancelled by user"
	if err := repo.UpdateStatus(ctx, ord.ID, models.OrderStatusCancelled, &reason); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got2, _ := repo.GetByID(ctx, ord.ID)
	if got2.Status != models.OrderStatusCancelled || got2.CancelReason == nil || *got2.CancelReason != reason {
		t.Fatalf("UpdateStatus mismatch: %+v", got2)
	}

	// List with filters and pagination
	// create extra orders
	for i := 0; i < 3; i++ {
		_ = repo.Create(ctx, &models.Order{UserID: userID, CurrencyCode: "USD"})
	}
	list, total, err := repo.List(ctx, repository.OrderListFilter{UserID: &userID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 4 {
		t.Fatalf("total expected >=4 got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("list len expected 2 got %d", len(list))
	}
}

func TestOrderRepo_WithTx(t *testing.T) {
	db := setupDB(t)
	orders := repository.NewOrderRepo(db)

	ctx := context.Background()
	userID := uuid.New()
	ord := &models.Order{UserID: userID, CurrencyCode: "USD"}
	if err := orders.Create(ctx, ord); err != nil {
		t.Fatalf("Create: %v", err)
	}

	repo := repository.New(db)
	err := repo.Orders.WithTx(ctx, func(txOrders repository.OrderRepo, txItems repository.OrderItemRepo) error {
		// add 2 items
		p1, p2 := uuid.New(), uuid.New()
		items := []models.OrderItem{
			{OrderID: ord.ID, ProductID: p1, Quantity: 2, UnitPriceCents: 500, LineTotalCents: 1000, CurrencyCode: "USD"},
			{OrderID: ord.ID, ProductID: p2, Quantity: 1, UnitPriceCents: 700, LineTotalCents: 700, CurrencyCode: "USD"},
		}
		if err := txItems.BulkCreate(ctx, items); err != nil {
			return err
		}
		total, currency, err := txItems.SumByOrder(ctx, ord.ID)
		if err != nil {
			return err
		}
		return txOrders.UpdateTotals(ctx, ord.ID, total, currency)
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	o, _ := orders.GetByID(ctx, ord.ID)
	if o.TotalPriceCents != 1700 || o.CurrencyCode != "USD" {
		t.Fatalf("totals mismatch: %+v", o)
	}
}

func TestOrderItemRepo_CRUD(t *testing.T) {
	db := setupDB(t)
	items := repository.NewOrderItemRepo(db)

	ctx := context.Background()
	orderID := uuid.New()
	userID := uuid.New()
	// create parent order to satisfy FK
	if err := repository.NewOrderRepo(db).Create(ctx, &models.Order{ID: orderID, UserID: userID, CurrencyCode: "USD"}); err != nil {
		t.Fatalf("create parent order: %v", err)
	}

	// BulkCreate
	p1, p2 := uuid.New(), uuid.New()
	batch := []models.OrderItem{
		{OrderID: orderID, ProductID: p1, Quantity: 2, UnitPriceCents: 500, LineTotalCents: 1000, CurrencyCode: "USD"},
		{OrderID: orderID, ProductID: p2, Quantity: 1, UnitPriceCents: 700, LineTotalCents: 700, CurrencyCode: "USD"},
	}
	if err := items.BulkCreate(ctx, batch); err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}

	// GetByOrderID
	got, err := items.GetByOrderID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetByOrderID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items got %d", len(got))
	}

	// SumByOrder
	total, currency, err := items.SumByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("SumByOrder: %v", err)
	}
	if total != 1700 || currency != "USD" {
		t.Fatalf("SumByOrder mismatch: total=%d curr=%s", total, currency)
	}

	// DeleteByOrderID
	deleted, err := items.DeleteByOrderID(ctx, orderID)
	if err != nil {
		t.Fatalf("DeleteByOrderID: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted expected 2 got %d", deleted)
	}

	// Delete again should return 0
	deleted2, err := items.DeleteByOrderID(ctx, orderID)
	if err != nil {
		t.Fatalf("DeleteByOrderID second: %v", err)
	}
	if deleted2 != 0 {
		t.Fatalf("deleted second expected 0 got %d", deleted2)
	}
}

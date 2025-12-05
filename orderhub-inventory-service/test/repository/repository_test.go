package repository_test

import (
	"context"
	"testing"
	"time"

	"inventory-service/internal/migrate"
	"inventory-service/internal/models"
	"inventory-service/internal/repository"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/testutil"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.SetupTestPostgres(t)
	if err := migrate.MigrateInventoryDB(context.Background(), db, zap.NewNop(), migrate.DefaultMigrateOptions()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestProductRepo_CRUD(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Create
	product := &models.Product{
		VendorID:     vendorID,
		SKU:          "SKU-001",
		Name:         "Test Product",
		Description:  "Test Description",
		PriceCents:   10000,
		CurrencyCode: "RUB",
		IsActive:     true,
	}

	if err := repo.Create(ctx, product); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// GetByID
	got, err := repo.GetByID(ctx, product.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.SKU != "SKU-001" || got.Name != "Test Product" {
		t.Fatalf("GetByID mismatch: %+v", got)
	}

	// GetByVendorAndSKU
	gotBySKU, err := repo.GetByVendorAndSKU(ctx, vendorID, "SKU-001")
	if err != nil {
		t.Fatalf("GetByVendorAndSKU: %v", err)
	}
	if gotBySKU == nil || gotBySKU.ID != product.ID {
		t.Fatalf("GetByVendorAndSKU mismatch: %+v", gotBySKU)
	}

	// UpdateFields
	if err := repo.UpdateFields(ctx, product.ID, map[string]any{
		"name":        "Updated Product",
		"price_cents": int64(15000),
	}); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}

	updated, _ := repo.GetByID(ctx, product.ID)
	if updated.Name != "Updated Product" || updated.PriceCents != 15000 {
		t.Fatalf("UpdateFields mismatch: %+v", updated)
	}

	// EnsureInventoryRow
	if err := repo.EnsureInventoryRow(ctx, product.ID); err != nil {
		t.Fatalf("EnsureInventoryRow: %v", err)
	}

	// Проверяем, что инвентарная запись создана
	invRepo := repository.NewInventoryRepo(db)
	inv, err := invRepo.Get(ctx, product.ID)
	if err != nil {
		t.Fatalf("Get inventory: %v", err)
	}
	if inv == nil {
		t.Fatal("inventory row not created")
	}

	// Delete
	deleted, err := repo.Delete(ctx, product.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	// Delete again should return false
	deleted2, err := repo.Delete(ctx, product.ID)
	if err != nil {
		t.Fatalf("Delete second: %v", err)
	}
	if deleted2 {
		t.Fatal("expected deleted2=false")
	}
}

func TestProductRepo_List(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем несколько продуктов
	products := []models.Product{
		{VendorID: vendorID, SKU: "SKU-A", Name: "Apple Product", PriceCents: 1000, CurrencyCode: "RUB", IsActive: true},
		{VendorID: vendorID, SKU: "SKU-B", Name: "Banana Product", PriceCents: 2000, CurrencyCode: "RUB", IsActive: true},
		{VendorID: vendorID, SKU: "SKU-C", Name: "Cherry Product", PriceCents: 3000, CurrencyCode: "RUB", IsActive: true},
	}

	for i := range products {
		if err := repo.Create(ctx, &products[i]); err != nil {
			t.Fatalf("Create product %d: %v", i, err)
		}
	}

	// Деактивируем третий продукт явно через UpdateFields
	if err := repo.UpdateFields(ctx, products[2].ID, map[string]any{"is_active": false}); err != nil {
		t.Fatalf("Deactivate product: %v", err)
	}

	// List all for vendor
	list, total, err := repo.List(ctx, repository.ProductListFilter{
		VendorID: &vendorID,
		Limit:    10,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 3 {
		t.Fatalf("expected total>=3, got %d", total)
	}
	if len(list) < 3 {
		t.Fatalf("expected len>=3, got %d", len(list))
	}

	// List only active
	activeTrue := true
	listActive, totalActive, err := repo.List(ctx, repository.ProductListFilter{
		VendorID:   &vendorID,
		OnlyActive: &activeTrue,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if totalActive != 2 {
		t.Fatalf("expected totalActive=2, got %d", totalActive)
	}
	if len(listActive) != 2 {
		t.Fatalf("expected listActive len=2, got %d", len(listActive))
	}

	// List with query search
	listSearch, totalSearch, err := repo.List(ctx, repository.ProductListFilter{
		VendorID: &vendorID,
		Query:    "apple",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("List search: %v", err)
	}
	if totalSearch != 1 {
		t.Fatalf("expected totalSearch=1, got %d", totalSearch)
	}
	if len(listSearch) != 1 {
		t.Fatalf("expected listSearch len=1, got %d", len(listSearch))
	}

	// Test pagination
	listPage, totalPage, err := repo.List(ctx, repository.ProductListFilter{
		VendorID: &vendorID,
		Limit:    2,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("List page: %v", err)
	}
	if totalPage != 3 {
		t.Fatalf("expected totalPage=3, got %d", totalPage)
	}
	if len(listPage) != 2 {
		t.Fatalf("expected listPage len=2, got %d", len(listPage))
	}
}

func TestProductRepo_BatchGetByIDs(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем несколько продуктов
	p1 := models.Product{VendorID: vendorID, SKU: "P1", Name: "Product 1", CurrencyCode: "RUB"}
	p2 := models.Product{VendorID: vendorID, SKU: "P2", Name: "Product 2", CurrencyCode: "RUB"}

	if err := repo.Create(ctx, &p1); err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	if err := repo.Create(ctx, &p2); err != nil {
		t.Fatalf("Create p2: %v", err)
	}

	// BatchGetByIDs
	list, err := repo.BatchGetByIDs(ctx, []uuid.UUID{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("BatchGetByIDs: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected len=2, got %d", len(list))
	}

	// Empty IDs
	emptyList, err := repo.BatchGetByIDs(ctx, []uuid.UUID{})
	if err != nil {
		t.Fatalf("BatchGetByIDs empty: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected empty list, got %d", len(emptyList))
	}
}

func TestInventoryRepo_BasicOperations(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewInventoryRepo(db)
	prodRepo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем продукт и инвентарную запись
	product := models.Product{VendorID: vendorID, SKU: "INV-001", Name: "Inventory Test", CurrencyCode: "RUB"}
	if err := prodRepo.Create(ctx, &product); err != nil {
		t.Fatalf("Create product: %v", err)
	}
	if err := prodRepo.EnsureInventoryRow(ctx, product.ID); err != nil {
		t.Fatalf("EnsureInventoryRow: %v", err)
	}

	// Get
	inv, err := repo.Get(ctx, product.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if inv.Available != 0 || inv.Reserved != 0 {
		t.Fatalf("expected available=0, reserved=0, got %+v", inv)
	}

	// SetAvailable
	if err := repo.SetAvailable(ctx, product.ID, 100); err != nil {
		t.Fatalf("SetAvailable: %v", err)
	}

	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 100 {
		t.Fatalf("expected available=100, got %d", inv.Available)
	}

	// AdjustAvailable - увеличение
	ok, err := repo.AdjustAvailable(ctx, product.ID, 50)
	if err != nil {
		t.Fatalf("AdjustAvailable: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 150 {
		t.Fatalf("expected available=150, got %d", inv.Available)
	}

	// AdjustAvailable - уменьшение
	ok, err = repo.AdjustAvailable(ctx, product.ID, -50)
	if err != nil {
		t.Fatalf("AdjustAvailable decrease: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 100 {
		t.Fatalf("expected available=100, got %d", inv.Available)
	}

	// AdjustAvailable - попытка уйти в минус (должно вернуть false)
	ok, err = repo.AdjustAvailable(ctx, product.ID, -200)
	if err != nil {
		t.Fatalf("AdjustAvailable negative: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for negative result")
	}

	// Проверяем, что значение не изменилось
	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 100 {
		t.Fatalf("expected available=100 unchanged, got %d", inv.Available)
	}
}

func TestInventoryRepo_Reservation(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewInventoryRepo(db)
	prodRepo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем продукт с начальным количеством
	product := models.Product{VendorID: vendorID, SKU: "RES-001", Name: "Reservation Test", CurrencyCode: "RUB"}
	if err := prodRepo.Create(ctx, &product); err != nil {
		t.Fatalf("Create product: %v", err)
	}
	if err := prodRepo.EnsureInventoryRow(ctx, product.ID); err != nil {
		t.Fatalf("EnsureInventoryRow: %v", err)
	}
	if err := repo.SetAvailable(ctx, product.ID, 100); err != nil {
		t.Fatalf("SetAvailable: %v", err)
	}

	// TryReserve - успешное резервирование
	ok, err := repo.TryReserve(ctx, product.ID, 30)
	if err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	if !ok {
		t.Fatal("expected TryReserve ok=true")
	}

	inv, _ := repo.Get(ctx, product.ID)
	if inv.Available != 70 || inv.Reserved != 30 {
		t.Fatalf("expected available=70, reserved=30, got %+v", inv)
	}

	// TryReserve - попытка зарезервировать больше, чем есть (должно вернуть false)
	ok, err = repo.TryReserve(ctx, product.ID, 100)
	if err != nil {
		t.Fatalf("TryReserve overflow: %v", err)
	}
	if ok {
		t.Fatal("expected TryReserve ok=false for overflow")
	}

	// Проверяем, что значения не изменились
	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 70 || inv.Reserved != 30 {
		t.Fatalf("expected unchanged available=70, reserved=30, got %+v", inv)
	}

	// Release - возврат резерва
	ok, err = repo.Release(ctx, product.ID, 10)
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if !ok {
		t.Fatal("expected Release ok=true")
	}

	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 80 || inv.Reserved != 20 {
		t.Fatalf("expected available=80, reserved=20, got %+v", inv)
	}

	// Release - попытка освободить больше, чем зарезервировано (должно вернуть false)
	ok, err = repo.Release(ctx, product.ID, 50)
	if err != nil {
		t.Fatalf("Release overflow: %v", err)
	}
	if ok {
		t.Fatal("expected Release ok=false for overflow")
	}

	// Confirm - подтверждение резерва (списание)
	ok, err = repo.Confirm(ctx, product.ID, 20)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !ok {
		t.Fatal("expected Confirm ok=true")
	}

	inv, _ = repo.Get(ctx, product.ID)
	if inv.Available != 80 || inv.Reserved != 0 {
		t.Fatalf("expected available=80, reserved=0, got %+v", inv)
	}

	// Confirm - попытка подтвердить больше, чем зарезервировано (должно вернуть false)
	ok, err = repo.Confirm(ctx, product.ID, 10)
	if err != nil {
		t.Fatalf("Confirm overflow: %v", err)
	}
	if ok {
		t.Fatal("expected Confirm ok=false when no reserved")
	}
}

func TestReservationRepo_Basic(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewReservationRepo(db)
	prodRepo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()
	orderID := uuid.New()

	// Создаем продукт
	product := models.Product{VendorID: vendorID, SKU: "RESV-001", Name: "Reservation Record Test", CurrencyCode: "RUB"}
	if err := prodRepo.Create(ctx, &product); err != nil {
		t.Fatalf("Create product: %v", err)
	}

	// UpsertPending
	if err := repo.UpsertPending(ctx, orderID, product.ID, 10); err != nil {
		t.Fatalf("UpsertPending: %v", err)
	}

	// Exists
	exists, err := repo.Exists(ctx, orderID, product.ID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}

	// ListByOrder
	list, err := repo.ListByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("ListByOrder: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected len=1, got %d", len(list))
	}
	if list[0].Status != models.ReservationPending || list[0].Quantity != 10 {
		t.Fatalf("expected status=PENDING, qty=10, got %+v", list[0])
	}

	// MarkReserved
	ok, err := repo.MarkReserved(ctx, orderID, product.ID)
	if err != nil {
		t.Fatalf("MarkReserved: %v", err)
	}
	if !ok {
		t.Fatal("expected MarkReserved ok=true")
	}

	list, _ = repo.ListByOrder(ctx, orderID)
	if list[0].Status != models.ReservationReserved {
		t.Fatalf("expected status=RESERVED, got %s", list[0].Status)
	}

	// MarkReleased
	ok, err = repo.MarkReleased(ctx, orderID, product.ID)
	if err != nil {
		t.Fatalf("MarkReleased: %v", err)
	}
	if !ok {
		t.Fatal("expected MarkReleased ok=true")
	}

	list, _ = repo.ListByOrder(ctx, orderID)
	if list[0].Status != models.ReservationReleased {
		t.Fatalf("expected status=RELEASED, got %s", list[0].Status)
	}

	// UpsertPending снова (должно обновить)
	if err := repo.UpsertPending(ctx, orderID, product.ID, 20); err != nil {
		t.Fatalf("UpsertPending update: %v", err)
	}

	list, _ = repo.ListByOrder(ctx, orderID)
	if list[0].Status != models.ReservationPending || list[0].Quantity != 20 {
		t.Fatalf("expected status=PENDING, qty=20 after upsert, got %+v", list[0])
	}

	// MarkFailed
	ok, err = repo.MarkFailed(ctx, orderID, product.ID)
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if !ok {
		t.Fatal("expected MarkFailed ok=true")
	}

	list, _ = repo.ListByOrder(ctx, orderID)
	if list[0].Status != models.ReservationFailed {
		t.Fatalf("expected status=FAILED, got %s", list[0].Status)
	}
}

func TestReservationRepo_BulkOperations(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewReservationRepo(db)
	prodRepo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()
	orderID := uuid.New()

	// Создаем несколько продуктов
	products := make([]models.Product, 3)
	for i := 0; i < 3; i++ {
		products[i] = models.Product{
			VendorID:     vendorID,
			SKU:          "BULK-" + string(rune('A'+i)),
			Name:         "Bulk Product",
			CurrencyCode: "RUB",
		}
		if err := prodRepo.Create(ctx, &products[i]); err != nil {
			t.Fatalf("Create product %d: %v", i, err)
		}
	}

	// UpsertPending для всех продуктов
	for i, p := range products {
		if err := repo.UpsertPending(ctx, orderID, p.ID, int32(10+i)); err != nil {
			t.Fatalf("UpsertPending %d: %v", i, err)
		}
	}

	// ListByOrder - должно вернуть 3 записи
	list, err := repo.ListByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("ListByOrder: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected len=3, got %d", len(list))
	}

	// Маркируем первую как RESERVED
	if _, err := repo.MarkReserved(ctx, orderID, products[0].ID); err != nil {
		t.Fatalf("MarkReserved: %v", err)
	}

	// ConfirmByOrder - должно обновить все записи на RESERVED
	count, err := repo.ConfirmByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("ConfirmByOrder: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count=3, got %d", count)
	}

	list, _ = repo.ListByOrder(ctx, orderID)
	for _, r := range list {
		if r.Status != models.ReservationReserved {
			t.Fatalf("expected all RESERVED, got %s", r.Status)
		}
	}

	// ReleaseByOrder - должно обновить все записи на RELEASED
	count, err = repo.ReleaseByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("ReleaseByOrder: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count=3, got %d", count)
	}

	list, _ = repo.ListByOrder(ctx, orderID)
	for _, r := range list {
		if r.Status != models.ReservationReleased {
			t.Fatalf("expected all RELEASED, got %s", r.Status)
		}
	}
}

func TestRepository_WithTx(t *testing.T) {
	db := setupDB(t)
	repo := repository.New(db)
	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем продукт
	product := models.Product{VendorID: vendorID, SKU: "TX-001", Name: "Transaction Test", CurrencyCode: "RUB"}
	if err := repo.Products.Create(ctx, &product); err != nil {
		t.Fatalf("Create product: %v", err)
	}
	if err := repo.Products.EnsureInventoryRow(ctx, product.ID); err != nil {
		t.Fatalf("EnsureInventoryRow: %v", err)
	}

	orderID := uuid.New()

	// Выполняем операции в транзакции
	err := repo.WithTx(func(tx *repository.Repository) error {
		// Резервируем товар
		if err := tx.Inventories.SetAvailable(ctx, product.ID, 100); err != nil {
			return err
		}
		ok, err := tx.Inventories.TryReserve(ctx, product.ID, 50)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("TryReserve failed in tx")
		}

		// Создаем запись резервирования
		if err := tx.Reservations.UpsertPending(ctx, orderID, product.ID, 50); err != nil {
			return err
		}
		if _, err := tx.Reservations.MarkReserved(ctx, orderID, product.ID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	// Проверяем результаты
	inv, _ := repo.Inventories.Get(ctx, product.ID)
	if inv.Available != 50 || inv.Reserved != 50 {
		t.Fatalf("expected available=50, reserved=50, got %+v", inv)
	}

	list, _ := repo.Reservations.ListByOrder(ctx, orderID)
	if len(list) != 1 || list[0].Status != models.ReservationReserved {
		t.Fatalf("expected 1 RESERVED reservation, got %+v", list)
	}
}

func TestRepository_WithTx_Rollback(t *testing.T) {
	db := setupDB(t)
	repo := repository.New(db)
	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем продукт
	product := models.Product{VendorID: vendorID, SKU: "TX-002", Name: "Rollback Test", CurrencyCode: "RUB"}
	if err := repo.Products.Create(ctx, &product); err != nil {
		t.Fatalf("Create product: %v", err)
	}
	if err := repo.Products.EnsureInventoryRow(ctx, product.ID); err != nil {
		t.Fatalf("EnsureInventoryRow: %v", err)
	}
	if err := repo.Inventories.SetAvailable(ctx, product.ID, 100); err != nil {
		t.Fatalf("SetAvailable: %v", err)
	}

	orderID := uuid.New()

	// Выполняем транзакцию с ошибкой (должна откатиться)
	err := repo.WithTx(func(tx *repository.Repository) error {
		ok, err := tx.Inventories.TryReserve(ctx, product.ID, 30)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("TryReserve failed in tx")
		}

		if err := tx.Reservations.UpsertPending(ctx, orderID, product.ID, 30); err != nil {
			return err
		}

		// Возвращаем ошибку для отката
		return gorm.ErrInvalidTransaction
	})

	if err == nil {
		t.Fatal("expected error from WithTx")
	}

	// Проверяем, что изменения не применились
	inv, _ := repo.Inventories.Get(ctx, product.ID)
	if inv.Available != 100 || inv.Reserved != 0 {
		t.Fatalf("expected rollback: available=100, reserved=0, got %+v", inv)
	}

	exists, _ := repo.Reservations.Exists(ctx, orderID, product.ID)
	if exists {
		t.Fatal("expected no reservation after rollback")
	}
}

func TestProductRepo_CaseInsensitiveSKU(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем продукт
	product := models.Product{VendorID: vendorID, SKU: "SKU-Case", Name: "Case Test", CurrencyCode: "RUB"}
	if err := repo.Create(ctx, &product); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Ищем в разных регистрах
	got1, err := repo.GetByVendorAndSKU(ctx, vendorID, "sku-case")
	if err != nil {
		t.Fatalf("GetByVendorAndSKU lowercase: %v", err)
	}
	if got1 == nil || got1.ID != product.ID {
		t.Fatalf("expected to find product by lowercase SKU")
	}

	got2, err := repo.GetByVendorAndSKU(ctx, vendorID, "SKU-CASE")
	if err != nil {
		t.Fatalf("GetByVendorAndSKU uppercase: %v", err)
	}
	if got2 == nil || got2.ID != product.ID {
		t.Fatalf("expected to find product by uppercase SKU")
	}

	got3, err := repo.GetByVendorAndSKU(ctx, vendorID, "SkU-CaSe")
	if err != nil {
		t.Fatalf("GetByVendorAndSKU mixedcase: %v", err)
	}
	if got3 == nil || got3.ID != product.ID {
		t.Fatalf("expected to find product by mixedcase SKU")
	}
}

func TestInventoryRepo_UpdatedAtTrigger(t *testing.T) {
	db := setupDB(t)
	repo := repository.NewInventoryRepo(db)
	prodRepo := repository.NewProductRepo(db)

	ctx := context.Background()
	vendorID := uuid.New()

	// Создаем продукт и инвентарную запись
	product := models.Product{VendorID: vendorID, SKU: "UPD-001", Name: "UpdatedAt Test", CurrencyCode: "RUB"}
	if err := prodRepo.Create(ctx, &product); err != nil {
		t.Fatalf("Create product: %v", err)
	}
	if err := prodRepo.EnsureInventoryRow(ctx, product.ID); err != nil {
		t.Fatalf("EnsureInventoryRow: %v", err)
	}

	inv1, _ := repo.Get(ctx, product.ID)
	initialUpdatedAt := inv1.UpdatedAt

	// Ждем немного и обновляем
	time.Sleep(100 * time.Millisecond)

	if err := repo.SetAvailable(ctx, product.ID, 50); err != nil {
		t.Fatalf("SetAvailable: %v", err)
	}

	inv2, _ := repo.Get(ctx, product.ID)
	if !inv2.UpdatedAt.After(initialUpdatedAt) {
		t.Fatalf("expected UpdatedAt to be updated, initial=%v, after=%v", initialUpdatedAt, inv2.UpdatedAt)
	}
}

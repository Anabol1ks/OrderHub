package migrate

import (
	"context"
	"inventory-service/internal/models" // ← поправь импорт под твой модуль

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrateOptions struct {
	CreateExtensions       bool // pgcrypto, uuid-ossp, pg_trgm
	CreateChecks           bool // CHECK-constraint'ы
	CreateIndexes          bool // индексы и UNIQUE
	CreateFKsViaSQL        bool // FK через Exec после AutoMigrate
	CreateUpdatedAtTrigger bool // триггеры updated_at
	CreateSearchIndexes    bool // GIN trgm для поиска по name/sku
}

func DefaultMigrateOptions() MigrateOptions {
	return MigrateOptions{
		CreateExtensions:       true,
		CreateChecks:           true,
		CreateIndexes:          true,
		CreateFKsViaSQL:        true,
		CreateUpdatedAtTrigger: true,
		CreateSearchIndexes:    true,
	}
}

func MigrateInventoryDB(ctx context.Context, db *gorm.DB, log *zap.Logger, opt MigrateOptions) error {
	log.Info("Начало миграции базы каталога/склада")

	// Расширения
	if opt.CreateExtensions {
		log.Info("Создание расширений PostgreSQL")
		if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
			log.Error("pgcrypto error", zap.Error(err))
			return err
		}
		if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
			log.Error("uuid-ossp error", zap.Error(err))
			return err
		}
		if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm`).Error; err != nil {
			log.Error("pg_trgm error", zap.Error(err))
			return err
		}
		log.Info("Расширения созданы")
	}

	// Таблицы
	log.Info("Создание таблиц: products, inventories, reservations")
	if err := db.AutoMigrate(&models.Product{}, &models.Inventory{}, &models.Reservation{}); err != nil {
		log.Error("AutoMigrate error", zap.Error(err))
		return err
	}
	log.Info("Таблицы созданы")

	// Триггеры updated_at
	if opt.CreateUpdatedAtTrigger {
		log.Info("Создание триггеров updated_at")
		if err := db.Exec(`
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_products_updated ON products;
CREATE TRIGGER trg_products_updated BEFORE UPDATE ON products
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_inventories_updated ON inventories;
CREATE TRIGGER trg_inventories_updated BEFORE UPDATE ON inventories
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
`).Error; err != nil {
			log.Error("triggers error", zap.Error(err))
			return err
		}
		log.Info("Триггеры созданы")
	}

	// CHECK-и
	if opt.CreateChecks {
		log.Info("Создание CHECK-ограничений")

		// Валюта — строго 'RUB'
		if err := db.Exec(`
ALTER TABLE products
	DROP CONSTRAINT IF EXISTS chk_products_currency_code_rub,
	ADD CONSTRAINT chk_products_currency_code_rub
	CHECK (currency_code = 'RUB' AND char_length(currency_code) = 3);
`).Error; err != nil {
			log.Error("chk currency_code", zap.Error(err))
			return err
		}

		// Цена и количества — неотрицательные / > 0
		if err := db.Exec(`
ALTER TABLE products
	DROP CONSTRAINT IF EXISTS chk_products_price_non_negative,
	ADD CONSTRAINT chk_products_price_non_negative
	CHECK (price_cents >= 0);
`).Error; err != nil {
			log.Error("chk price", zap.Error(err))
			return err
		}

		if err := db.Exec(`
ALTER TABLE inventories
	DROP CONSTRAINT IF EXISTS chk_inventories_non_negative,
	ADD CONSTRAINT chk_inventories_non_negative
	CHECK (available >= 0 AND reserved >= 0);
`).Error; err != nil {
			log.Error("chk inventories", zap.Error(err))
			return err
		}

		if err := db.Exec(`
ALTER TABLE reservations
	DROP CONSTRAINT IF EXISTS chk_reservations_quantity_gt_zero,
	ADD CONSTRAINT chk_reservations_quantity_gt_zero
	CHECK (quantity > 0);
`).Error; err != nil {
			log.Error("chk reservations.qty", zap.Error(err))
			return err
		}

		// Допустимые статусы
		if err := db.Exec(`
ALTER TABLE reservations
	DROP CONSTRAINT IF EXISTS chk_reservations_status_allowed,
	ADD CONSTRAINT chk_reservations_status_allowed
	CHECK (status IN ('PENDING','RESERVED','RELEASED','FAILED'));
`).Error; err != nil {
			log.Error("chk reservations.status", zap.Error(err))
			return err
		}

		log.Info("CHECK-и созданы")
	}

	// Индексы и уникальности
	if opt.CreateIndexes {
		log.Info("Создание индексов и уникальностей")

		// Уникальность SKU в разрезе продавца, кейс-инсensitive: (vendor_id, lower(sku))
		if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS ux_products_vendor_sku
ON products (vendor_id, lower(sku));
`).Error; err != nil {
			log.Error("ux vendor_sku", zap.Error(err))
			return err
		}

		// 1:1 inventory с продуктом — PK уже product_id, но FK создадим ниже
		// Резервации: уникальность на (order_id, product_id) для идемпотентности
		if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS ux_reservations_order_product
ON reservations (order_id, product_id);
`).Error; err != nil {
			log.Error("ux reservations order_product", zap.Error(err))
			return err
		}

		// Поиск и сортировки
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS ix_products_vendor_created
ON products (vendor_id, created_at DESC);
`).Error; err != nil {
			log.Error("ix products vendor_created", zap.Error(err))
			return err
		}
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS ix_products_active_created
ON products (is_active, created_at DESC);
`).Error; err != nil {
			log.Error("ix products active_created", zap.Error(err))
			return err
		}

		log.Info("Индексы созданы")
	}

	// Индексы для поиска (trgm) — опционально
	if opt.CreateSearchIndexes {
		log.Info("Создание GIN(trgm) индексов для поиска")
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS gin_products_name_trgm
ON products USING gin (name gin_trgm_ops);
`).Error; err != nil {
			log.Error("gin name", zap.Error(err))
			return err
		}
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS gin_products_sku_trgm
ON products USING gin (sku gin_trgm_ops);
`).Error; err != nil {
			log.Error("gin sku", zap.Error(err))
			return err
		}
		log.Info("GIN индексы созданы")
	}

	// Внешние ключи
	if opt.CreateFKsViaSQL {
		log.Info("Создание внешних ключей")

		// inventories.product_id -> products.id (CASCADE)
		if err := db.Exec(`
ALTER TABLE inventories
  DROP CONSTRAINT IF EXISTS fk_inventories_product,
  ADD CONSTRAINT fk_inventories_product
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;
`).Error; err != nil {
			log.Error("fk inventories.product_id", zap.Error(err))
			return err
		}

		// reservations.product_id -> products.id (RESTRICT/NO ACTION)
		if err := db.Exec(`
ALTER TABLE reservations
  DROP CONSTRAINT IF EXISTS fk_reservations_product,
  ADD CONSTRAINT fk_reservations_product
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT;
`).Error; err != nil {
			log.Error("fk reservations.product_id", zap.Error(err))
			return err
		}

		log.Info("Внешние ключи созданы")
	}

	log.Info("Миграция базы каталога/склада успешно завершена")
	return nil
}

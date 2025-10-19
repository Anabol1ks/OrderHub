package migrate

import (
	"context"
	"order-service/internal/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrateOptions struct {
	CreateExtensions       bool // pgcrypto, uuid-ossp, pg_trgm
	CreateChecks           bool // CHECK-constraint для целостности
	CreateIndexes          bool // индексы и UNIQUE
	CreateFKsViaSQL        bool // FK через SQL (поверх GORM-constraint)
	CreateUpdatedAtTrigger bool // триггер обновления updated_at
}

func DefaultMigrateOptions() MigrateOptions {
	return MigrateOptions{
		CreateExtensions:       true,
		CreateChecks:           true,
		CreateIndexes:          true,
		CreateFKsViaSQL:        true,
		CreateUpdatedAtTrigger: true,
	}
}

func MigrateOrderDB(ctx context.Context, db *gorm.DB, log *zap.Logger, opt MigrateOptions) error {
	log.Info("Начало миграции базы данных заказов")

	// Расширения
	if opt.CreateExtensions {
		log.Info("Создание расширений PostgreSQL")
		if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
			log.Error("Не удалось включить расширение pgcrypto", zap.Error(err))
			return err
		}
		if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
			log.Error("Не удалось включить расширение uuid-ossp", zap.Error(err))
			return err
		}
		if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm`).Error; err != nil {
			log.Error("Не удалось включить расширение pg_trgm", zap.Error(err))
			return err
		}
		log.Info("Расширения PostgreSQL успешно созданы")
	}

	// Таблицы
	log.Info("Создание таблиц orders и order_items")
	if err := db.AutoMigrate(&models.Order{}, &models.OrderItem{}); err != nil {
		log.Error("Не удалось создать таблицы", zap.Error(err))
		return err
	}
	log.Info("Таблицы успешно созданы")

	// Триггер updated_at для orders
	if opt.CreateUpdatedAtTrigger {
		log.Info("Создание триггера updated_at для orders")
		if err := db.Exec(`
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_orders_updated ON orders;
CREATE TRIGGER trg_orders_updated
BEFORE UPDATE ON orders
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
`).Error; err != nil {
			log.Error("Не удалось создать триггер updated_at", zap.Error(err))
			return err
		}
		log.Info("Триггер updated_at успешно создан")
	}

	// CHECK-constraint
	if opt.CreateChecks {
		log.Info("Создание CHECK-ограничений")

		// Статусы (так как храним TEXT)
		if err := db.Exec(`
ALTER TABLE orders
  DROP CONSTRAINT IF EXISTS chk_orders_status_allowed;
ALTER TABLE orders
  ADD CONSTRAINT chk_orders_status_allowed
  CHECK (status IN ('ORDER_STATUS_PENDING','ORDER_STATUS_CONFIRMED','ORDER_STATUS_CANCELLED'));
`).Error; err != nil {
			log.Error("Не удалось создать CHECK для статусов", zap.Error(err))
			return err
		}

		// Валюта (ровно 3 символа) — orders
		if err := db.Exec(`
ALTER TABLE orders
  DROP CONSTRAINT IF EXISTS chk_orders_currency_code_len;
ALTER TABLE orders
  ADD CONSTRAINT chk_orders_currency_code_len
  CHECK (char_length(currency_code) = 3);
`).Error; err != nil {
			log.Error("Не удалось создать CHECK для orders.currency_code", zap.Error(err))
			return err
		}

		// Валюта (ровно 3 символа) — order_items
		if err := db.Exec(`
ALTER TABLE order_items
  DROP CONSTRAINT IF EXISTS chk_order_items_currency_code_len;
ALTER TABLE order_items
  ADD CONSTRAINT chk_order_items_currency_code_len
  CHECK (char_length(currency_code) = 3);
`).Error; err != nil {
			log.Error("Не удалось создать CHECK для order_items.currency_code", zap.Error(err))
			return err
		}

		// Количество > 0
		if err := db.Exec(`
ALTER TABLE order_items
  DROP CONSTRAINT IF EXISTS chk_order_items_quantity_gt_zero;
ALTER TABLE order_items
  ADD CONSTRAINT chk_order_items_quantity_gt_zero
  CHECK (quantity > 0);
`).Error; err != nil {
			log.Error("Не удалось создать CHECK для order_items.quantity", zap.Error(err))
			return err
		}

		// Цены неотрицательные
		if err := db.Exec(`
ALTER TABLE order_items
  DROP CONSTRAINT IF EXISTS chk_order_items_prices_non_negative;
ALTER TABLE order_items
  ADD CONSTRAINT chk_order_items_prices_non_negative
  CHECK (unit_price_cents >= 0 AND line_total_cents >= 0);
`).Error; err != nil {
			log.Error("Не удалось создать CHECK для цен в order_items", zap.Error(err))
			return err
		}
		if err := db.Exec(`
ALTER TABLE orders
  DROP CONSTRAINT IF EXISTS chk_orders_total_price_non_negative;
ALTER TABLE orders
  ADD CONSTRAINT chk_orders_total_price_non_negative
  CHECK (total_price_cents >= 0);
`).Error; err != nil {
			log.Error("Не удалось создать CHECK для orders.total_price_cents", zap.Error(err))
			return err
		}

		log.Info("CHECK-ограничения успешно созданы")
	}

	// Индексы
	if opt.CreateIndexes {
		log.Info("Создание индексов")

		// Композитный UNIQUE(order_id, product_id) на случай если тегами не создался
		if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS ux_order_items_order_product
ON order_items (order_id, product_id);
`).Error; err != nil {
			log.Error("Не удалось создать уникальный индекс ux_order_items_order_product", zap.Error(err))
			return err
		}

		// Для выборок: заказы пользователя по дате
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS ix_orders_user_created
ON orders (user_id, created_at DESC);
`).Error; err != nil {
			log.Error("Не удалось создать индекс ix_orders_user_created", zap.Error(err))
			return err
		}

		// Для выборок по статусу
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS ix_orders_status_created
ON orders (status, created_at DESC);
`).Error; err != nil {
			log.Error("Не удалось создать индекс ix_orders_status_created", zap.Error(err))
			return err
		}

		log.Info("Индексы успешно созданы")
	}

	// Внешние ключи
	if opt.CreateFKsViaSQL {
		log.Info("Создание внешних ключей")

		// order_items.order_id -> orders.id (CASCADE)
		if err := db.Exec(`
ALTER TABLE order_items
  DROP CONSTRAINT IF EXISTS fk_order_items_order,
  ADD CONSTRAINT fk_order_items_order
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE;
`).Error; err != nil {
			log.Error("Не удалось создать FK order_items.order_id -> orders.id", zap.Error(err))
			return err
		}

		log.Info("Внешние ключи успешно созданы")
	}

	log.Info("Миграция базы данных заказов успешно завершена")
	return nil
}

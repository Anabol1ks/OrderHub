package migrate

import (
	"auth-service/internal/models"
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrateOptions struct {
	WithJWK             bool // хранить ключи в БД
	WithEmailFlows      bool // email_verifications
	WithPasswordReset   bool // password_reset_tokens
	WithSessions        bool // user_sessions
	CreateFunctionalIdx bool // lower(email) уникальный индекс
	CreateFKsViaSQL     bool // создадим FK через Exec после AutoMigrate
}

func DefaultMigrateOptions() MigrateOptions {
	return MigrateOptions{
		WithJWK:             true,
		WithEmailFlows:      true,
		WithPasswordReset:   true,
		WithSessions:        true,
		CreateFunctionalIdx: true,
		CreateFKsViaSQL:     true,
	}
}

func MigrateAuthDB(ctx context.Context, db *gorm.DB, log *zap.Logger, opt MigrateOptions) error {
	log.Info("Начало миграции базы данных аутентификации")

	// Расширения (генераторы UUID, крипта, триграммы)
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

	// Базовые таблицы
	log.Info("Создание базовых таблиц")
	modelsAny := []any{
		&models.User{},
		&models.RefreshToken{},
	}
	if err := db.AutoMigrate(modelsAny...); err != nil {
		log.Error("Не удалось создать базовые таблицы", zap.Error(err))
		return err
	}
	log.Info("Базовые таблицы успешно созданы")

	// Опциональные
	log.Info("Создание опциональных таблиц",
		zap.Bool("withJWK", opt.WithJWK),
		zap.Bool("withEmailFlows", opt.WithEmailFlows),
		zap.Bool("withPasswordReset", opt.WithPasswordReset),
		zap.Bool("withSessions", opt.WithSessions))

	if opt.WithJWK {
		if err := db.AutoMigrate(&models.JwkKey{}); err != nil {
			log.Error("Не удалось создать таблицу JWK ключей", zap.Error(err))
			return err
		}
		log.Info("Таблица JWK ключей создана")
	}
	if opt.WithEmailFlows {
		if err := db.AutoMigrate(&models.EmailVerification{}); err != nil {
			log.Error("Не удалось создать таблицу верификации email", zap.Error(err))
			return err
		}
		log.Info("Таблица верификации email создана")
	}
	if opt.WithPasswordReset {
		if err := db.AutoMigrate(&models.PasswordResetToken{}); err != nil {
			log.Error("Не удалось создать таблицу токенов сброса пароля", zap.Error(err))
			return err
		}
		log.Info("Таблица токенов сброса пароля создана")
	}
	if opt.WithSessions {
		if err := db.AutoMigrate(&models.UserSession{}); err != nil {
			log.Error("Не удалось создать таблицу пользовательских сессий", zap.Error(err))
			return err
		}
		log.Info("Таблица пользовательских сессий создана")
	}

	// Триггер updated_at
	log.Info("Создание триггера updated_at")
	if err := db.Exec(`
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_users_updated ON users;
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
`).Error; err != nil {
		log.Error("Не удалось создать триггер updated_at", zap.Error(err))
		return err
	}
	log.Info("Триггер updated_at успешно создан")

	// Функциональный уникальный индекс на email (lower(email))
	if opt.CreateFunctionalIdx {
		log.Info("Создание уникального индекса на lower(email)")
		if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_users_email ON users (lower(email))`).Error; err != nil {
			log.Error("Не удалось создать уникальный индекс на lower(email)", zap.Error(err))
			return err
		}
		log.Info("Уникальный индекс на lower(email) создан")
	}

	// Внешние ключи через SQL
	if opt.CreateFKsViaSQL {
		log.Info("Создание внешних ключей")
		if err := db.Exec(`
ALTER TABLE refresh_tokens
  DROP CONSTRAINT IF EXISTS fk_refresh_user,
  ADD CONSTRAINT fk_refresh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
`).Error; err != nil {
			log.Error("Не удалось создать FK refresh_tokens.user_id -> users.id", zap.Error(err))
			return err
		}

		if opt.WithEmailFlows {
			if err := db.Exec(`
ALTER TABLE email_verifications
  DROP CONSTRAINT IF EXISTS fk_emailv_user,
  ADD CONSTRAINT fk_emailv_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
`).Error; err != nil {
				log.Error("Не удалось создать FK email_verifications.user_id -> users.id", zap.Error(err))
				return err
			}
		}

		if opt.WithPasswordReset {
			if err := db.Exec(`
ALTER TABLE password_reset_tokens
  DROP CONSTRAINT IF EXISTS fk_pr_user,
  ADD CONSTRAINT fk_pr_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
`).Error; err != nil {
				log.Error("Не удалось создать FK password_reset_tokens.user_id -> users.id", zap.Error(err))
				return err
			}
		}

		if opt.WithSessions {
			if err := db.Exec(`
ALTER TABLE user_sessions
  DROP CONSTRAINT IF EXISTS fk_sessions_user,
  ADD CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
`).Error; err != nil {
				log.Error("Не удалось создать FK user_sessions.user_id -> users.id", zap.Error(err))
				return err
			}
		}
		log.Info("Внешние ключи успешно созданы")
	}

	log.Info("Миграция базы данных аутентификации успешно завершена")
	return nil
}

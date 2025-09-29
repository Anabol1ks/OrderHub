package database

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config представляет конфигурацию для подключения к базе данных
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// NewConfig создает новую конфигурацию базы данных
func NewConfig(host, port, user, password, name, sslMode string) *Config {
	return &Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Name:     name,
		SSLMode:  sslMode,
	}
}

// DBConfigInterface интерфейс для любой структуры конфигурации БД
type DBConfigInterface interface {
	GetHost() string
	GetPort() string
	GetUser() string
	GetPassword() string
	GetName() string
	GetSSLMode() string
}

// NewConfigFromInterface создает Config из любой структуры, реализующей DBConfigInterface
func NewConfigFromInterface(cfg DBConfigInterface) *Config {
	return &Config{
		Host:     cfg.GetHost(),
		Port:     cfg.GetPort(),
		User:     cfg.GetUser(),
		Password: cfg.GetPassword(),
		Name:     cfg.GetName(),
		SSLMode:  cfg.GetSSLMode(),
	}
}

func ConnectDB(cfg *Config, log *zap.Logger) *gorm.DB {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{PrepareStmt: false})
	if err != nil {
		log.Fatal("Не удалось подключиться к базе данных", zap.Error(err))
		return nil
	}

	log.Info("Подключение к базе данных успешно установлено")
	return db
}

// ConnectDBForMigration подключается к БД с настройками для миграций
func ConnectDBForMigration(cfg *Config, log *zap.Logger) *gorm.DB {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt:                              false,
		DisableForeignKeyConstraintWhenMigrating: true, // FK создадим вручную
	})
	if err != nil {
		log.Fatal("Не удалось подключиться к базе данных для миграции", zap.Error(err))
		return nil
	}

	log.Info("Подключение к базе данных для миграции успешно установлено")
	return db
}

func CloseDB(db *gorm.DB, log *zap.Logger) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("Не удалось получить объект sql.DB для закрытия", zap.Error(err))
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Error("Ошибка при закрытии соединения с БД", zap.Error(err))
	} else {
		log.Info("Соединение с БД закрыто")
	}
}

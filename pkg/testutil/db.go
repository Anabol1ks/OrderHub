package testutil

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
	postgres_gorm "gorm.io/driver/postgres"

	"gorm.io/gorm"
)

// SetupTestPostgres запускает PostgreSQL в контейнере и возвращает *gorm.DB.
// Миграции выполняются явно в тесте (вызовите migrate.MigrateAuthDB или другую
// функцию миграции после получения db).
func SetupTestPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:17",

		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	// Terminate контейнер один раз в t.Cleanup (без двойного вызова и без немедленного остановa)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	dsn, err := pgContainer.ConnectionString(ctx,
		"sslmode=disable",
		"TimeZone=UTC",
	)
	if err != nil {
		t.Fatalf("failed to get dsn: %v", err)
	}

	db, err := gorm.Open(postgres_gorm.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect gorm: %v", err)
	}

	// Миграции выполняются явно в тесте.
	return db
}

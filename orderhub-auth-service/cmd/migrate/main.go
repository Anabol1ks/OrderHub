package main

import (
	"auth-service/config"
	"auth-service/internal/migrate"
	"context"
	"os"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/database"
	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()
	isDev := os.Getenv("ENV") == "development"
	if err := logger.Init(isDev); err != nil {
		panic(err)
	}

	defer logger.Sync()

	log := logger.L()

	cfg := config.Load(log)

	db := database.ConnectDBForMigration(&cfg.DB.Config, log)
	defer database.CloseDB(db, log)

	ctx := context.Background()
	opts := migrate.DefaultMigrateOptions()

	if err := migrate.MigrateAuthDB(ctx, db, log, opts); err != nil {
		log.Fatal("Ошибка при выполнении миграции", zap.Error(err))
	}

	log.Info("Миграция успешно завершена")
}

package main

import (
	"order-service/config"
	"os"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/database"
	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/logger"
	"github.com/joho/godotenv"
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
	db := database.ConnectDB(&cfg.DB.Config, log)
	defer database.CloseDB(db, log)
}

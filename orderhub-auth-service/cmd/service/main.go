package main

import (
	"auth-service/config"
	"os"

	"orderhub-utils-go/logger"

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
	log.Info("Loaded configuration", zap.Any("config", cfg))
}

package main

import (
	"inventory-service/config"
	"inventory-service/internal/repository"
	"inventory-service/internal/service"
	"inventory-service/internal/transport/grpc"
	"os"
	"os/signal"
	"syscall"

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
	log.Info("cfg: ", zap.Any("config", cfg))
	db := database.ConnectDB(&cfg.DB.Config, log)
	defer database.CloseDB(db, log)

	repos := repository.New(db)
	svc := service.NewInventoryService(repos)
	_ = grpc.NewHandler(svc)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("Shutting down Order gRPC server...")
	log.Info("Order gRPC server stopped gracefully")
}

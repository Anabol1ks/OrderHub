package main

import (
	"auth-service/config"
	"net"
	"os"
	"os/signal"
	"syscall"

	"orderhub-utils-go/database"
	"orderhub-utils-go/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	lis, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("Starting gRPC server", zap.String("addr", cfg.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	<-quit
	log.Info("Shutting down gRPC server...")
	grpcServer.GracefulStop()
	log.Info("gRPC server stopped gracefully")
}

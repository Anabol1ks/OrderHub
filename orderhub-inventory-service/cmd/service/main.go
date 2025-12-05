package main

import (
	"inventory-service/config"
	"inventory-service/internal/repository"
	"inventory-service/internal/service"
	gtransport "inventory-service/internal/transport/grpc"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/database"
	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/logger"
	authv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1"
	inventoryv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/inventory/v1"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
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

	authConn, err := grpc.Dial(cfg.AuthAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("failed to connect to auth service", zap.Error(err))
	}
	defer authConn.Close()
	authClient := authv1.NewAuthServiceClient(authConn)

	svc := service.NewInventoryService(repos)

	lis, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	authInterceptor := gtransport.NewAuthUnaryServerInterceptor(authClient)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authInterceptor),
	)

	// Health server
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)

	// Reflection for local debugging
	reflection.Register(grpcServer)

	invServer := gtransport.NewHandler(svc)
	inventoryv1.RegisterInventoryServiceServer(grpcServer, invServer)

	go func() {
		log.Info("Starting Inventory gRPC server", zap.String("addr", cfg.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("Shutting down Inventory gRPC server...")
	log.Info("Inventory gRPC server stopped gracefully")
}

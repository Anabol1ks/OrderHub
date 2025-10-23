package main

import (
	"net"
	"order-service/config"
	"order-service/internal/repository"
	"order-service/internal/service"
	gtransport "order-service/internal/transport/grpc"
	"os"
	"os/signal"
	"syscall"

	authv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1"
	orderv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/order/v1"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/database"
	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/logger"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
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
	db := database.ConnectDB(&cfg.DB.Config, log)
	defer database.CloseDB(db, log)

	repos := repository.New(db)
	pricing := service.StaticPricing{}

	// Event bus is optional for now (nil disables publishing)
	svc := service.NewOrderService(repos, pricing, nil)

	lis, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	// Connect to Auth service for token introspection
	authConn, err := grpc.Dial(cfg.AuthAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("failed to connect to auth service", zap.Error(err))
	}
	defer authConn.Close()
	authClient := authv1.NewAuthServiceClient(authConn)

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

	// Register Order gRPC service
	orderServer := gtransport.NewOrderServer(svc)
	orderv1.RegisterOrderServiceServer(grpcServer, orderServer)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("Starting Order gRPC server", zap.String("addr", cfg.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	<-quit
	log.Info("Shutting down Order gRPC server...")
	grpcServer.GracefulStop()
	log.Info("Order gRPC server stopped gracefully")
}

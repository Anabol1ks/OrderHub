package main

import (
	"auth-service/config"
	"auth-service/internal/hashing"
	"auth-service/internal/repository"
	"auth-service/internal/service"
	"auth-service/internal/token"
	gtransport "auth-service/internal/transport/grpc"
	"net"
	authv1 "orderhub-proto/auth/v1"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orderhub-utils-go/database"
	"orderhub-utils-go/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	db := database.ConnectDB(&cfg.DB.Config, log)
	defer database.CloseDB(db, log)

	repos := repository.New(db)

	hasher := hashing.NewBcrypt(0)

	jwkStore := repos.JWKs
	tokens := token.NewRSAProvider(jwkStore, cfg.JWT.Issuer, cfg.JWT.Audience)

	authInterceptor := gtransport.NewAuthUnaryServerInterceptor(tokens)

	authSvc := service.NewAuthService(
		repos.Users, repos.RefreshTokens, repos.JWKs,
		hasher, tokens, repos.Session, repos.PasswordReset,
		time.Duration(cfg.JWT.AccessExp),
		time.Duration(cfg.JWT.RefreshExp),
	)

	lis, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authInterceptor),
	)

	// Health server
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)

	reflection.Register(grpcServer)

	// Регистрируем gRPC handler
	authServer := gtransport.NewAuthServer(authSvc, log)
	authv1.RegisterAuthServiceServer(grpcServer, authServer)

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

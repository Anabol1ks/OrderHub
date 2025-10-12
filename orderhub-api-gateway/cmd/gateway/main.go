package main

import (
	"api-gateway/config"
	_ "api-gateway/docs"
	"api-gateway/internal/auth"
	"api-gateway/internal/router"
	"os"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/logger"

	authv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// @Title OrderHub API
// @Version 1.0
// @Description API для управления заказами
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	_ = godotenv.Load()
	isDev := os.Getenv("ENV") == "development"
	if err := logger.Init(isDev); err != nil {
		panic(err)
	}

	defer logger.Sync()

	log := logger.L()

	cfg := config.Load(log)

	authConn, err := grpc.NewClient(
		cfg.AuthAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Error("auth service dial failed: ", zap.Error(err))
	}
	defer authConn.Close()

	rawAuthClient := authv1.NewAuthServiceClient(authConn)
	authClient := auth.NewClient(rawAuthClient)

	r := router.Router(authClient, log)

	if err := r.Run(":8080"); err != nil {
		log.Fatal("failed to run http server", zap.Error(err))
	}
}

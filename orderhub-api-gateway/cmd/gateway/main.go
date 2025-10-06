package main

import (
	"api-gateway/config"
	"context"
	"os"

	authv1 "orderhub-proto/auth/v1"
	"orderhub-utils-go/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	authConn, err := grpc.NewClient(
		cfg.AuthAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Error("auth service dial failed: ", zap.Error(err))
	}
	defer authConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)

	jwks, err := authClient.GetJwks(context.Background(), &authv1.GetJwksRequest{})
	if err != nil {
		log.Error("GetJwks error: ", zap.Error(err))
	}
	log.Info("JWKS keys loaded: ", zap.Any("keys", jwks.Keys))

}

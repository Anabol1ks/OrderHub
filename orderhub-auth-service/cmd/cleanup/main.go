package main

import (
	"auth-service/config"
	"auth-service/internal/cleanup"
	"context"
	"fmt"
	"os"

	"orderhub-utils-go/database"
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

	db := database.ConnectDB(&cfg.DB.Config, log)
	defer database.CloseDB(db, log)

	cleanupSvc := cleanup.NewCleanupService(db, log)

	ctx := context.Background()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "expired":
			log.Info("running expired tokens cleanup")
			if err := cleanupSvc.CleanupExpiredTokens(ctx); err != nil {
				log.Fatal("failed to cleanup expired tokens", zap.Error(err))
			}
		case "sessions":
			log.Info("running sessions cleanup")
			if err := cleanupSvc.CleanupOrphanedSessions(ctx); err != nil {
				log.Fatal("failed to cleanup orphaned sessions", zap.Error(err))
			}
			if err := cleanupSvc.CleanupOldSessions(ctx); err != nil {
				log.Fatal("failed to cleanup old sessions", zap.Error(err))
			}
		case "consumed":
			log.Info("running consumed tokens cleanup")
			if err := cleanupSvc.CleanupConsumedTokens(ctx); err != nil {
				log.Fatal("failed to cleanup consumed tokens", zap.Error(err))
			}
		case "all":
			fallthrough
		default:
			log.Info("running full cleanup")
			if err := cleanupSvc.RunFullCleanup(ctx); err != nil {
				log.Fatal("failed to run full cleanup", zap.Error(err))
			}
		}
	} else {
		fmt.Println("Usage: go run cmd/cleanup/main.go [expired|sessions|consumed|all]")
		fmt.Println("  expired  - cleanup expired tokens only")
		fmt.Println("  sessions - cleanup orphaned and old sessions")
		fmt.Println("  consumed - cleanup consumed tokens")
		fmt.Println("  all      - run full cleanup (default)")
		os.Exit(1)
	}

	log.Info("cleanup completed successfully")
}

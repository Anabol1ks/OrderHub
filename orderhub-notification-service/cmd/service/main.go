package main

import (
	"context"
	"notification-service/config"
	"notification-service/internal/consumer"
	"notification-service/internal/sender"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	emailSender := sender.NewEmailSender(cfg)

	if len(cfg.KafkaBrokers) == 0 {
		log.Fatal("no kafka brokers configured (KAFKA_BROKERS)")
	}

	cons := consumer.NewKafkaEmailConsumer(cfg.KafkaBrokers, cfg.KafkaGroupID, cfg.KafkaTopic, emailSender, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := cons.Run(ctx); err != nil {
			log.Error("consumer stopped", zap.Error(err))
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("shutdown signal received")
	cancel()
	_ = cons.Close()
	time.Sleep(200 * time.Millisecond)
}

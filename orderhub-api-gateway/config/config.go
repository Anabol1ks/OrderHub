package config

import (
	"os"

	"go.uber.org/zap"
)

type Config struct {
	AuthAddr string
}

func Load(log *zap.Logger) *Config {
	return &Config{
		AuthAddr: getEnv("AUTH_SERVICE_ADDR", log),
	}
}

func getEnv(key string, log *zap.Logger) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	log.Error("Обязательная переменная окружения не установлена", zap.String("key", key))
	panic("missing required environment variable: " + key)
}

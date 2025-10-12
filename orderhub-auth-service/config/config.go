package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/database"

	"go.uber.org/zap"
)

type Config struct {
	Port  string
	JWT   JWT
	DB    DB
	Redis Redis
}

type JWT struct {
	Issuer     string
	Audience   string
	AccessExp  time.Duration
	RefreshExp time.Duration
}

type DB struct {
	database.Config
}

type Redis struct {
	Enabled    bool
	Addr       string
	Password   string
	DB         int
	TTLSeconds int
}

func Load(log *zap.Logger) *Config {
	return &Config{
		Port: getEnv("APP_PORT", log),
		JWT: JWT{
			Issuer:     getEnv("JWT_ISSUER", log),
			Audience:   getEnv("JWT_AUDIENCE", log),
			AccessExp:  parseDurationWithDays(getEnv("ACCESS_EXP", log)),
			RefreshExp: parseDurationWithDays(getEnv("REFRESH_EXP", log)),
		},
		DB: DB{
			Config: database.Config{
				Host:     getEnv("DB_HOST", log),
				Port:     getEnv("DB_PORT", log),
				User:     getEnv("DB_USER", log),
				Password: getEnv("DB_PASSWORD", log),
				Name:     getEnv("DB_NAME", log),
				SSLMode:  getEnv("DB_SSLMODE", log),
			},
		},
		Redis: Redis{
			Enabled:    getEnv("REDIS_ENABLED", log) == "true",
			Addr:       getEnv("REDIS_ADDR", log),
			Password:   getEnv("REDIS_PASSWORD", log),
			DB:         atoiDefault(getEnv("REDIS_DB", log), 0),
			TTLSeconds: atoiDefault(getEnv("CACHE_TTL_SECONDS", log), 60),
		},
	}
}

func getEnv(key string, log *zap.Logger) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	log.Error("Обязательная переменная окружения не установлена", zap.String("key", key))
	panic("missing required environment variable: " + key)
}

func parseDurationWithDays(s string) time.Duration {
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := time.ParseDuration(daysStr + "h")
		if err != nil {
			log.Printf("Ошибка парсинга TTL: %v", err)
			return 0
		}
		return time.Duration(24) * days
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return duration
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

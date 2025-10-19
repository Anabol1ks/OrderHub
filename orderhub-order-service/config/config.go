package config

import (
	"os"

	"github.com/Anabol1ks/orderhub-pkg-proto/pkg/database"

	"go.uber.org/zap"
)

type Config struct {
	Port string
	DB   DB
	// 	Redis Redis

	// 	KafkaBrokers []string
	// 	KafkaTopic   string
}

type DB struct {
	database.Config
}

// type Redis struct {
// 	Enabled    bool
// 	Addr       string
// 	Password   string
// 	DB         int
// 	TTLSeconds int
// }

func Load(log *zap.Logger) *Config {
	return &Config{
		Port: getEnv("APP_PORT", log),
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
		// Redis: Redis{
		// 	Enabled:    getEnv("REDIS_ENABLED", log) == "true",
		// 	Addr:       getEnv("REDIS_ADDR", log),
		// 	Password:   getEnv("REDIS_PASSWORD", log),
		// 	DB:         atoiDefault(getEnv("REDIS_DB", log), 0),
		// 	TTLSeconds: atoiDefault(getEnv("CACHE_TTL_SECONDS", log), 60),
		// },
		// KafkaBrokers: splitAndTrim(os.Getenv("KAFKA_BROKERS")),
		// KafkaTopic:   getEnv("KAFKA_TOPIC_EMAIL", log),
	}
}

func getEnv(key string, log *zap.Logger) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	log.Error("Обязательная переменная окружения не установлена", zap.String("key", key))
	panic("missing required environment variable: " + key)
}

// func atoiDefault(s string, def int) int {
// 	n, err := strconv.Atoi(s)
// 	if err != nil {
// 		return def
// 	}
// 	return n
// }

// func splitAndTrim(s string) []string {
// 	if s == "" {
// 		return nil
// 	}
// 	parts := []string{}
// 	for _, p := range strings.Split(s, ",") {
// 		pt := strings.TrimSpace(p)
// 		if pt != "" {
// 			parts = append(parts, pt)
// 		}
// 	}
// 	return parts
// }

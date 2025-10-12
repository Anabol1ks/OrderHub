package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisClient struct {
	client *redis.Client
	log    *zap.Logger
}

func NewRedisClient(addr, password string, db int, log *zap.Logger) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Info("Redis connected successfully", zap.String("addr", addr))

	return &RedisClient{
		client: rdb,
		log:    log,
	}, nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) SetRateLimit(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *RedisClient) CheckRateLimit(ctx context.Context, key string) (bool, error) {
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// JWK кэширование
func (r *RedisClient) SetJWK(ctx context.Context, kid string, jwkData []byte, ttl time.Duration) error {
	key := fmt.Sprintf("jwk:%s", kid)
	return r.client.Set(ctx, key, jwkData, ttl).Err()
}

func (r *RedisClient) GetJWK(ctx context.Context, kid string) ([]byte, error) {
	key := fmt.Sprintf("jwk:%s", kid)
	return r.client.Get(ctx, key).Bytes()
}

// Blacklist для токенов
func (r *RedisClient) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:%s", jti)
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *RedisClient) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", jti)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// Общий кэш
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

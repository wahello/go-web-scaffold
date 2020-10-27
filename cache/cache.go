package cache

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
)

// RedisConfig config to establish connection to Redis
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Red is a holder for Redis and cache methods
type Red struct {
	R *redis.Client
}

// NewRedisClient create new redis client via config
func NewRedisClient(ctx context.Context, config RedisConfig) (*Red, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	err := client.Ping(ctx).Err()
	if err != nil {
		err = fmt.Errorf("redis PING: %w", err)
		return nil, err
	}

	return &Red{
		R: client,
	}, nil
}

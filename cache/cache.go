package cache

import (
	"context"
	"fmt"
	"time"

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
func NewRedisClient(config RedisConfig) (*Red, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	err := client.Ping(ctx).Err()
	if err != nil {
		err = fmt.Errorf("redis PING: %w", err)
		return nil, err
	}

	return &Red{
		R: client,
	}, nil
}

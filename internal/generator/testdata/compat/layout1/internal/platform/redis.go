package platform

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"example.com/compat/internal/config"
)

// OpenRedis connects to Redis and checks it with a ping.
func OpenRedis(ctx context.Context, cfg config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("platform: ping redis %s: %w", cfg.Addr, err)
	}
	return client, nil
}

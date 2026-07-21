package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, redisURL string, operationTimeout ...time.Duration) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	timeout := time.Second
	if len(operationTimeout) > 0 && operationTimeout[0] > 0 {
		timeout = operationTimeout[0]
	}
	opt.DialTimeout = timeout
	opt.ReadTimeout = timeout
	opt.WriteTimeout = timeout
	opt.PoolTimeout = timeout
	client := redis.NewClient(opt)
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		log.Printf("redis ping failed, continuing without cache guarantees: %v", err)
	}
	return client, nil
}

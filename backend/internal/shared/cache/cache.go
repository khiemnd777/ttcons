package cache

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, redisURL string) *redis.Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("redis url parse failed, using localhost: %v", err)
		opt = &redis.Options{Addr: "localhost:6379"}
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("redis ping failed, continuing without cache guarantees: %v", err)
	}
	return client
}

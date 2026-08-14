package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(addr string, timeout time.Duration, retryAttempts int, retryBackoff time.Duration) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})

	var err error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err = client.Ping(ctx).Err()
		cancel()
		if err == nil {
			return client, nil
		}

		slog.Warn("failed to connect to Redis, retrying", "attempt", attempt, "error", err)
		if attempt < retryAttempts {
			time.Sleep(retryBackoff)
		}
	}

	return nil, err
}

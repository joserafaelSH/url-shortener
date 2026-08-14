//go:build integration

package ratelimit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedisAddr() string {
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: testRedisAddr()})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to connect to test Redis: %v", err)
	}
	return client
}

func cleanupBucketKey(t *testing.T, client *redis.Client, namespace, key string) {
	t.Helper()
	t.Cleanup(func() {
		client.Del(context.Background(), "bucket:"+namespace+":"+key)
	})
}

func TestRedisLimiter_BurstRespectsCapacity(t *testing.T) {
	client := newTestRedisClient(t)
	key := fmt.Sprintf("burst-%d", time.Now().UnixNano())
	limiter := NewRedisLimiter(client, "test:burst", 5, 0)
	cleanupBucketKey(t, client, "test:burst", key)

	allowed := 0
	denied := 0
	for i := 0; i < 8; i++ {
		ok, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Fatalf("call %d returned an unexpected error: %v", i, err)
		}
		if ok {
			allowed++
		} else {
			denied++
		}
	}

	if allowed != 5 {
		t.Errorf("expected exactly 5 allowed requests, got %d", allowed)
	}
	if denied != 3 {
		t.Errorf("expected exactly 3 denied requests, got %d", denied)
	}
}

func TestRedisLimiter_NamespacesAreIndependent(t *testing.T) {
	client := newTestRedisClient(t)
	key := fmt.Sprintf("shared-key-%d", time.Now().UnixNano())

	limiterA := NewRedisLimiter(client, "test:ns-a", 1, 0)
	limiterB := NewRedisLimiter(client, "test:ns-b", 1, 0)
	cleanupBucketKey(t, client, "test:ns-a", key)
	cleanupBucketKey(t, client, "test:ns-b", key)

	okA, err := limiterA.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if !okA {
		t.Fatal("expected the first request on namespace A to be allowed")
	}

	okB, err := limiterB.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if !okB {
		t.Error("expected namespace B to have an independent bucket from namespace A for the same key")
	}
}

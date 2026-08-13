package ratelimit

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed bucket.lua
var tokenBucketScript string


const requestCost = 1.0



type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

type RedisLimiter struct {
	client     *redis.Client
	namespace  string
	maxTokens  float64
	refillRate float64
	script     *redis.Script
}

func NewRedisLimiter(client *redis.Client, namespace string, maxTokens float64, refillRate float64) *RedisLimiter {
	script := redis.NewScript(tokenBucketScript)
	return &RedisLimiter{
		client:     client,
		namespace:  namespace,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		script:     script,
	}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := float64(time.Now().Unix())
	combinedKey := "bucket:" + l.namespace + ":" + key

	result, err := l.script.Run(ctx, l.client, []string{combinedKey}, l.maxTokens, l.refillRate, requestCost, now).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

type bucket struct {
	currentTokens float64
	lastTimestamp time.Time
}

type InMemoryLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*bucket
	maxTokens    float64
	refillRate   float64 // tokens por segundo

}

func NewInMemoryLimiter(maxTokens float64, refillRate float64) *InMemoryLimiter {
	return &InMemoryLimiter{
		buckets: make(map[string]*bucket),
		maxTokens: maxTokens,
		refillRate: refillRate,
	}
}

func (l *InMemoryLimiter) Allow(ctx context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.buckets[key]
	if !exists {
		
		newBucket := &bucket{
			currentTokens: l.maxTokens - requestCost,
			lastTimestamp: time.Now(),
		}
		l.buckets[key] = newBucket
		return true, nil
	}
	now := time.Now()
	elapsed := now.Sub(b.lastTimestamp)          
	tokensToAdd := elapsed.Seconds() * l.refillRate

	b.currentTokens = min(b.currentTokens+tokensToAdd, l.maxTokens)
	b.lastTimestamp = now

	if b.currentTokens >= requestCost {
		b.currentTokens -= requestCost
		return true, nil
	}

	return false, nil
}
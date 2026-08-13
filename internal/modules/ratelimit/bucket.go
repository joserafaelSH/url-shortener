package ratelimit

import (
	"sync"
	"time"
)
const requestCost = 1.0
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

func (l *InMemoryLimiter) Allow(key string) (bool, error) {
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
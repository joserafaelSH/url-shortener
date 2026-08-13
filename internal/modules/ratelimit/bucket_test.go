package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

// --- Basic behavior ---

func TestAllow_FirstRequestIsAllowedAndCharged(t *testing.T) {
	limiter := NewInMemoryLimiter(10, 1.0)

	ok, err := limiter.Allow(context.Background(), "user1")
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if !ok {
		t.Fatal("expected the first request to be allowed")
	}

	b := limiter.buckets["user1"]
	if b == nil {
		t.Fatal("expected a bucket to have been created for the key")
	}
	if b.currentTokens != 9 {
		t.Errorf("expected currentTokens=9 after the first request (maxTokens - cost), got %v", b.currentTokens)
	}
}

func TestAllow_BurstRespectsBucketCapacity(t *testing.T) {
	limiter := NewInMemoryLimiter(20, 10.0/60.0) // slow refill, irrelevant during a fast burst

	allowed := 0
	denied := 0
	for i := 0; i < 25; i++ {
		ok, err := limiter.Allow(context.Background(), "user1")
		if err != nil {
			t.Fatalf("call %d returned an unexpected error: %v", i, err)
		}
		if ok {
			allowed++
		} else {
			denied++
		}
	}

	if allowed != 20 {
		t.Errorf("expected exactly 20 allowed requests, got %d", allowed)
	}
	if denied != 5 {
		t.Errorf("expected exactly 5 denied requests, got %d", denied)
	}
}

func TestAllow_DeniesWhenBucketIsEmpty(t *testing.T) {
	limiter := NewInMemoryLimiter(1, 0) // capacity 1, no refill at all

	ok1, _ := limiter.Allow(context.Background(), "user1")
	if !ok1 {
		t.Fatal("expected the first request to be allowed")
	}

	ok2, _ := limiter.Allow(context.Background(), "user1")
	if ok2 {
		t.Error("expected the second request to be denied (bucket should be empty and refillRate=0)")
	}
}

// --- Independence between keys ---

func TestAllow_DifferentKeysHaveIndependentBuckets(t *testing.T) {
	limiter := NewInMemoryLimiter(1, 0)

	okA, _ := limiter.Allow(context.Background(), "userA")
	okB, _ := limiter.Allow(context.Background(), "userB")

	if !okA || !okB {
		t.Error("expected both keys to be allowed independently on their first request")
	}

	// both buckets should now be exhausted, independently
	okA2, _ := limiter.Allow(context.Background(), "userA")
	okB2, _ := limiter.Allow(context.Background(), "userB")

	if okA2 || okB2 {
		t.Error("expected both keys to be denied on their second request, independently of each other")
	}
}

// --- Refill over time ---

func TestAllow_RefillGrantsTokensBackOverTime(t *testing.T) {
	// small numbers chosen so the test doesn't need to sleep for long:
	// capacity 2, refill rate 10 tokens/second (1 token every 100ms)
	limiter := NewInMemoryLimiter(2, 10.0)

	// drain the bucket completely
	limiter.Allow(context.Background(), "user1")
	limiter.Allow(context.Background(), "user1")

	ok, _ := limiter.Allow(context.Background(), "user1")
	if ok {
		t.Fatal("expected the bucket to be empty right after draining it")
	}

	// wait long enough for at least one token to be refilled
	time.Sleep(150 * time.Millisecond)

	ok2, err := limiter.Allow(context.Background(), "user1")
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if !ok2 {
		t.Error("expected a request to be allowed after waiting for a refill")
	}
}

func TestAllow_TokensNeverExceedMaxTokens(t *testing.T) {
	limiter := NewInMemoryLimiter(5, 100.0) // fast refill on purpose

	limiter.Allow(context.Background(), "user1") // creates the bucket, currentTokens = maxTokens - 1 = 4

	// wait long enough that, without a cap, refill would add way more than maxTokens
	time.Sleep(200 * time.Millisecond)

	limiter.mu.Lock()
	b := limiter.buckets["user1"]
	limiter.mu.Unlock()

	// force a refill calculation by making another call, then inspect the internal state
	limiter.Allow(context.Background(), "user1")

	limiter.mu.Lock()
	tokensAfter := b.currentTokens
	limiter.mu.Unlock()

	if tokensAfter > 5 {
		t.Errorf("expected currentTokens to be capped at maxTokens=5, got %v", tokensAfter)
	}
}

// --- Concurrency safety ---

func TestAllow_ConcurrentRequestsNeverExceedCapacity(t *testing.T) {
	const capacity = 50
	limiter := NewInMemoryLimiter(capacity, 0) // no refill, to make the expected count deterministic

	const numGoroutines = 20
	const requestsPerGoroutine = 10 // total attempts = 200, way more than capacity

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedCount := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				ok, err := limiter.Allow(context.Background(), "shared-key")
				if err != nil {
					t.Error(err)
					return
				}
				if ok {
					mu.Lock()
					allowedCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	if allowedCount != capacity {
		t.Errorf("expected exactly %d allowed requests under concurrency, got %d", capacity, allowedCount)
	}
}
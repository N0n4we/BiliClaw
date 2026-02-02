package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_Acquire(t *testing.T) {
	tb := NewTokenBucket(10.0, 5.0) // 10 tokens/sec, capacity 5

	// Should be able to acquire 5 tokens immediately (full capacity)
	for i := 0; i < 5; i++ {
		if !tb.Acquire(1.0, false) {
			t.Errorf("Expected to acquire token %d, but failed", i+1)
		}
	}

	// Should fail to acquire 6th token without blocking
	if tb.Acquire(1.0, false) {
		t.Error("Expected to fail acquiring 6th token, but succeeded")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(10.0, 5.0) // 10 tokens/sec, capacity 5

	// Drain all tokens
	for i := 0; i < 5; i++ {
		tb.Acquire(1.0, false)
	}

	// Wait for refill (100ms should give us ~1 token)
	time.Sleep(100 * time.Millisecond)

	// Should be able to acquire at least 1 token now
	if !tb.Acquire(1.0, false) {
		t.Error("Expected to acquire token after refill, but failed")
	}
}

func TestTokenBucket_Blocking(t *testing.T) {
	tb := NewTokenBucket(100.0, 1.0) // 100 tokens/sec, capacity 1

	// Drain the bucket
	tb.Acquire(1.0, false)

	// Blocking acquire should succeed after waiting
	start := time.Now()
	if !tb.Acquire(1.0, true) {
		t.Error("Expected blocking acquire to succeed")
	}
	elapsed := time.Since(start)

	// Should have waited approximately 10ms (1 token / 100 tokens per sec)
	if elapsed < 5*time.Millisecond {
		t.Errorf("Expected to wait at least 5ms, but only waited %v", elapsed)
	}
}

func TestTokenBucket_SetRate(t *testing.T) {
	tb := NewTokenBucket(1.0, 5.0)

	// Drain all tokens
	for i := 0; i < 5; i++ {
		tb.Acquire(1.0, false)
	}

	// Set a higher rate
	tb.SetRate(100.0)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Should have more tokens now due to higher rate
	tokens := tb.GetTokens()
	if tokens < 4.0 {
		t.Errorf("Expected at least 4 tokens after rate increase, got %f", tokens)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	tb := NewTokenBucket(1000.0, 100.0) // High rate for fast test

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Launch 50 goroutines trying to acquire tokens
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Acquire(1.0, false) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should have acquired at most 100 tokens (capacity)
	if successCount > 100 {
		t.Errorf("Acquired more tokens than capacity: %d", successCount)
	}
}

func TestGetRateLimiter_Singleton(t *testing.T) {
	limiter1 := GetRateLimiter()
	limiter2 := GetRateLimiter()

	if limiter1 != limiter2 {
		t.Error("GetRateLimiter should return the same instance")
	}
}

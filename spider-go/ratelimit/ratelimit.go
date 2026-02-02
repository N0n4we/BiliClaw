package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	rate     float64
	capacity float64
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewTokenBucket creates a new token bucket with the given rate and capacity
func NewTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity,
		lastTime: time.Now(),
	}
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens = min(tb.capacity, tb.tokens+elapsed*tb.rate)
	tb.lastTime = now
}

// Acquire attempts to acquire the specified number of tokens
// If blocking is true, it will wait until tokens are available
func (tb *TokenBucket) Acquire(tokens float64, blocking bool) bool {
	for {
		tb.mu.Lock()
		tb.refill()
		if tb.tokens >= tokens {
			tb.tokens -= tokens
			tb.mu.Unlock()
			return true
		}
		if !blocking {
			tb.mu.Unlock()
			return false
		}
		waitTime := (tokens - tb.tokens) / tb.rate
		tb.mu.Unlock()
		time.Sleep(time.Duration(waitTime * float64(time.Second)))
	}
}

// SetRate updates the rate of token generation
func (tb *TokenBucket) SetRate(rate float64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	tb.rate = rate
}

// GetTokens returns the current number of available tokens (for testing)
func (tb *TokenBucket) GetTokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

var (
	globalLimiter *TokenBucket
	limiterMu     sync.Mutex
)

// InitRateLimiter initializes the global rate limiter with custom rate and capacity
func InitRateLimiter(rate, capacity float64) {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	globalLimiter = NewTokenBucket(rate, capacity)
}

// GetRateLimiter returns the global rate limiter singleton
func GetRateLimiter() *TokenBucket {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	if globalLimiter == nil {
		globalLimiter = NewTokenBucket(2.0, 5.0)
	}
	return globalLimiter
}

// WaitForToken acquires one token from the global rate limiter
func WaitForToken() {
	GetRateLimiter().Acquire(1.0, true)
}

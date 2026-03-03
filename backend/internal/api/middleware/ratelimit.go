package middleware

import (
	"sync"
	"time"
)

// RateLimiter is a simple in-memory rate limiter using sliding windows.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter creates a new in-memory rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
	}
}

// Allow returns true if the request is within the rate limit.
// key is a unique identifier (e.g. "oauth-callback:<ip>").
// limit is the max requests per window.
// window is the time window duration.
func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, ok := rl.entries[key]

	if !ok || now.After(e.resetAt) {
		rl.entries[key] = &rateLimitEntry{
			count:   1,
			resetAt: now.Add(window),
		}
		return true
	}

	if e.count >= limit {
		return false
	}

	e.count++
	return true
}

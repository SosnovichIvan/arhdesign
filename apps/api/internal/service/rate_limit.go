package service

import (
	"sync"
	"time"
)

type RateLimiter struct {
	hits   map[string]time.Time
	mutex  sync.Mutex
	now    func() time.Time
	window time.Duration
}

func NewRateLimiter(now func() time.Time, window time.Duration) *RateLimiter {
	return &RateLimiter{hits: map[string]time.Time{}, now: now, window: window}
}

func (limiter *RateLimiter) RetryAfter(key string) time.Duration {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	currentTime := limiter.now()
	if until := limiter.hits[key]; until.After(currentTime) {
		return until.Sub(currentTime)
	}
	limiter.hits[key] = currentTime.Add(limiter.window)
	return 0
}

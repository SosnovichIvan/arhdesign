package service

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Now()
	limiter := NewRateLimiter(func() time.Time { return now }, time.Minute)

	if got := limiter.RetryAfter("ip"); got != 0 {
		t.Fatal(got)
	}
	if got := limiter.RetryAfter("ip"); got != time.Minute {
		t.Fatal(got)
	}
	now = now.Add(time.Minute)
	if got := limiter.RetryAfter("ip"); got != 0 {
		t.Fatal(got)
	}
}

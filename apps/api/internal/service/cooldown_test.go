package service

import (
	"testing"
	"time"
)

func TestCooldown(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewCooldown(func() time.Time { return now })
	c.Set("browser")
	if c.RetryAfter("browser") != time.Hour {
		t.Fatal("expected one hour")
	}
	now = now.Add(time.Hour)
	if c.RetryAfter("browser") != 0 {
		t.Fatal("expected expiry")
	}
}

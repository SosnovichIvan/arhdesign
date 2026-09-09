package service

import (
	"sync"
	"time"
)

type Cooldown struct {
	until map[string]time.Time
	now   func() time.Time
	mutex sync.Mutex
}

func NewCooldown(now func() time.Time) *Cooldown {
	return &Cooldown{until: map[string]time.Time{}, now: now}
}
func (c *Cooldown) RetryAfter(key string) time.Duration {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if until := c.until[key]; until.After(c.now()) {
		return until.Sub(c.now())
	}
	return 0
}
func (c *Cooldown) Set(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.until[key] = c.now().Add(time.Hour)
}

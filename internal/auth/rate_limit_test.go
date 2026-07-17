package auth

import (
	"testing"
	"time"
)

func TestMemoryRateLimiterEnforcesWindowAndResets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	limiter := NewMemoryRateLimiter(RateLimiterConfig{
		Limit:      2,
		Window:     time.Minute,
		MaxEntries: 10,
	})

	if !limiter.Allow("login:person", now) {
		t.Fatal("first attempt was rejected")
	}
	if !limiter.Allow("login:person", now.Add(time.Second)) {
		t.Fatal("second attempt was rejected")
	}
	if limiter.Allow("login:person", now.Add(2*time.Second)) {
		t.Fatal("attempt beyond limit was accepted")
	}
	if !limiter.Allow("login:person", now.Add(time.Minute)) {
		t.Fatal("attempt after window reset was rejected")
	}
}

func TestMemoryRateLimiterBoundsAttackerControlledKeys(t *testing.T) {
	t.Parallel()

	now := time.Now()
	limiter := NewMemoryRateLimiter(RateLimiterConfig{
		Limit:      1,
		Window:     time.Hour,
		MaxEntries: 1,
	})
	if !limiter.Allow("first", now) {
		t.Fatal("first key was rejected")
	}
	if limiter.Allow("second", now) {
		t.Fatal("new key was accepted after bounded store filled")
	}
}

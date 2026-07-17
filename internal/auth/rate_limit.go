package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type AttemptLimiter interface {
	Allow(key string, now time.Time) bool
}

type RateLimiterConfig struct {
	Limit      int
	Window     time.Duration
	MaxEntries int
}

type rateEntry struct {
	count       int
	windowStart time.Time
}

type MemoryRateLimiter struct {
	mu         sync.Mutex
	limit      int
	window     time.Duration
	maxEntries int
	entries    map[string]rateEntry
}

func NewMemoryRateLimiter(config RateLimiterConfig) *MemoryRateLimiter {
	if config.Limit <= 0 {
		config.Limit = 10
	}
	if config.Window <= 0 {
		config.Window = 15 * time.Minute
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 10_000
	}
	return &MemoryRateLimiter{
		limit:      config.Limit,
		window:     config.Window,
		maxEntries: config.MaxEntries,
		entries:    make(map[string]rateEntry),
	}
}

func (l *MemoryRateLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, ok := l.entries[key]; ok {
		if now.Sub(entry.windowStart) < l.window {
			entry.count++
			l.entries[key] = entry
			return entry.count <= l.limit
		}
		delete(l.entries, key)
	}

	if len(l.entries) >= l.maxEntries {
		l.removeExpired(now)
		if len(l.entries) >= l.maxEntries {
			// Fail closed when the bounded store is saturated. This avoids turning
			// attacker-controlled keys into unbounded memory growth.
			return false
		}
	}
	l.entries[key] = rateEntry{count: 1, windowStart: now}
	return true
}

func (l *MemoryRateLimiter) removeExpired(now time.Time) {
	for key, entry := range l.entries {
		if now.Sub(entry.windowStart) >= l.window {
			delete(l.entries, key)
		}
	}
}

func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

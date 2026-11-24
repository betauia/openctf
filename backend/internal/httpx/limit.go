package httpx

import (
	"sync"
	"time"
)

// Limiter is a per-key leaky bucket. Each key may burst up to max calls, after which calls
// are allowed again at a steady rate of max per window. Keys are independent, so one key
// running out never affects another, and nothing resets all keys at the same moment.
// Counts live in memory.
type Limiter struct {
	mu        sync.Mutex
	max       float64
	window    time.Duration
	now       func() time.Time // replaced in tests
	lastSweep time.Time
	buckets   map[string]bucket
}

type bucket struct {
	level float64 // calls recorded, drains at max/window per second
	at    time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: float64(max), window: window, now: time.Now, buckets: map[string]bucket{}}
}

// drained returns b's level after draining it up to now.
func (l *Limiter) drained(b bucket, now time.Time) float64 {
	return max(b.level-now.Sub(b.at).Seconds()*l.max/l.window.Seconds(), 0)
}

// Allow records a call for key and reports whether it is within the limit. A rejected call
// is not recorded, so a caller that keeps retrying is let back in as soon as the bucket drains.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	level := l.drained(l.buckets[key], now)
	if level+1 > l.max {
		l.buckets[key] = bucket{level, now}
		return false
	}
	l.buckets[key] = bucket{level + 1, now}
	return true
}

// sweep drops keys that have fully drained so the map does not grow without bound.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for k, b := range l.buckets {
		if l.drained(b, now) == 0 {
			delete(l.buckets, k)
		}
	}
}

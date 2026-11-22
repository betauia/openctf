package httpx

import (
	"sync"
	"time"
)

// Limiter allows at most max calls per key in each fixed window. Counts live in memory and
// reset for every key when the window ends.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	start  time.Time
	counts map[string]int
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, counts: map[string]int{}}
}

// Allow records a call for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now := time.Now(); now.Sub(l.start) >= l.window {
		l.start = now
		clear(l.counts)
	}
	l.counts[key]++
	return l.counts[key] <= l.max
}

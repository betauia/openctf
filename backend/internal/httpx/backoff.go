package httpx

import (
	"sync"
	"time"
)

// Backoff slows down repeated failures per key with exponential delays. The first free
// attempts go through immediately (typos); every attempt after that must wait twice as long
// as the one before, up to maxDelay. Keys are independent, and a key that has been quiet for
// forget starts over.
//
// Attempts are charged when they begin, not when they fail, so a burst of parallel requests
// cannot all get through while the first is still being checked. Call Reset on success.
type Backoff struct {
	mu       sync.Mutex
	free     int
	base     time.Duration
	maxDelay time.Duration
	forget   time.Duration
	now      func() time.Time // replaced in tests
	swept    time.Time
	entries  map[string]*backoffEntry
}

type backoffEntry struct {
	fails int
	next  time.Time // no attempt is allowed before this
	last  time.Time
}

// NewBackoff allows free attempts, then makes each one wait base, 2*base, 4*base ... capped
// at maxDelay. Keys idle for forget are forgotten, so forget should exceed maxDelay.
func NewBackoff(free int, base, maxDelay, forget time.Duration) *Backoff {
	return &Backoff{free: free, base: base, maxDelay: maxDelay, forget: forget, now: time.Now, entries: map[string]*backoffEntry{}}
}

// delay is how long to wait after the fails-th attempt.
func (b *Backoff) delay(fails int) time.Duration {
	n := fails - b.free
	if n <= 0 {
		return 0
	}
	d := b.base
	for ; n > 1 && d < b.maxDelay; n-- {
		d *= 2
	}
	return min(d, b.maxDelay)
}

// Begin charges an attempt for key. If key is still waiting it charges nothing and returns
// how long until the next attempt is allowed.
func (b *Backoff) Begin(key string) (wait time.Duration, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	b.sweep(now)
	e := b.entries[key]
	if e == nil || now.Sub(e.last) >= b.forget {
		e = &backoffEntry{}
		b.entries[key] = e
	}
	if now.Before(e.next) {
		return e.next.Sub(now), false
	}
	e.fails++
	e.last = now
	e.next = now.Add(b.delay(e.fails))
	return 0, true
}

// Refund takes back an attempt charged by Begin, for when a later check rejected the request
// before it was really tried.
func (b *Backoff) Refund(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.entries[key]
	if e == nil {
		return
	}
	if e.fails--; e.fails <= 0 {
		delete(b.entries, key)
		return
	}
	e.next = e.last.Add(b.delay(e.fails))
}

// Reset forgets key, for a successful login.
func (b *Backoff) Reset(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, key)
}

// sweep drops forgotten keys so the map does not grow without bound.
func (b *Backoff) sweep(now time.Time) {
	if now.Sub(b.swept) < b.forget {
		return
	}
	b.swept = now
	for k, e := range b.entries {
		if now.Sub(e.last) >= b.forget {
			delete(b.entries, k)
		}
	}
}

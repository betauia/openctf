package httpx

import (
	"testing"
	"time"
)

// newTestLimiter returns a limiter of max calls per window with a clock the test moves.
func newTestLimiter(max int, window time.Duration) (*Limiter, *time.Time) {
	now := time.Unix(1_000_000, 0)
	l := NewLimiter(max, window)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestLimiterAllowsABurstThenRefuses(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 1; i <= 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("call %d was refused, want a burst of 3", i)
		}
	}
	if l.Allow("a") {
		t.Error("the fourth call was allowed")
	}
}

func TestLimiterRefillsAtMaxPerWindow(t *testing.T) {
	l, now := newTestLimiter(3, time.Minute) // one call back every 20 seconds
	for range 3 {
		l.Allow("a")
	}

	*now = now.Add(19 * time.Second)
	if l.Allow("a") {
		t.Error("allowed after 19s, before the first slot drained")
	}
	*now = now.Add(time.Second) // 20s since the burst
	if !l.Allow("a") {
		t.Error("refused after 20s, when one slot should be free")
	}
	if l.Allow("a") {
		t.Error("allowed a second call when only one slot had drained")
	}
}

func TestLimiterIsFullyRestoredAfterAWindow(t *testing.T) {
	l, now := newTestLimiter(3, time.Minute)
	for range 3 {
		l.Allow("a")
	}

	*now = now.Add(time.Minute)

	for i := 1; i <= 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("call %d refused after a full window", i)
		}
	}
	if l.Allow("a") {
		t.Error("allowed more than max after a full window")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)

	if !l.Allow("a") || l.Allow("a") {
		t.Fatal("a should have exactly one call")
	}
	if !l.Allow("b") {
		t.Error("b was refused because a ran out")
	}
}

// A refused call is not recorded, so retrying doesn't push the moment a slot frees back.
// The window is 64s so that every step below drains an exact binary fraction of a slot.
func TestLimiterRefusedCallsDoNotCount(t *testing.T) {
	l, now := newTestLimiter(1, 64*time.Second)
	l.Allow("a")

	for range 3 {
		*now = now.Add(16 * time.Second)
		if l.Allow("a") {
			t.Fatal("allowed before the slot drained")
		}
	}

	*now = now.Add(16 * time.Second) // 64s since the only recorded call
	if !l.Allow("a") {
		t.Error("refused after a full window although the retries were refused")
	}
}

func TestLimiterForgetsDrainedKeys(t *testing.T) {
	l, now := newTestLimiter(2, time.Minute)
	l.Allow("a")
	l.Allow("b")

	*now = now.Add(2 * time.Minute) // both drained, and a sweep is due
	l.Allow("c")

	if len(l.buckets) != 1 {
		t.Errorf("%d keys remembered, want only c", len(l.buckets))
	}
}

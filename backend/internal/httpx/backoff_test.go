package httpx

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	now := time.Unix(1000, 0)
	b := NewBackoff(2, time.Second, 8*time.Second, time.Minute)
	b.now = func() time.Time { return now }

	for i := range 3 { // 2 free attempts, then a 3rd that starts a 1s wait
		if _, ok := b.Begin("a"); !ok {
			t.Fatalf("attempt %d rejected", i)
		}
	}
	if wait, ok := b.Begin("a"); ok || wait != time.Second {
		t.Fatalf("want 1s wait, got %v ok=%v", wait, ok)
	}
	if _, ok := b.Begin("b"); !ok {
		t.Fatal("b must not be affected by a")
	}
	now = now.Add(time.Second)
	if _, ok := b.Begin("a"); !ok {
		t.Fatal("a should be allowed after 1s")
	}
	if wait, ok := b.Begin("a"); ok || wait != 2*time.Second { // doubled
		t.Fatalf("want 2s wait, got %v ok=%v", wait, ok)
	}
	b.Refund("a") // undo the 4th attempt: only the 1s wait from the 3rd remains
	now = now.Add(time.Second)
	if _, ok := b.Begin("a"); !ok {
		t.Fatal("a should be allowed after a refund")
	}
	b.Reset("a")
	for i := range 3 {
		if _, ok := b.Begin("a"); !ok {
			t.Fatalf("reset should clear the backoff, attempt %d rejected", i)
		}
	}
	for range 20 { // the delay is capped
		now = now.Add(b.maxDelay)
		b.Begin("a")
	}
	if wait, _ := b.Begin("a"); wait > b.maxDelay {
		t.Fatalf("wait %v exceeds cap", wait)
	}
	now = now.Add(2 * time.Minute) // idle keys are forgotten and swept
	b.Begin("c")
	if len(b.entries) != 1 {
		t.Fatalf("idle keys should be swept, have %d", len(b.entries))
	}
}

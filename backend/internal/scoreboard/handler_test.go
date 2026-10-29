package scoreboard

import "testing"

func TestNotifyCoalesces(t *testing.T) {
	h := New(nil)
	h.Notify()
	h.Notify() // must not block while an update is pending
	select {
	case <-h.notify:
	default:
		t.Fatal("no pending notification")
	}
	select {
	case <-h.notify:
		t.Error("notifications were not coalesced")
	default:
	}
}

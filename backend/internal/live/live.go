package live

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

type Hub struct {
	a    *auth.Auth
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func New(a *auth.Auth) *Hub {
	return &Hub{a: a, subs: map[chan []byte]struct{}{}}
}

func (h *Hub) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/live", h.a.Authorise(policy.ChallengeViewSolves, h.stream))
}

func (h *Hub) Publish(event string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, payload))
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		select {
		case c <- msg:
		default:
		}
	}
}

func (h *Hub) stream(w http.ResponseWriter, r *http.Request, _ *auth.User) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	if err := rc.Flush(); err != nil {
		httpx.InternalError(w, r, err)
		return
	}

	c := make(chan []byte, 16)
	h.mu.Lock()
	h.subs[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.subs, c)
		h.mu.Unlock()
	}()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-c:
			if _, err := w.Write(msg); err != nil {
				return
			}
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}

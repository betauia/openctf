// Package scoreboard pushes the live team ranking to websocket clients.
package scoreboard

import (
	"context"
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/teams"
)

// Handler tracks connected websocket clients and broadcasts the ranking to them.
type Handler struct {
	store *database.Store

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	notify  chan struct{}
}

// New returns a Handler that reads the ranking from store. Call Run to start pushing updates.
func New(store *database.Store) *Handler {
	return &Handler{
		store:   store,
		clients: make(map[*websocket.Conn]struct{}),
		notify:  make(chan struct{}, 1),
	}
}

// Register adds the /api/scoreboard routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/scoreboard/ws", h.live)
}

const (
	scoreboardInterval = 10 * time.Second
	writeTimeout       = 5 * time.Second
)

// Notify asks Run to push a fresh scoreboard to every connected client.
func (h *Handler) Notify() {
	select {
	case h.notify <- struct{}{}:
	default: // an update is already pending
	}
}

// Run broadcasts the scoreboard on Notify and every scoreboardInterval until ctx is done.
func (h *Handler) Run(ctx context.Context) {
	tick := time.NewTicker(scoreboardInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				c.Close(websocket.StatusGoingAway, "server shutting down")
			}
			h.mu.Unlock()
			return
		case <-tick.C:
		case <-h.notify:
		}
		h.broadcast(ctx)
	}
}

func (h *Handler) broadcast(ctx context.Context) {
	h.mu.Lock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()
	if len(clients) == 0 {
		return
	}

	msg, err := h.scoreboardMessage(ctx)
	if err != nil {
		slog.Error("build scoreboard", "err", err)
		return
	}
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Go(func() { send(ctx, c, msg) })
	}
	wg.Wait()
}

func (h *Handler) scoreboardMessage(ctx context.Context) ([]byte, error) {
	ranking, err := teams.Ranking(ctx, h.store)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ranking)
}

// send writes msg to c, dropping the connection if the client can't keep up.
func send(ctx context.Context, c *websocket.Conn, msg []byte) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, msg); err != nil {
		c.CloseNow()
	}
}

// live streams the ranking over a websocket: once on connect, then on every broadcast.
func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	// The scoreboard is public, read-only data, so cross-origin clients are allowed.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return // Accept has already written the error response
	}
	// Clients never send anything; ctx ends when the connection closes.
	ctx := c.CloseRead(r.Context())

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		c.CloseNow()
	}()

	msg, err := h.scoreboardMessage(ctx)
	if err != nil {
		slog.Error("build scoreboard", "err", err)
		c.Close(websocket.StatusInternalError, "")
		return
	}
	send(ctx, c, msg)
	<-ctx.Done()
}

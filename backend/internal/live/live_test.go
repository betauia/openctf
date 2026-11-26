package live

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
	"github.com/betauia/openctf/backend/internal/testutil/testenv"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

const wait = 5 * time.Second

// rig is a hub behind a real HTTP server, so a client can hold a stream open.
type rig struct {
	hub *Hub
	env *testenv.Env
	srv *httptest.Server
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{}
	r.env = testenv.New(t, func(mux *http.ServeMux, a *auth.Auth) {
		r.hub = New(a)
		r.hub.Register(mux)
	})
	r.srv = httptest.NewServer(r.env.Mux)
	t.Cleanup(r.srv.Close) // runs after every client below has been cancelled
	return r
}

// client is one open stream.
type client struct {
	resp   *http.Response
	reader *bufio.Reader
}

// open requests the stream as u. The response is returned even when it isn't a 200.
func (r *rig) open(t *testing.T, u *testdb.User) *client {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", r.srv.URL+"/api/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		if u.Cookie == nil {
			r.env.Login(t, u)
		}
		req.AddCookie(u.Cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		resp.Body.Close()
	})
	return &client{resp: resp, reader: bufio.NewReader(resp.Body)}
}

// connect opens the stream, checks it is a 200 and waits for the hub to register it.
func (r *rig) connect(t *testing.T, u *testdb.User) *client {
	t.Helper()
	before := r.subscribers()
	c := r.open(t, u)
	if c.resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", c.resp.StatusCode)
	}
	if first := c.next(t); first != ": connected" {
		t.Fatalf("first block = %q, want the connected comment", first)
	}
	// The comment is sent before the client is subscribed, so wait for the hub to know it.
	eventually(t, func() bool { return r.subscribers() == before+1 }, "the hub to register the client")
	return c
}

// next returns the next block of lines, up to the blank line that ends it.
func (c *client) next(t *testing.T) string {
	t.Helper()
	block, err := c.tryNext(wait)
	if err != nil {
		t.Fatalf("reading the stream: %v (got %q)", err, block)
	}
	return block
}

// tryNext is next without failing the test, so a goroutine can use it.
func (c *client) tryNext(timeout time.Duration) (string, error) {
	type result struct {
		block string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		var lines []string
		for {
			line, err := c.reader.ReadString('\n')
			if err != nil {
				done <- result{strings.Join(lines, "\n"), err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				done <- result{strings.Join(lines, "\n"), nil}
				return
			}
			lines = append(lines, line)
		}
	}()
	select {
	case r := <-done:
		return r.block, r.err
	case <-time.After(timeout):
		return "", errors.New("timed out waiting for the next block")
	}
}

func (r *rig) subscribers() int {
	r.hub.mu.Lock()
	defer r.hub.mu.Unlock()
	return len(r.hub.subs)
}

func eventually(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// --- the stream ---

func TestStreamHeadersAndConnectedComment(t *testing.T) {
	r := newRig(t)

	c := r.connect(t, testdb.NewUser(t))

	if got := c.resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := c.resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestAClientIsRemovedWhenItDisconnects(t *testing.T) {
	r := newRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	u := testdb.NewUser(t)
	r.env.Login(t, u)
	req, _ := http.NewRequestWithContext(ctx, "GET", r.srv.URL+"/api/live", nil)
	req.AddCookie(u.Cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	eventually(t, func() bool { return r.subscribers() == 1 }, "the client to be registered")

	cancel()

	eventually(t, func() bool { return r.subscribers() == 0 }, "the hub to drop the client")
}

// --- publishing ---

// solved is event data with a fixed field order, which a map does not have.
type solved struct {
	Challenge string `json:"challenge"`
	Points    int    `json:"points"`
}

func TestPublishReachesEveryClientInOrder(t *testing.T) {
	r := newRig(t)
	a, b := r.connect(t, testdb.NewUser(t)), r.connect(t, testdb.NewAdmin(t))

	r.hub.Publish("solve", solved{"pwn", 100})
	r.hub.Publish("solve", solved{"web", 200})

	for name, c := range map[string]*client{"first client": a, "second client": b} {
		if got, want := c.next(t), "event: solve\ndata: {\"challenge\":\"pwn\",\"points\":100}"; got != want {
			t.Errorf("%s: first event = %q, want %q", name, got, want)
		}
		if got, want := c.next(t), "event: solve\ndata: {\"challenge\":\"web\",\"points\":200}"; got != want {
			t.Errorf("%s: second event = %q, want %q", name, got, want)
		}
	}
}

func TestPublishUsesTheEventName(t *testing.T) {
	r := newRig(t)
	c := r.connect(t, testdb.NewUser(t))

	r.hub.Publish("announcement", "hello")

	if got, want := c.next(t), "event: announcement\ndata: \"hello\""; got != want {
		t.Errorf("event = %q, want %q", got, want)
	}
}

func TestPublishWithoutClientsIsHarmless(t *testing.T) {
	r := newRig(t)

	r.hub.Publish("solve", map[string]any{"a": 1})

	if r.subscribers() != 0 {
		t.Error("publishing created a subscriber")
	}
}

func TestPublishDropsDataThatCannotBeEncoded(t *testing.T) {
	r := newRig(t)
	c := r.connect(t, testdb.NewUser(t))

	r.hub.Publish("bad", map[string]any{"channel": make(chan int)})
	r.hub.Publish("good", "ok")

	if got, want := c.next(t), "event: good\ndata: \"ok\""; got != want {
		t.Errorf("first event = %q, want the bad one skipped and %q", got, want)
	}
}

// A subscriber that falls behind has a full buffer, and Publish must skip it, not wait for it.
func TestPublishSkipsAClientWhoseBufferIsFull(t *testing.T) {
	h := New(nil)
	slow := make(chan []byte, 16)
	fast := make(chan []byte, 64)
	h.subs[slow], h.subs[fast] = struct{}{}, struct{}{}

	done := make(chan struct{})
	go func() {
		for i := range 40 {
			h.Publish("tick", i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(wait):
		t.Fatal("Publish blocked on a client that wasn't reading")
	}

	if len(slow) != 16 {
		t.Errorf("slow client buffered %d events, want its full buffer of 16", len(slow))
	}
	if len(fast) != 40 {
		t.Errorf("fast client got %d events, want all 40", len(fast))
	}
	if first := string(<-slow); first != "event: tick\ndata: 0\n\n" {
		t.Errorf("slow client's first event = %q, want the oldest one kept", first)
	}
}

// A client that connects and never reads must not hold up Publish or the other clients.
func TestAStalledClientDoesNotBlockOthers(t *testing.T) {
	r := newRig(t)
	r.connect(t, testdb.NewUser(t)) // connects, then never reads again
	reader := r.connect(t, testdb.NewUser(t))
	const events = 40
	payload := strings.Repeat("x", 128<<10)

	received := make(chan int, 1)
	go func() {
		n := 0
		for n < events {
			block, err := reader.tryNext(2 * wait)
			if err != nil {
				break
			}
			if strings.HasPrefix(block, "event: big") {
				n++
			}
		}
		received <- n
	}()
	published := make(chan struct{})
	go func() {
		for range events {
			r.hub.Publish("big", payload)
			time.Sleep(time.Millisecond)
		}
		close(published)
	}()

	select {
	case <-published:
	case <-time.After(2 * wait):
		t.Fatal("Publish was held up by the client that wasn't reading")
	}
	select {
	case n := <-received:
		if n != events {
			t.Errorf("the other client got %d events, want %d", n, events)
		}
	case <-time.After(2 * wait):
		t.Fatal("the other client did not receive its events")
	}
}

// --- access ---

func TestEveryRoleMayConnect(t *testing.T) {
	r := newRig(t)

	for name, u := range map[string]*testdb.User{
		"a player":     testdb.NewUser(t),
		"a maintainer": testdb.NewMaintainer(t),
		"an admin":     testdb.NewAdmin(t),
	} {
		t.Run(name, func(t *testing.T) {
			r.connect(t, u)
		})
		// The previous client leaves asynchronously; connect counts subscribers, so let it go first.
		eventually(t, func() bool { return r.subscribers() == 0 }, "the hub to drop the previous client")
	}
}

func TestVisitorsAreRefused(t *testing.T) {
	r := newRig(t)

	c := r.open(t, nil)

	if c.resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", c.resp.StatusCode)
	}
	if r.subscribers() != 0 {
		t.Error("a refused visitor was subscribed")
	}
}

func TestABannedUserIsRefused(t *testing.T) {
	r := newRig(t)
	u := testdb.NewUser(t)
	r.env.Login(t, u)
	u.Ban(t)

	c := r.open(t, u)

	if c.resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", c.resp.StatusCode)
	}
	if r.subscribers() != 0 {
		t.Error("a refused user was subscribed")
	}
}

func TestPlayersMayConnectBeforeTheCTFStarts(t *testing.T) {
	r := newRig(t)
	testdb.DelayCTF(t)

	r.connect(t, testdb.NewUser(t))
}

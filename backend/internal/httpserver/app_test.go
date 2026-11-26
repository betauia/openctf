package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/betauia/openctf/backend/internal/config"
	"github.com/betauia/openctf/backend/internal/policy"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
	"github.com/betauia/openctf/backend/internal/testutil/testenv"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// newApp is the real route table on the test database and the default policy.
func newApp(t *testing.T) *testenv.Env {
	t.Helper()
	ps, err := policy.LoadFile("../policies/default.json")
	if err != nil {
		t.Fatalf("load default policy: %v", err)
	}
	cfg := config.Config{SecretKey: "test-secret-key", FilesDir: t.TempDir()}
	return &testenv.Env{Mux: NewMux(testdb.Pool, cfg, policy.NewEngine(ps))}
}

// Each feature registers its own routes, so one request to each is enough to show NewMux wired it.
func TestNewMuxRegistersEveryFeature(t *testing.T) {
	app := newApp(t)
	player, admin := testdb.NewUser(t), testdb.NewAdmin(t)

	tests := []struct {
		name   string
		user   *testdb.User
		method string
		path   string
		body   any
		status int
	}{
		{"health", nil, "GET", "/health", nil, http.StatusOK},
		{"auth: logging in as nobody", nil, "POST", "/api/auth/login", map[string]string{"username": "nobody", "password": "x"}, http.StatusUnauthorized},
		{"auth: solves are public", nil, "GET", "/api/auth/solves", nil, http.StatusOK},
		{"teams: the public list", nil, "GET", "/api/teams", nil, http.StatusOK},
		{"teams: the scoreboard needs a login", nil, "GET", "/api/scoreboard", nil, http.StatusUnauthorized},
		{"teams: the scoreboard for a player", player, "GET", "/api/scoreboard", nil, http.StatusOK},
		{"settings: public", nil, "GET", "/api/settings", nil, http.StatusOK},
		{"settings: admin only", player, "GET", "/api/admin/settings", nil, http.StatusForbidden},
		{"settings: for an admin", admin, "GET", "/api/admin/settings", nil, http.StatusOK},
		{"challenges: need a login", nil, "GET", "/api/challenges", nil, http.StatusUnauthorized},
		{"challenges: for a player", player, "GET", "/api/challenges", nil, http.StatusOK},
		{"challenges: admin list", admin, "GET", "/api/admin/challenges", nil, http.StatusOK},
		{"live: needs a login", nil, "GET", "/api/live", nil, http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := app.Do(t, tc.user, tc.method, tc.path, tc.body)

			if rec.Code != tc.status {
				t.Errorf("%s %s = %d, want %d (body %s)", tc.method, tc.path, rec.Code, tc.status, rec.Body)
			}
		})
	}
}

func TestUnknownPathsAreNotFound(t *testing.T) {
	app := newApp(t)

	for _, path := range []string{"/", "/nope", "/api", "/api/nope", "/health/extra"} {
		if rec := app.Get(t, nil, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

func TestWrongMethodsAreRefusedWithAnAllowHeader(t *testing.T) {
	app := newApp(t)

	tests := []struct{ method, path, allowed string }{
		{"DELETE", "/health", "GET"},
		{"GET", "/api/auth/login", "POST"},
		{"PUT", "/api/challenges", "GET"},
		{"POST", "/api/scoreboard", "GET"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := app.Do(t, nil, tc.method, tc.path, nil)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, tc.allowed) {
				t.Errorf("Allow = %q, want it to include %s", allow, tc.allowed)
			}
		})
	}
}

// The challenges handler publishes solves through a function NewMux passes it, and the live hub
// serves them. This follows one solve from the flag submission to a connected stream.
func TestASolvedChallengeIsPublishedToTheLiveFeed(t *testing.T) {
	srv := httptest.NewServer(newApp(t).Mux)
	t.Cleanup(srv.Close)
	user := testdb.NewUser(t)
	testdb.NewTeam(t, user)
	flag := "flag{" + testdb.Unique() + "}"
	testdb.AddChallenge(t, testdb.Challenge{Title: "wired", Category: "misc", Points: 321, Flags: []string{flag}})

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	login, err := client.Post(srv.URL+"/api/auth/login", "application/json",
		strings.NewReader(`{"username":"`+user.Name+`","password":"`+testdb.Password+`"}`))
	if err != nil || login.StatusCode != http.StatusOK {
		t.Fatalf("login: %v, %v", login, err)
	}
	login.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // runs before srv.Close, ending the stream
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/live", nil)
	stream, err := client.Do(req)
	if err != nil || stream.StatusCode != http.StatusOK {
		t.Fatalf("live: %v, %v", stream, err)
	}
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	if line, _ := reader.ReadString('\n'); !strings.HasPrefix(line, ": connected") {
		t.Fatalf("first line = %q, want the connected comment", line)
	}
	time.Sleep(300 * time.Millisecond) // the hub registers the client just after that comment

	events := make(chan string, 1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				events <- strings.TrimSpace(data)
				return
			}
		}
	}()
	submit, err := client.Post(srv.URL+"/api/challenges/submit", "application/json", strings.NewReader(`{"flag":"`+flag+`"}`))
	if err != nil || submit.StatusCode != http.StatusOK {
		t.Fatalf("submit: %v, %v", submit, err)
	}
	submit.Body.Close()

	select {
	case data := <-events:
		var got struct {
			Challenge string `json:"challenge"`
			Category  string `json:"category"`
			Points    int    `json:"points"`
			Username  string `json:"username"`
		}
		if err := json.Unmarshal([]byte(data), &got); err != nil {
			t.Fatalf("event data %q: %v", data, err)
		}
		if got.Challenge != "wired" || got.Category != "misc" || got.Points != 321 || got.Username != user.Name {
			t.Errorf("event = %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the solve never reached the live feed")
	}
}

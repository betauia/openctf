package httpserver

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/config"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/instances"
)

// testServer starts the API against a fresh schema in TEST_DATABASE_URL, which is dropped afterwards.
func testServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := "test_" + uuid.NewV4().String()[:8]

	admin, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	handler, _ := New(database.NewStore(pool), config.Config{SecretKey: "test", UploadDir: t.TempDir()}, fakeProvider{})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, pool
}

type fakeProvider struct{ instances.Kubernetes }

func (fakeProvider) Create(_ context.Context, id uuid.UUID, image string, _ int) (instances.Instance, error) {
	return instances.Instance{ID: "abc123", Image: image, ChallengeID: &id, HostPort: "32768"}, nil
}

// client is a logged-in (or anonymous) API user with its own cookie jar.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, srv *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
}

// do sends body as JSON, decodes the response into out (if non-nil) and returns the status code.
func (c *client) do(method, path string, body, out any) int {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil {
		// The response structs below are mostly untagged, so match "detail" to Detail etc.
		if err := json.Unmarshal(data, out, json.MatchCaseInsensitiveNames(true)); err != nil {
			c.t.Fatalf("%s %s: decode %s: %v", method, path, data, err)
		}
	}
	return resp.StatusCode
}

func (c *client) mustDo(method, path string, body, out any) {
	c.t.Helper()
	if code := c.do(method, path, body, out); code != http.StatusOK {
		c.t.Fatalf("%s %s: status %d", method, path, code)
	}
}

func register(t *testing.T, srv *httptest.Server, name string) (*client, uuid.UUID) {
	c := newClient(t, srv)
	var u struct{ ID uuid.UUID }
	c.mustDo("POST", "/api/auth/register", map[string]string{"username": name, "email": name + "@x.test", "password": "password123"}, &u)
	return c, u.ID
}

type team struct {
	ID         uuid.UUID
	Name       string
	Score      int
	InviteCode *string `json:"invite_code"`
	Members    []struct {
		ID     uuid.UUID
		Solves int
	}
}

func TestEndToEnd(t *testing.T) {
	srv, pool := testServer(t)
	ctx := context.Background()

	// auth
	anon := newClient(t, srv)
	var detail struct{ Detail string }
	if code := anon.do("GET", "/api/auth/me", nil, &detail); code != 401 || detail.Detail != "Not authenticated" {
		t.Fatalf("anonymous /me: %d %q", code, detail.Detail)
	}
	alice, aliceID := register(t, srv, "alice")
	if code := newClient(t, srv).do("POST", "/api/auth/register", map[string]string{"username": "alice", "email": "a2@x.test", "password": "password123"}, nil); code != 400 {
		t.Errorf("duplicate register: %d", code)
	}
	if code := anon.do("POST", "/api/auth/login", map[string]string{"username": "alice", "password": "nope"}, nil); code != 401 {
		t.Errorf("bad login: %d", code)
	}
	newClient(t, srv).mustDo("POST", "/api/auth/login", map[string]string{"username": "alice", "password": "password123"}, nil)

	// challenges
	var chID uuid.UUID
	err := pool.QueryRow(ctx, `INSERT INTO challenges (title, points, category, flags, files, docker_image, is_visible)
		VALUES ('Warmup', 100, 'misc', '["flag{x}"]', '{a.txt}', 'nginx', TRUE) RETURNING id`).Scan(&chID)
	if err != nil {
		t.Fatal(err)
	}
	var list []struct {
		ID    uuid.UUID
		Files []string
	}
	anon.mustDo("GET", "/api/challenges", nil, &list)
	if len(list) != 1 || list[0].ID != chID || len(list[0].Files) != 1 {
		t.Fatalf("challenges = %+v", list)
	}

	var res struct {
		Correct bool
		Points  int
	}
	alice.mustDo("POST", "/api/challenges/submit", map[string]string{"flag": "flag{nope}"}, &res)
	if res.Correct {
		t.Error("wrong flag accepted")
	}
	alice.mustDo("POST", "/api/challenges/submit", map[string]string{"flag": "flag{x}"}, &res)
	alice.mustDo("POST", "/api/challenges/submit", map[string]string{"flag": "flag{x}"}, &res) // resubmitting scores nothing
	if !res.Correct || res.Points != 100 {
		t.Errorf("submit = %+v", res)
	}
	var me struct{ Score, Rank int }
	alice.mustDo("GET", "/api/auth/me", nil, &me)
	if me.Score != 100 || me.Rank != 1 {
		t.Errorf("me = %+v, want score 100 rank 1", me)
	}
	var solved []uuid.UUID
	alice.mustDo("GET", "/api/auth/solves", nil, &solved)
	if len(solved) != 1 || solved[0] != chID {
		t.Errorf("solves = %v", solved)
	}
	var chSolves []struct{ UID uuid.UUID }
	anon.mustDo("GET", fmt.Sprintf("/api/challenges/%s/solves", chID), nil, &chSolves)
	if len(chSolves) != 1 || chSolves[0].UID != aliceID {
		t.Errorf("challenge solves = %+v", chSolves)
	}

	// teams
	var created team
	alice.mustDo("POST", "/api/teams", map[string]string{"name": "Team A"}, &created)
	if created.InviteCode == nil || created.Score != 100 {
		t.Fatalf("created team = %+v", created)
	}
	bob, _ := register(t, srv, "bob")
	if code := bob.do("POST", "/api/teams", map[string]string{"name": "Team A"}, nil); code != 400 {
		t.Errorf("duplicate team name: %d", code)
	}
	var joined team
	bob.mustDo("POST", "/api/teams/join", map[string]string{"invite_code": *created.InviteCode}, &joined)
	if len(joined.Members) != 2 {
		t.Errorf("joined team members = %d", len(joined.Members))
	}

	var all []team
	anon.mustDo("GET", "/api/teams", nil, &all)
	if len(all) != 1 || all[0].InviteCode != nil || len(all[0].Members) != 2 {
		t.Errorf("public team list = %+v", all)
	}
	var profile struct {
		Username string
		Solves   []struct{ Challenge string }
	}
	anon.mustDo("GET", "/api/users/"+aliceID.String(), nil, &profile)
	if profile.Username != "alice" || len(profile.Solves) != 1 || profile.Solves[0].Challenge != "Warmup" {
		t.Errorf("profile = %+v", profile)
	}
	var activity []struct{ Username, Challenge string }
	bob.mustDo("GET", "/api/teams/mine/activity", nil, &activity)
	if len(activity) != 1 || activity[0].Username != "alice" {
		t.Errorf("activity = %+v", activity)
	}

	bob.mustDo("POST", "/api/teams/mine/leave", nil, nil)
	if code := bob.do("POST", "/api/teams/mine/leave", nil, nil); code != 400 {
		t.Errorf("leave twice: %d", code)
	}
	var mine *team
	bob.mustDo("GET", "/api/teams/mine", nil, &mine)
	if mine != nil {
		t.Errorf("bob still has a team: %+v", mine)
	}

	// instances
	if code := anon.do("POST", "/api/challenges/"+chID.String()+"/instance", nil, nil); code != 401 {
		t.Errorf("anonymous spawn: %d", code)
	}
	var inst struct {
		ID          string
		ChallengeID uuid.UUID `json:"challenge_id"`
	}
	alice.mustDo("POST", "/api/challenges/"+chID.String()+"/instance", nil, &inst)
	if inst.ID != "abc123" || inst.ChallengeID != chID {
		t.Errorf("instance = %+v", inst)
	}
	if code := alice.do("GET", "/api/admin/instances", nil, nil); code != http.StatusNotImplemented {
		t.Errorf("list on stub provider: %d", code)
	}
}

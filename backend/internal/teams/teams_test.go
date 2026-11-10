package teams_test

// These tests drive the team endpoints over HTTP through the real mux, against a throwaway
// Postgres database. They need TEST_DATABASE_URL (or DATABASE_URL) pointing at a server where
// the user may CREATE DATABASE; without one they are skipped. Each run creates a database
// named openctf_test_<random>, applies the migrations and drops it afterwards.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/config"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpserver"
	"github.com/betauia/openctf/backend/internal/policy"
)

var (
	pool    *pgxpool.Pool
	srv     *httptest.Server
	skipMsg string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		skipMsg = "set TEST_DATABASE_URL or DATABASE_URL to run the team API tests"
		return m.Run()
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		skipMsg = "database unreachable: " + err.Error()
		return m.Run()
	}
	defer admin.Close(ctx)

	name := "openctf_test_" + strings.ToLower(rand.Text()[:12])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		skipMsg = "cannot create test database: " + err.Error()
		return m.Run()
	}
	defer admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")

	u, err := url.Parse(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse database url:", err)
		return 1
	}
	u.Path = "/" + name
	pool, err = database.Open(ctx, u.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, "open test database:", err)
		return 1
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "migrate test database:", err)
		return 1
	}

	ps, err := policy.LoadFile("../policies/default.json") // tests run in internal/teams
	if err != nil {
		fmt.Fprintln(os.Stderr, "load policies:", err)
		return 1
	}
	srv = httptest.NewServer(httpserver.NewMux(pool, config.Config{SecretKey: "test-secret"}, policy.NewEngine(ps)))
	defer srv.Close()
	return m.Run()
}

// --- helpers ---

type client struct {
	t    *testing.T
	http *http.Client
	ID   string
	Name string
}

type resp struct {
	Status int
	Body   map[string]any
	List   []map[string]any
}

func needDB(t *testing.T) {
	t.Helper()
	if skipMsg != "" {
		t.Skip(skipMsg)
	}
}

// anon returns a client with no session.
func anon(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, http: &http.Client{Jar: jar}}
}

// newUser registers a fresh player and returns a client logged in as them.
func newUser(t *testing.T) *client {
	t.Helper()
	c := anon(t)
	c.Name = "u" + strings.ToLower(rand.Text()[:10])
	r := c.do("POST", "/api/auth/register", map[string]string{
		"username": c.Name, "email": c.Name + "@example.com", "password": "correct horse battery",
	})
	if r.Status != http.StatusOK {
		t.Fatalf("register %s: %d %v", c.Name, r.Status, r.Body)
	}
	c.ID = r.Body["id"].(string)
	return c
}

// newAdmin registers a user and promotes them to admin.
func newAdmin(t *testing.T) *client {
	t.Helper()
	c := newUser(t)
	exec(t, `UPDATE users SET role = 'admin' WHERE id = $1`, c.ID)
	return c
}

func (c *client) do(method, path string, body any) resp {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	r := resp{Status: res.StatusCode}
	if len(bytes.TrimSpace(raw)) > 0 {
		if raw[0] == '[' {
			err = json.Unmarshal(raw, &r.List)
		} else {
			err = json.Unmarshal(raw, &r.Body)
		}
		if err != nil {
			c.t.Fatalf("%s %s: invalid JSON %q", method, path, raw)
		}
	}
	return r
}

// must performs a request and fails the test unless it answers want.
func (c *client) must(want int, method, path string, body any) resp {
	c.t.Helper()
	r := c.do(method, path, body)
	if r.Status != want {
		c.t.Fatalf("%s %s: got %d %v, want %d", method, path, r.Status, r.Body, want)
	}
	return r
}

// createTeam makes c the captain of a new team and returns its id and invite code.
func (c *client) createTeam() (id, code string) {
	c.t.Helper()
	r := c.must(200, "POST", "/api/teams", map[string]string{"name": teamName()})
	id = r.Body["id"].(string)
	mine := c.must(200, "GET", "/api/teams/mine", nil)
	return id, mine.Body["invite_code"].(string)
}

func (c *client) join(code string) {
	c.t.Helper()
	c.must(200, "POST", "/api/teams/join", map[string]string{"invite_code": code})
}

func teamName() string { return "t" + strings.ToLower(rand.Text()[:10]) }

func exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

// setMaxTeamSize changes the setting for the rest of the test.
func setMaxTeamSize(t *testing.T, n int) {
	exec(t, `UPDATE settings SET max_team_size = $1`, n)
	t.Cleanup(func() { exec(t, `UPDATE settings SET max_team_size = 5`) })
}

// endCTF moves the CTF into the past for the rest of the test.
func endCTF(t *testing.T) {
	exec(t, `UPDATE settings SET ctf_start = now() - interval '2 hours', ctf_end = now() - interval '1 hour'`)
	t.Cleanup(func() {
		exec(t, `UPDATE settings SET ctf_start = now() - interval '1 hour', ctf_end = now() + interval '7 days'`)
	})
}

// --- tests ---

func TestCreateAndRead(t *testing.T) {
	needDB(t)
	cap := newUser(t)
	name := teamName()

	anon(t).must(401, "POST", "/api/teams", map[string]string{"name": teamName()})
	cap.must(400, "POST", "/api/teams", map[string]string{"name": "ab"})
	cap.must(400, "POST", "/api/teams", map[string]string{"name": strings.Repeat("x", 33)})

	id := cap.must(200, "POST", "/api/teams", map[string]string{"name": name}).Body["id"].(string)

	// Already in a team: team:create needs no_team.
	cap.must(403, "POST", "/api/teams", map[string]string{"name": teamName()})
	// Name taken.
	newUser(t).must(400, "POST", "/api/teams", map[string]string{"name": name})

	mine := cap.must(200, "GET", "/api/teams/mine", nil).Body
	if mine["id"] != id || mine["name"] != name || mine["captain_id"] != cap.ID {
		t.Errorf("mine = %v", mine)
	}
	if code, _ := mine["invite_code"].(string); code == "" {
		t.Error("mine has no invite_code")
	}
	if members := mine["members"].([]any); len(members) != 1 {
		t.Errorf("members = %v, want 1", members)
	}

	pub := anon(t).must(200, "GET", "/api/teams/"+id, nil).Body
	if _, ok := pub["invite_code"]; ok {
		t.Error("public team exposes invite_code")
	}
	if pub["member_count"] != float64(1) {
		t.Errorf("member_count = %v", pub["member_count"])
	}

	list := anon(t).must(200, "GET", "/api/teams?limit=100", nil).List
	found := false
	for _, tm := range list {
		if _, ok := tm["invite_code"]; ok {
			t.Error("team list exposes invite_code")
		}
		found = found || tm["id"] == id
	}
	if !found {
		t.Error("new team missing from GET /api/teams")
	}

	anon(t).must(400, "GET", "/api/teams?limit=0", nil)
	anon(t).must(400, "GET", "/api/teams?offset=-1", nil)
	anon(t).must(404, "GET", "/api/teams/00000000-0000-7000-8000-000000000000", nil)
	anon(t).must(422, "GET", "/api/teams/not-a-uuid", nil)
	newUser(t).must(404, "GET", "/api/teams/mine", nil)
	anon(t).must(401, "GET", "/api/teams/mine", nil)
}

func TestJoinAndLeave(t *testing.T) {
	needDB(t)
	cap, mem := newUser(t), newUser(t)
	id, code := cap.createTeam()

	mem.must(404, "POST", "/api/teams/join", map[string]string{"invite_code": "wrong"})
	mem.join(code)
	mem.must(400, "POST", "/api/teams/join", map[string]string{"invite_code": code})

	if n := anon(t).must(200, "GET", "/api/teams/"+id, nil).Body["member_count"]; n != float64(2) {
		t.Errorf("member_count = %v, want 2", n)
	}

	// The captain can't leave while others remain (CaptainMustTransfer).
	cap.must(403, "POST", "/api/teams/mine/leave", nil)

	mem.must(200, "POST", "/api/teams/mine/leave", nil)
	mem.must(400, "POST", "/api/teams/mine/leave", nil)

	// The last member leaving deletes the team.
	cap.must(200, "POST", "/api/teams/mine/leave", nil)
	anon(t).must(404, "GET", "/api/teams/"+id, nil)
}

func TestTeamFull(t *testing.T) {
	needDB(t)
	setMaxTeamSize(t, 2)
	cap := newUser(t)
	_, code := cap.createTeam()
	newUser(t).join(code)
	newUser(t).must(403, "POST", "/api/teams/join", map[string]string{"invite_code": code})
}

func TestKickAndTransferCaptain(t *testing.T) {
	needDB(t)
	cap, a, b, outsider := newUser(t), newUser(t), newUser(t), newUser(t)
	_, code := cap.createTeam()
	a.join(code)
	b.join(code)

	a.must(403, "DELETE", "/api/teams/mine/members/"+b.ID, nil) // not captain
	cap.must(400, "DELETE", "/api/teams/mine/members/"+cap.ID, nil)
	cap.must(404, "DELETE", "/api/teams/mine/members/"+outsider.ID, nil)
	outsider.must(400, "DELETE", "/api/teams/mine/members/"+a.ID, nil) // not in a team
	cap.must(200, "DELETE", "/api/teams/mine/members/"+b.ID, nil)
	b.must(404, "GET", "/api/teams/mine", nil)

	a.must(403, "PUT", "/api/teams/mine/captain", map[string]string{"user_id": a.ID})
	cap.must(404, "PUT", "/api/teams/mine/captain", map[string]string{"user_id": outsider.ID})
	cap.must(200, "PUT", "/api/teams/mine/captain", map[string]string{"user_id": a.ID})

	if got := a.must(200, "GET", "/api/teams/mine", nil).Body["captain_id"]; got != a.ID {
		t.Errorf("captain_id = %v, want %v", got, a.ID)
	}
	// The old captain is a plain member now, so they can leave.
	cap.must(200, "POST", "/api/teams/mine/leave", nil)
}

func TestUpdateTeam(t *testing.T) {
	needDB(t)
	cap, mem := newUser(t), newUser(t)
	_, oldCode := cap.createTeam()
	mem.join(oldCode)

	newUser(t).must(400, "PATCH", "/api/teams/mine", map[string]string{"name": teamName()}) // no team
	cap.must(400, "PATCH", "/api/teams/mine", map[string]any{})
	cap.must(400, "PATCH", "/api/teams/mine", map[string]string{"name": "ab"})
	mem.must(403, "PATCH", "/api/teams/mine", map[string]string{"name": teamName()})

	name := teamName()
	r := cap.must(200, "PATCH", "/api/teams/mine", map[string]string{"name": name})
	if _, ok := r.Body["invite_code"]; ok {
		t.Error("rename alone returned an invite_code")
	}
	if got := cap.must(200, "GET", "/api/teams/mine", nil).Body["name"]; got != name {
		t.Errorf("name = %v, want %v", got, name)
	}

	// Another team can't take the name.
	other := newUser(t)
	other.createTeam()
	other.must(400, "PATCH", "/api/teams/mine", map[string]string{"name": name})

	newCode := cap.must(200, "PATCH", "/api/teams/mine", map[string]bool{"regenerate_invite": true}).Body["invite_code"].(string)
	if newCode == oldCode {
		t.Fatal("invite code was not regenerated")
	}
	joiner := newUser(t)
	joiner.must(404, "POST", "/api/teams/join", map[string]string{"invite_code": oldCode})
	joiner.join(newCode)
}

func TestBannedTeam(t *testing.T) {
	needDB(t)
	admin := newAdmin(t)
	cap, mem := newUser(t), newUser(t)
	id, code := cap.createTeam()
	mem.join(code)

	admin.must(200, "PATCH", "/api/admin/teams/"+id, map[string]bool{"banned": true})

	cap.must(200, "GET", "/api/teams/mine", nil) // members can still read their team
	cap.must(403, "PATCH", "/api/teams/mine", map[string]string{"name": teamName()})
	cap.must(403, "DELETE", "/api/teams/mine/members/"+mem.ID, nil)
	cap.must(403, "PUT", "/api/teams/mine/captain", map[string]string{"user_id": mem.ID})
	mem.must(403, "POST", "/api/teams/mine/leave", nil)
	newUser(t).must(403, "POST", "/api/teams/join", map[string]string{"invite_code": code})

	admin.must(200, "PATCH", "/api/admin/teams/"+id, map[string]bool{"banned": false})
	mem.must(200, "POST", "/api/teams/mine/leave", nil)
}

func TestRosterFrozenAfterCTF(t *testing.T) {
	needDB(t)
	cap, mem := newUser(t), newUser(t)
	id, code := cap.createTeam()
	mem.join(code)
	admin := newAdmin(t)

	endCTF(t)
	newUser(t).must(403, "POST", "/api/teams/join", map[string]string{"invite_code": code})
	mem.must(403, "POST", "/api/teams/mine/leave", nil)
	cap.must(403, "DELETE", "/api/teams/mine/members/"+mem.ID, nil)

	// Admins aren't frozen.
	admin.must(200, "DELETE", "/api/admin/teams/"+id+"/members/"+mem.ID, nil)
}

func TestAdminAccess(t *testing.T) {
	needDB(t)
	player := newUser(t)
	id, _ := newUser(t).createTeam()

	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/admin/teams"},
		{"GET", "/api/admin/teams/" + id},
		{"PATCH", "/api/admin/teams/" + id},
		{"DELETE", "/api/admin/teams/" + id},
		{"DELETE", "/api/admin/teams/" + id + "/members/" + player.ID},
	} {
		var body any
		if tc.method == "PATCH" {
			body = map[string]string{"name": teamName()}
		}
		anon(t).must(401, tc.method, tc.path, body)
		player.must(403, tc.method, tc.path, body)
	}

	// A captain may manage their own team, but not through the admin routes.
	cap, mem := newUser(t), newUser(t)
	capID, code := cap.createTeam()
	mem.join(code)
	cap.must(403, "PATCH", "/api/admin/teams/"+capID, map[string]string{"name": teamName()})
	cap.must(403, "DELETE", "/api/admin/teams/"+capID+"/members/"+mem.ID, nil)
	cap.must(403, "DELETE", "/api/admin/teams/"+capID, nil)
	cap.must(200, "GET", "/api/teams/mine", nil)
}

func TestAdminReadAndUpdate(t *testing.T) {
	needDB(t)
	admin := newAdmin(t)
	id, code := newUser(t).createTeam()

	got := admin.must(200, "GET", "/api/admin/teams/"+id, nil).Body
	if got["invite_code"] != code {
		t.Errorf("admin team invite_code = %v, want %v", got["invite_code"], code)
	}
	admin.must(404, "GET", "/api/admin/teams/00000000-0000-7000-8000-000000000000", nil)

	list := admin.must(200, "GET", "/api/admin/teams?limit=100", nil).List
	found := false
	for _, tm := range list {
		found = found || tm["id"] == id
	}
	if !found {
		t.Error("team missing from GET /api/admin/teams")
	}

	name := teamName()
	admin.must(200, "PATCH", "/api/admin/teams/"+id, map[string]string{"name": name})
	if got := anon(t).must(200, "GET", "/api/teams/"+id, nil).Body["name"]; got != name {
		t.Errorf("name = %v, want %v", got, name)
	}
	admin.must(400, "PATCH", "/api/admin/teams/"+id, map[string]string{"name": "ab"})
	admin.must(404, "PATCH", "/api/admin/teams/00000000-0000-7000-8000-000000000000", map[string]string{"name": teamName()})
}

func TestAdminKick(t *testing.T) {
	needDB(t)
	admin := newAdmin(t)
	cap, a, b, outsider := newUser(t), newUser(t), newUser(t), newUser(t)
	id, code := cap.createTeam()
	a.join(code)
	b.join(code)

	admin.must(404, "DELETE", "/api/admin/teams/"+id+"/members/"+outsider.ID, nil)
	admin.must(422, "DELETE", "/api/admin/teams/"+id+"/members/nope", nil)

	// Kicking the captain promotes the remaining member with the oldest account (a).
	admin.must(200, "DELETE", "/api/admin/teams/"+id+"/members/"+cap.ID, nil)
	if got := a.must(200, "GET", "/api/teams/mine", nil).Body["captain_id"]; got != a.ID {
		t.Errorf("captain_id = %v, want %v", got, a.ID)
	}
	cap.must(404, "GET", "/api/teams/mine", nil)

	admin.must(200, "DELETE", "/api/admin/teams/"+id+"/members/"+b.ID, nil)
	// Kicking the last member deletes the team.
	admin.must(200, "DELETE", "/api/admin/teams/"+id+"/members/"+a.ID, nil)
	anon(t).must(404, "GET", "/api/teams/"+id, nil)
}

func TestHiddenTeam(t *testing.T) {
	needDB(t)
	admin := newAdmin(t)
	cap := newUser(t)
	id, _ := cap.createTeam()
	exec(t, `UPDATE teams SET hidden = TRUE WHERE id = $1`, id)

	inList := func(list []map[string]any) bool {
		for _, tm := range list {
			if tm["id"] == id {
				return true
			}
		}
		return false
	}

	// Hidden from the public...
	anon(t).must(404, "GET", "/api/teams/"+id, nil)
	if inList(anon(t).must(200, "GET", "/api/teams?limit=100", nil).List) {
		t.Error("hidden team in GET /api/teams")
	}
	// ...but not from admins or its own members, who can still change it.
	admin.must(200, "GET", "/api/admin/teams/"+id, nil)
	if !inList(admin.must(200, "GET", "/api/admin/teams?limit=100", nil).List) {
		t.Error("hidden team missing from GET /api/admin/teams")
	}
	cap.must(200, "GET", "/api/teams/mine", nil)
	cap.must(200, "PATCH", "/api/teams/mine", map[string]string{"name": teamName()})

	// Admins can unhide it through PATCH.
	admin.must(200, "PATCH", "/api/admin/teams/"+id, map[string]bool{"hidden": false})
	if got := anon(t).must(200, "GET", "/api/teams/"+id, nil).Body["hidden"]; got != false {
		t.Errorf("hidden = %v, want false", got)
	}
}

func TestAdminDelete(t *testing.T) {
	needDB(t)
	admin := newAdmin(t)
	cap, mem := newUser(t), newUser(t)
	id, code := cap.createTeam()
	mem.join(code)

	admin.must(200, "DELETE", "/api/admin/teams/"+id, nil)
	anon(t).must(404, "GET", "/api/teams/"+id, nil)
	cap.must(404, "GET", "/api/teams/mine", nil)
	mem.must(404, "GET", "/api/teams/mine", nil)
	admin.must(404, "DELETE", "/api/admin/teams/"+id, nil)

	// Former members are free to start a new team.
	mem.createTeam()
}

func TestTeamNameRules(t *testing.T) {
	needDB(t)
	cap := newUser(t)
	name := teamName()

	// Surrounding whitespace is trimmed, so "  ab  " is too short.
	cap.must(400, "POST", "/api/teams", map[string]string{"name": "  ab  "})
	cap.must(400, "POST", "/api/teams", map[string]string{"name": "bad\x00name"})
	id := cap.must(200, "POST", "/api/teams", map[string]string{"name": "  " + name + "  "}).Body["id"].(string)
	if got := anon(t).must(200, "GET", "/api/teams/"+id, nil).Body["name"]; got != name {
		t.Errorf("name = %q, want %q", got, name)
	}
	// Names are unique regardless of case.
	newUser(t).must(400, "POST", "/api/teams", map[string]string{"name": strings.ToUpper(name)})
}

func TestAdminSearchMembersAndCaptain(t *testing.T) {
	needDB(t)
	admin := newAdmin(t)
	cap, mem, outsider := newUser(t), newUser(t), newUser(t)
	id, code := cap.createTeam()
	mem.join(code)
	name := admin.must(200, "GET", "/api/admin/teams/"+id, nil).Body["name"].(string)

	// Search matches part of the name, ignoring case, and treats wildcards literally.
	list := admin.must(200, "GET", "/api/admin/teams?q="+strings.ToUpper(name[2:8]), nil).List
	if len(list) != 1 || list[0]["id"] != id {
		t.Errorf("search %q = %v, want only %s", name[2:8], list, id)
	}
	if list := admin.must(200, "GET", "/api/admin/teams?q=%25", nil).List; len(list) != 0 {
		t.Errorf("search for %% matched %d teams, want 0", len(list))
	}
	admin.must(400, "GET", "/api/admin/teams?q="+strings.Repeat("x", 33), nil)

	// The admin view lists members.
	members := admin.must(200, "GET", "/api/admin/teams/"+id, nil).Body["members"].([]any)
	if len(members) != 2 {
		t.Errorf("members = %v, want 2", members)
	}

	// Admins can change the captain, but only to a member.
	admin.must(400, "PATCH", "/api/admin/teams/"+id, map[string]string{"captain_id": outsider.ID})
	admin.must(200, "PATCH", "/api/admin/teams/"+id, map[string]string{"captain_id": mem.ID})
	if got := mem.must(200, "GET", "/api/teams/mine", nil).Body["captain_id"]; got != mem.ID {
		t.Errorf("captain_id = %v, want %v", got, mem.ID)
	}
	admin.must(400, "PATCH", "/api/admin/teams/"+id, map[string]any{})
}

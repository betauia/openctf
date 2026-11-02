package auth_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
)

// testAuth serves the auth routes against a fresh schema in TEST_DATABASE_URL, which is dropped afterwards.
func testAuth(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
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

	mux := http.NewServeMux()
	auth.New(database.NewStore(pool), "test-secret").Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, pool
}

// post sends a POST with an optional JSON body and cookies and returns the response with its body closed.
func post(t *testing.T, srv *httptest.Server, path, body string, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func cookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

const aliceBody = `{"username":"alice","email":"alice@x.test","password":"password123"}`

// registerAlice registers a user and returns her session and refresh cookies.
func registerAlice(t *testing.T, srv *httptest.Server) (session, refresh *http.Cookie) {
	t.Helper()
	resp := post(t, srv, "/api/auth/register", aliceBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register: status %d", resp.StatusCode)
	}
	session, refresh = cookie(resp, "session"), cookie(resp, "refresh")
	if session == nil || refresh == nil {
		t.Fatalf("register cookies = %v, want session and refresh", resp.Cookies())
	}
	return session, refresh
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	srv, _ := testAuth(t)
	tests := map[string]string{
		"invalid email":  `{"username":"alice","email":"not-an-email","password":"password123"}`,
		"short password": `{"username":"alice","email":"alice@x.test","password":"short"}`,
		"short username": `{"username":"a","email":"alice@x.test","password":"password123"}`,
	}
	for name, body := range tests {
		if resp := post(t, srv, "/api/auth/register", body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	srv, _ := testAuth(t)
	registerAlice(t, srv)
	tests := map[string]string{
		"same username": `{"username":"alice","email":"other@x.test","password":"password123"}`,
		"same email":    `{"username":"other","email":"alice@x.test","password":"password123"}`,
	}
	for name, body := range tests {
		if resp := post(t, srv, "/api/auth/register", body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
}

func TestLogin(t *testing.T) {
	srv, _ := testAuth(t)
	registerAlice(t, srv)

	resp := post(t, srv, "/api/auth/login", `{"username":"alice","password":"password123"}`)
	if resp.StatusCode != http.StatusOK || cookie(resp, "session") == nil || cookie(resp, "refresh") == nil {
		t.Errorf("valid login: status %d, cookies %v", resp.StatusCode, resp.Cookies())
	}

	// Unknown user and wrong password must be indistinguishable.
	for name, body := range map[string]string{
		"wrong password": `{"username":"alice","password":"wrong-password"}`,
		"unknown user":   `{"username":"nobody","password":"password123"}`,
	} {
		resp := post(t, srv, "/api/auth/login", body)
		if resp.StatusCode != http.StatusUnauthorized || cookie(resp, "session") != nil {
			t.Errorf("%s: status %d, cookies %v; want 401 and no session", name, resp.StatusCode, resp.Cookies())
		}
	}
}

func TestMeWithSessionCookie(t *testing.T) {
	srv, _ := testAuth(t)
	session, _ := registerAlice(t, srv)

	req, _ := http.NewRequest("GET", srv.URL+"/api/auth/me", nil)
	req.AddCookie(session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	srv, _ := testAuth(t)
	_, refresh := registerAlice(t, srv)

	resp := post(t, srv, "/api/auth/refresh", "", refresh)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	next := cookie(resp, "refresh")
	if cookie(resp, "session") == nil || next == nil || next.Value == refresh.Value {
		t.Errorf("cookies = %v, want a new session and a rotated refresh token", resp.Cookies())
	}

	// The rotated token works too.
	if resp := post(t, srv, "/api/auth/refresh", "", next); resp.StatusCode != http.StatusOK {
		t.Errorf("rotated token: status = %d, want 200", resp.StatusCode)
	}
}

func TestRefreshUnknownToken(t *testing.T) {
	srv, _ := testAuth(t)
	resp := post(t, srv, "/api/auth/refresh", "", &http.Cookie{Name: "refresh", Value: "unknown"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if c := cookie(resp, "session"); c == nil || c.MaxAge >= 0 {
		t.Errorf("session cookie = %+v, want it cleared", c)
	}
}

func TestRefreshReplayRevokesFamily(t *testing.T) {
	srv, pool := testAuth(t)
	_, first := registerAlice(t, srv)

	resp := post(t, srv, "/api/auth/refresh", "", first)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first refresh: status = %d", resp.StatusCode)
	}
	second := cookie(resp, "refresh")

	// Replaying within the reuse window is tolerated (e.g. two tabs racing).
	if resp := post(t, srv, "/api/auth/refresh", "", first); resp.StatusCode != http.StatusOK {
		t.Fatalf("replay inside reuse window: status = %d, want 200", resp.StatusCode)
	}

	// Age the used token past the window; replaying it now looks like theft.
	if _, err := pool.Exec(context.Background(), `UPDATE refresh_tokens SET used_at = now() - interval '1 hour' WHERE used_at IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if resp := post(t, srv, "/api/auth/refresh", "", first); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("late replay: status = %d, want 401", resp.StatusCode)
	}

	// Every token from the same login is revoked, including the legitimate newest one.
	if resp := post(t, srv, "/api/auth/refresh", "", second); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("newest token after replay: status = %d, want 401", resp.StatusCode)
	}
}

// The tests above attach cookies by hand, which bypasses Path scoping. A cookie jar behaves like a browser.
func TestLogoutRevokesRefreshTokenViaCookieJar(t *testing.T) {
	srv, _ := testAuth(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	do := func(path string) int {
		t.Helper()
		resp, err := client.Post(srv.URL+path, "application/json", strings.NewReader(aliceBody))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if status := do("/api/auth/register"); status != http.StatusOK {
		t.Fatalf("register: status = %d", status)
	}
	u, _ := url.Parse(srv.URL)
	var refresh *http.Cookie
	for _, c := range jar.Cookies(u) {
		if c.Name == "refresh" {
			refresh = c
		}
	}
	if refresh == nil {
		t.Fatal("jar has no refresh cookie after register")
	}
	saved := *refresh

	// The jar only returns cookies whose Path matches, so the logout request carries the cookie only if the path allows it.
	if status := do("/api/auth/logout"); status != http.StatusOK {
		t.Fatalf("logout: status = %d, want 200", status)
	}
	if resp := post(t, srv, "/api/auth/refresh", "", &saved); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status = %d, want 401", resp.StatusCode)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	srv, _ := testAuth(t)
	_, refresh := registerAlice(t, srv)

	resp := post(t, srv, "/api/auth/logout", "", refresh)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: status = %d, want 200", resp.StatusCode)
	}
	if resp := post(t, srv, "/api/auth/refresh", "", refresh); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status = %d, want 401", resp.StatusCode)
	}
}

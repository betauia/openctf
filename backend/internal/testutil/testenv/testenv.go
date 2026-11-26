// Package testenv builds the HTTP routes under test on top of the shared test database, using
// the default policy file, and sends requests as logged-in users. It is for tests only.
package testenv

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/policy"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

const secret = "test-secret-key"

// Env is a mux with the auth routes and whatever else the test registered.
type Env struct {
	Mux  *http.ServeMux
	Auth *auth.Auth
}

// New returns an Env with the auth routes registered, plus the routes of each register function.
// Each call makes a fresh Auth, so login backoff starts empty.
func New(t *testing.T, register ...func(mux *http.ServeMux, a *auth.Auth)) *Env {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	ps, err := policy.LoadFile(filepath.Join(filepath.Dir(file), "..", "..", "policies", "default.json"))
	if err != nil {
		t.Fatalf("load default policy: %v", err)
	}
	a := auth.New(testdb.Pool, secret, policy.NewEngine(ps))
	mux := http.NewServeMux()
	a.Register(mux)
	for _, r := range register {
		r(mux, a)
	}
	return &Env{Mux: mux, Auth: a}
}

// Login logs u in through POST /api/auth/login and keeps the session cookie on u.
func (e *Env) Login(t *testing.T, u *testdb.User) {
	t.Helper()
	rec := e.send(t, nil, "POST", "/api/auth/login", map[string]string{"username": u.Name, "password": testdb.Password})
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			u.Cookie = c
		}
	}
	if u.Cookie == nil {
		t.Fatalf("login as %s: status %d, body %s", u.Name, rec.Code, rec.Body)
	}
}

// Do sends a request as u, or with no session when u is nil, logging u in on first use. body is
// JSON-encoded unless nil.
func (e *Env) Do(t *testing.T, u *testdb.User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	if u != nil && u.Cookie == nil {
		e.Login(t, u)
	}
	return e.send(t, u, method, path, body)
}

// Get sends a GET request as u, or with no session when u is nil.
func (e *Env) Get(t *testing.T, u *testdb.User, path string) *httptest.ResponseRecorder {
	t.Helper()
	return e.Do(t, u, "GET", path, nil)
}

func (e *Env) send(t *testing.T, u *testdb.User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	if u != nil && u.Cookie != nil {
		r.AddCookie(u.Cookie)
	}
	rec := httptest.NewRecorder()
	e.Mux.ServeHTTP(rec, r)
	return rec
}

// Expect fails the test unless rec has the given status and, when detail isn't empty, the given
// {"detail": ...} error message.
func Expect(t *testing.T, rec *httptest.ResponseRecorder, status int, detail string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	if detail == "" {
		return
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body, err)
	}
	if got, _ := body["detail"].(string); got != detail {
		t.Errorf("detail = %q, want %q", got, detail)
	}
}

// Decode returns the JSON body of a 200 response as T.
func Decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body %q: %v", rec.Body, err)
	}
	return v
}

// DoWith is Do with extra request headers, for example a Range header.
func (e *Env) DoWith(t *testing.T, u *testdb.User, method, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	if u != nil && u.Cookie == nil {
		e.Login(t, u)
	}
	r := httptest.NewRequest(method, path, nil)
	for k, v := range header {
		r.Header[k] = v
	}
	if u != nil && u.Cookie != nil {
		r.AddCookie(u.Cookie)
	}
	rec := httptest.NewRecorder()
	e.Mux.ServeHTTP(rec, r)
	return rec
}

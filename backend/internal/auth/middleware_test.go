package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
)

// These cases all return before the store is touched, so a nil store is fine.

func TestCurrentUserWithoutValidSession(t *testing.T) {
	a := New(nil, "secret")
	cases := map[string]*http.Cookie{
		"no cookie":    nil,
		"garbage":      {Name: cookieName, Value: "garbage"},
		"wrong secret": {Name: cookieName, Value: sessions.NewToken([]byte("other"), uuid.NewV7(), time.Now())},
		"expired":      {Name: cookieName, Value: sessions.NewToken([]byte("secret"), uuid.NewV7(), time.Now().Add(-sessions.MaxAge-time.Minute))},
	}
	for name, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		if c != nil {
			r.AddCookie(c)
		}
		u, err := a.CurrentUser(r)
		if u != nil || err != nil {
			t.Errorf("%s: CurrentUser = %v, %v; want nil, nil", name, u, err)
		}
	}
}

func TestOptionalAnonymous(t *testing.T) {
	a := New(nil, "secret")
	called := false
	h := a.Optional(func(w http.ResponseWriter, r *http.Request, u *dbgen.User) {
		called = true
		if u != nil {
			t.Errorf("user = %v, want nil", u)
		}
	})
	h(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !called {
		t.Error("handler not called")
	}
}

func TestRequireRejectsAnonymous(t *testing.T) {
	a := New(nil, "secret")
	for name, wrap := range map[string]func(func(http.ResponseWriter, *http.Request, dbgen.User)) http.HandlerFunc{
		"Require":      a.Require,
		"RequireAdmin": a.RequireAdmin,
	} {
		h := wrap(func(http.ResponseWriter, *http.Request, dbgen.User) { t.Errorf("%s: handler called", name) })
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, w.Code)
		}
	}
}

func TestSetSession(t *testing.T) {
	a := New(nil, "secret")
	id := uuid.NewV7()
	w := httptest.NewRecorder()
	a.setSession(w, id)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v", cookies)
	}
	c := cookies[0]
	if c.Name != cookieName || !c.HttpOnly || c.Path != "/" || c.SameSite != http.SameSiteLaxMode || c.MaxAge != int(sessions.MaxAge.Seconds()) {
		t.Errorf("cookie = %+v", c)
	}
	if got, ok := sessions.VerifyToken(a.secret, c.Value, time.Now()); !ok || got != id {
		t.Errorf("cookie token = %v, %v; want %v, true", got, ok, id)
	}
}

func TestLogout(t *testing.T) {
	a := New(nil, "secret")
	w := httptest.NewRecorder()
	a.logout(w, httptest.NewRequest("POST", "/api/auth/logout", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != cookieName || cookies[1].Name != refreshCookieName ||
		cookies[0].MaxAge >= 0 || cookies[1].MaxAge >= 0 || cookies[1].Path != refreshPath {
		t.Errorf("logout cookies = %+v, want expired session and refresh cookies", cookies)
	}
}

func TestRefreshWithoutCookie(t *testing.T) {
	a := New(nil, "secret")
	w := httptest.NewRecorder()
	a.refresh(w, httptest.NewRequest("POST", refreshPath, nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSetRefreshCookie(t *testing.T) {
	a := New(nil, "secret")
	w := httptest.NewRecorder()
	a.setRefreshCookie(w, "tok")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v", cookies)
	}
	c := cookies[0]
	if c.Name != refreshCookieName || c.Value != "tok" || !c.HttpOnly || c.Path != refreshPath ||
		c.SameSite != http.SameSiteStrictMode || c.MaxAge != int(sessions.RefreshMaxAge.Seconds()) {
		t.Errorf("cookie = %+v", c)
	}
}

func TestRegisterValidation(t *testing.T) {
	a := New(nil, "secret")
	tests := []struct {
		name, body string
		wantStatus int
		wantDetail string
	}{
		{"bad json", `{`, http.StatusUnprocessableEntity, "Invalid request body"},
		{"short username", `{"username":"a","email":"a@x","password":"password123"}`, http.StatusBadRequest, "Username"},
		{"long username", `{"username":"` + strings.Repeat("a", 33) + `","email":"a@x","password":"password123"}`, http.StatusBadRequest, "Username"},
		{"short password", `{"username":"alice","email":"a@x","password":"short"}`, http.StatusBadRequest, "Password"},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		a.register(w, httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(tt.body)))
		if w.Code != tt.wantStatus || !strings.Contains(w.Body.String(), tt.wantDetail) {
			t.Errorf("%s: got %d %s, want %d containing %q", tt.name, w.Code, w.Body, tt.wantStatus, tt.wantDetail)
		}
	}
}

func TestAnonymousSolvesIsEmptyList(t *testing.T) {
	a := New(nil, "secret")
	w := httptest.NewRecorder()
	a.solves(w, httptest.NewRequest("GET", "/api/auth/solves", nil), nil)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("got %d %s, want 200 []", w.Code, w.Body)
	}
}

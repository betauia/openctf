package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/policy"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

const testSecret = "test-secret-key"

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// newMux returns a mux with the auth routes on a fresh Auth, so each test starts with empty
// login backoff. Users live in the shared database; each test makes its own with unique names.
func newMux(t *testing.T) *http.ServeMux {
	t.Helper()
	ps, err := policy.LoadFile("../policies/default.json")
	if err != nil {
		t.Fatalf("load default policy: %v", err)
	}
	mux := http.NewServeMux()
	New(testdb.Pool, testSecret, policy.NewEngine(ps)).Register(mux)
	return mux
}

// newUser inserts a user whose password is testdb.Password.
func newUser(t *testing.T) User {
	t.Helper()
	id := uuid.NewV7()
	name := "u" + strings.ReplaceAll(id.String(), "-", "")[12:]
	u, err := createUser(context.Background(), testdb.Pool, id, name, name+"@example.com", testdb.PasswordHash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func ban(t *testing.T, u User) {
	t.Helper()
	if _, err := testdb.Pool.Exec(context.Background(), `UPDATE users SET banned = true WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("ban user: %v", err)
	}
}

// do sends a request with the given cookies. body is JSON-encoded unless nil.
func do(t *testing.T, mux http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		if c != nil {
			r.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func login(t *testing.T, mux http.Handler, username, password string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, mux, "POST", "/api/auth/login", map[string]string{"username": username, "password": password}, cookies...)
}

// cookie returns the last Set-Cookie with the given name, or nil.
func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	var found *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			found = c
		}
	}
	return found
}

func cleared(c *http.Cookie) bool { return c != nil && c.MaxAge < 0 }

// session is the cookies a browser holds after logging in.
type session struct{ session, refresh, device *http.Cookie }

func startSession(t *testing.T, mux http.Handler, u User) session {
	t.Helper()
	rec := login(t, mux, u.Username, testdb.Password)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status %d, body %s", rec.Code, rec.Body)
	}
	return session{cookie(rec, "session"), cookie(rec, "refresh"), cookie(rec, "device")}
}

func detail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body, err)
	}
	s, _ := body["detail"].(string)
	return s
}

func refresh(t *testing.T, mux http.Handler, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, mux, "POST", "/api/auth/refresh", nil, c)
}

// meStatus is the status of GET /api/auth/me with the given session cookie, which tells whether
// that session is still valid.
func meStatus(t *testing.T, mux http.Handler, c *http.Cookie) int {
	t.Helper()
	return do(t, mux, "GET", "/api/auth/me", nil, c).Code
}

func TestLoginSuccess(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)

	rec := login(t, mux, u.Username, testdb.Password)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var body userRef
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != u.ID || body.Username != u.Username {
		t.Errorf("body = %+v, want id %v username %q", body, u.ID, u.Username)
	}

	s, r, d := cookie(rec, "session"), cookie(rec, "refresh"), cookie(rec, "device")
	if s == nil || r == nil || d == nil {
		t.Fatalf("cookies: session %v, refresh %v, device %v; want all set", s, r, d)
	}
	if id, _, ok := sessions.VerifyToken([]byte(testSecret), s.Value, time.Now()); !ok || id != u.ID {
		t.Errorf("session cookie does not verify for user %v", u.ID)
	}
	if id, ok := sessions.VerifyDeviceToken([]byte(testSecret), d.Value); !ok || id != u.ID {
		t.Errorf("device cookie does not verify for user %v", u.ID)
	}
	if !s.HttpOnly || !r.HttpOnly || !d.HttpOnly {
		t.Error("cookies must be HttpOnly")
	}
	if r.Path != "/api/auth" {
		t.Errorf("refresh cookie path = %q, want /api/auth", r.Path)
	}
	if meStatus(t, mux, s) != http.StatusOK {
		t.Error("the new session cookie is not accepted")
	}
}

func TestLoginIsCaseInsensitiveOnUsername(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)

	if rec := login(t, mux, strings.ToUpper(u.Username), testdb.Password); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginWhileLoggedInIs409(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)

	rec := login(t, mux, u.Username, testdb.Password, s.session)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if got := detail(t, rec); got != "Already logged in" {
		t.Errorf("detail = %q", got)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("a 409 must not touch cookies, got %v", rec.Result().Cookies())
	}
}

// The 409 comes before the password check, so a logged-in user sending a wrong password still
// gets 409, and none of those attempts count against the login backoff.
func TestLoginWhileLoggedInDoesNotChargeBackoff(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)

	for range 10 {
		if rec := login(t, mux, u.Username, "wrong password", s.session); rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
	}

	if rec := login(t, mux, u.Username, testdb.Password); rec.Code != http.StatusOK {
		t.Errorf("login without a session after the 409s: status %d, want 200", rec.Code)
	}
}

func TestLoginWithInvalidSessionCookieProceeds(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	bad := &http.Cookie{Name: "session", Value: "not-a-token"}

	if rec := login(t, mux, u.Username, testdb.Password, bad); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginAfterLogoutIsAllowed(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)
	do(t, mux, "POST", "/api/auth/logout", nil, s.refresh)

	// The browser still holds the old session cookie, but its refresh family is revoked.
	if rec := login(t, mux, u.Username, testdb.Password, s.session); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginBadCredentials(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)

	wrong := login(t, mux, u.Username, "wrong password")
	unknown := login(t, mux, "nobody"+u.Username, testdb.Password)

	for name, rec := range map[string]*httptest.ResponseRecorder{"wrong password": wrong, "unknown user": unknown} {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: set cookies %v", name, rec.Result().Cookies())
		}
	}
	// The two failures must be indistinguishable, or the endpoint reveals which usernames exist.
	if wrong.Body.String() != unknown.Body.String() {
		t.Errorf("bodies differ: %q vs %q", wrong.Body, unknown.Body)
	}
	if got := detail(t, wrong); got != "Invalid username or password" {
		t.Errorf("detail = %q", got)
	}
}

func TestLoginBannedUserIsDenied(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	ban(t, u)

	rec := login(t, mux, u.Username, testdb.Password)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("a denied login set cookies %v", rec.Result().Cookies())
	}
}

// Backoff allows 5 free failures, then each attempt is followed by a wait, so the seventh
// attempt in a row is refused straight away.
func failUntilBlocked(t *testing.T, mux http.Handler, username string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	for i := 1; i <= 6; i++ {
		if rec := login(t, mux, username, "wrong password", cookies...); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, rec.Code)
		}
	}
	return login(t, mux, username, "wrong password", cookies...)
}

func TestLoginBackoffReturns429WithRetryAfter(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)

	rec := failUntilBlocked(t, mux, u.Username)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
	// A blocked attempt is refused before the password is checked, so the right password fails too.
	if rec := login(t, mux, u.Username, testdb.Password); rec.Code != http.StatusTooManyRequests {
		t.Errorf("correct password while blocked: status = %d, want 429", rec.Code)
	}
}

func TestLoginBackoffAppliesToUnknownUsernames(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)

	if rec := failUntilBlocked(t, mux, "nobody"+u.Username); rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
}

// Strangers guessing at a username must not lock the real owner out of a browser they have used.
func TestLoginKnownDeviceIsNotBlockedByStrangers(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	device := startSession(t, mux, u).device

	if rec := failUntilBlocked(t, mux, u.Username); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("strangers: status = %d, want 429", rec.Code)
	}

	if rec := login(t, mux, u.Username, testdb.Password, device); rec.Code != http.StatusOK {
		t.Errorf("owner on a known device: status = %d, want 200", rec.Code)
	}
}

// A known device has its own bucket, so its failures don't slow anyone else down.
func TestLoginKnownDeviceFailuresDoNotBlockOthers(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	device := startSession(t, mux, u).device

	if rec := failUntilBlocked(t, mux, u.Username, device); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("known device: status = %d, want 429", rec.Code)
	}

	if rec := login(t, mux, u.Username, testdb.Password); rec.Code != http.StatusOK {
		t.Errorf("stranger with the right password: status = %d, want 200", rec.Code)
	}
}

// A device cookie for another user must not unlock this user's separate bucket.
func TestLoginDeviceCookieOfAnotherUserGivesNoSeparateBucket(t *testing.T) {
	mux := newMux(t)
	victim, other := newUser(t), newUser(t)
	otherDevice := startSession(t, mux, other).device

	if rec := failUntilBlocked(t, mux, victim.Username); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}

	if rec := login(t, mux, victim.Username, testdb.Password, otherDevice); rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
}

func TestRefreshRotatesTokenWithinTheSameFamily(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)

	rec := refresh(t, mux, s.refresh)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	newRefresh, newSession := cookie(rec, "refresh"), cookie(rec, "session")
	if newRefresh == nil || newSession == nil {
		t.Fatalf("cookies: refresh %v, session %v; want both set", newRefresh, newSession)
	}
	if newRefresh.Value == s.refresh.Value {
		t.Error("the refresh token was not rotated")
	}
	_, oldFamily, _ := sessions.VerifyToken([]byte(testSecret), s.session.Value, time.Now())
	id, family, ok := sessions.VerifyToken([]byte(testSecret), newSession.Value, time.Now())
	if !ok || id != u.ID || family != oldFamily {
		t.Errorf("new session: user %v family %v ok %v; want user %v family %v", id, family, ok, u.ID, oldFamily)
	}
	if meStatus(t, mux, newSession) != http.StatusOK {
		t.Error("the new session cookie is not accepted")
	}
}

// Several tabs refreshing at once, or a lost response, send the same token twice. Inside the
// reuse window the second use must succeed, not be mistaken for theft.
func TestRefreshReuseInsideWindowSucceeds(t *testing.T) {
	mux := newMux(t)
	s := startSession(t, mux, newUser(t))

	first := refresh(t, mux, s.refresh)
	second := refresh(t, mux, s.refresh)

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses = %d, %d; want 200, 200", first.Code, second.Code)
	}
	// Both responses hand out a live token, and the family is intact.
	for i, rec := range []*httptest.ResponseRecorder{first, second} {
		if got := refresh(t, mux, cookie(rec, "refresh")); got.Code != http.StatusOK {
			t.Errorf("token from response %d: status = %d, want 200", i+1, got.Code)
		}
	}
}

func ageRefreshToken(t *testing.T, c *http.Cookie) {
	t.Helper()
	_, err := testdb.Pool.Exec(context.Background(),
		`UPDATE refresh_tokens SET used_at = now() - interval '1 minute' WHERE token_hash = $1`,
		sessions.HashRefreshToken(c.Value))
	if err != nil {
		t.Fatalf("age refresh token: %v", err)
	}
}

func TestRefreshReuseAfterWindowRevokesTheFamily(t *testing.T) {
	mux := newMux(t)
	s := startSession(t, mux, newUser(t))
	rotated := refresh(t, mux, s.refresh)
	newRefresh, newSession := cookie(rotated, "refresh"), cookie(rotated, "session")
	ageRefreshToken(t, s.refresh) // the old token was used well outside the window

	replay := refresh(t, mux, s.refresh)

	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay: status = %d, want 401", replay.Code)
	}
	if !cleared(cookie(replay, "session")) || !cleared(cookie(replay, "refresh")) {
		t.Errorf("replay did not clear the cookies: %v", replay.Result().Cookies())
	}
	// The thief and the owner share a family, so the owner's newest token dies with it.
	if got := refresh(t, mux, newRefresh); got.Code != http.StatusUnauthorized {
		t.Errorf("newest refresh token after theft: status = %d, want 401", got.Code)
	}
	if got := meStatus(t, mux, newSession); got != http.StatusUnauthorized {
		t.Errorf("session of the revoked family: status = %d, want 401", got)
	}
}

func TestRefreshReplayDoesNotAffectOtherFamilies(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	stolen := startSession(t, mux, u)
	other := startSession(t, mux, u) // a second device
	refresh(t, mux, stolen.refresh)
	ageRefreshToken(t, stolen.refresh)

	refresh(t, mux, stolen.refresh)

	if got := refresh(t, mux, other.refresh); got.Code != http.StatusOK {
		t.Errorf("other device: status = %d, want 200", got.Code)
	}
}

func TestRefreshWithoutOrWithUnknownToken(t *testing.T) {
	mux := newMux(t)
	s := startSession(t, mux, newUser(t))

	none := refresh(t, mux, nil)
	unknown := refresh(t, mux, &http.Cookie{Name: "refresh", Value: "never-issued"})

	if none.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: status = %d, want 401", none.Code)
	}
	if unknown.Code != http.StatusUnauthorized {
		t.Errorf("unknown token: status = %d, want 401", unknown.Code)
	}
	// Neither revoked anything: the real token still works.
	if got := refresh(t, mux, s.refresh); got.Code != http.StatusOK {
		t.Errorf("real token afterwards: status = %d, want 200", got.Code)
	}
}

func TestLogoutRevokesTheFamilyAndClearsCookies(t *testing.T) {
	mux := newMux(t)
	s := startSession(t, mux, newUser(t))

	rec := do(t, mux, "POST", "/api/auth/logout", nil, s.refresh)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !cleared(cookie(rec, "session")) || !cleared(cookie(rec, "refresh")) {
		t.Errorf("cookies not cleared: %v", rec.Result().Cookies())
	}
	if got := refresh(t, mux, s.refresh); got.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status = %d, want 401", got.Code)
	}
	if got := meStatus(t, mux, s.session); got != http.StatusUnauthorized {
		t.Errorf("old session after logout: status = %d, want 401", got)
	}
}

func TestLogoutKeepsTheDeviceCookie(t *testing.T) {
	mux := newMux(t)
	s := startSession(t, mux, newUser(t))

	rec := do(t, mux, "POST", "/api/auth/logout", nil, s.refresh)

	if c := cookie(rec, "device"); c != nil {
		t.Errorf("logout touched the device cookie: %v", c)
	}
}

func TestLogoutWithoutCookiesStillSucceeds(t *testing.T) {
	mux := newMux(t)

	for name, c := range map[string]*http.Cookie{
		"no cookie":     nil,
		"unknown token": {Name: "refresh", Value: "never-issued"},
	} {
		rec := do(t, mux, "POST", "/api/auth/logout", nil, c)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", name, rec.Code)
		}
	}
}

func TestLogoutOnlyEndsThisDevice(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	a, b := startSession(t, mux, u), startSession(t, mux, u)

	do(t, mux, "POST", "/api/auth/logout", nil, a.refresh)

	if got := refresh(t, mux, b.refresh); got.Code != http.StatusOK {
		t.Errorf("other device: status = %d, want 200", got.Code)
	}
}

func changePassword(t *testing.T, mux http.Handler, current, next string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, mux, "PUT", "/api/auth/password", map[string]string{"current_password": current, "new_password": next}, c)
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)

	rec := changePassword(t, mux, "not my password", "a brand new password", s.session)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := detail(t, rec); got != "Current password is incorrect" {
		t.Errorf("detail = %q", got)
	}
	if got := login(t, mux, u.Username, testdb.Password); got.Code != http.StatusOK {
		t.Errorf("the old password stopped working: status %d", got.Code)
	}
	if got := refresh(t, mux, s.refresh); got.Code != http.StatusOK {
		t.Errorf("the session was revoked: status %d", got.Code)
	}
}

func TestChangePasswordRejectsShortPassword(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)

	tests := []struct{ name, password string }{
		{"empty", ""},
		{"11 ascii characters", "elevenchars"},
		// 22 bytes but 11 runes: the limit counts characters, not bytes.
		{"11 multi-byte characters", strings.Repeat("å", 11)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := changePassword(t, mux, testdb.Password, tc.password, s.session)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if got := detail(t, rec); got != "Password must be at least 12 characters" {
				t.Errorf("detail = %q", got)
			}
			if got := login(t, mux, u.Username, testdb.Password); got.Code != http.StatusOK {
				t.Errorf("the old password stopped working: status %d", got.Code)
			}
		})
	}
}

func TestChangePasswordAcceptsTwelveMultiByteCharacters(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)
	next := strings.Repeat("å", 12)

	if rec := changePassword(t, mux, testdb.Password, next, s.session); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}

	if got := login(t, mux, u.Username, next); got.Code != http.StatusOK {
		t.Errorf("login with the new password: status %d", got.Code)
	}
}

func TestChangePasswordRequiresASession(t *testing.T) {
	mux := newMux(t)

	rec := changePassword(t, mux, testdb.Password, "a brand new password", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestChangePasswordLogsOutOtherDevices(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	here, elsewhere := startSession(t, mux, u), startSession(t, mux, u)
	const next = "a brand new password"

	rec := changePassword(t, mux, testdb.Password, next, here.session)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if got := login(t, mux, u.Username, testdb.Password); got.Code != http.StatusUnauthorized {
		t.Errorf("old password after the change: status %d, want 401", got.Code)
	}
	if got := login(t, mux, u.Username, next); got.Code != http.StatusOK {
		t.Errorf("new password: status %d, want 200", got.Code)
	}
	if got := refresh(t, mux, elsewhere.refresh); got.Code != http.StatusUnauthorized {
		t.Errorf("other device refresh: status %d, want 401", got.Code)
	}
	if got := meStatus(t, mux, elsewhere.session); got != http.StatusUnauthorized {
		t.Errorf("other device session: status %d, want 401", got)
	}
	// The caller's old tokens are revoked too, but the response carries a fresh session.
	if got := refresh(t, mux, here.refresh); got.Code != http.StatusUnauthorized {
		t.Errorf("caller's old refresh token: status %d, want 401", got.Code)
	}
	newSession, newRefresh := cookie(rec, "session"), cookie(rec, "refresh")
	if newSession == nil || newRefresh == nil {
		t.Fatalf("cookies: session %v, refresh %v; want both set", newSession, newRefresh)
	}
	if got := meStatus(t, mux, newSession); got != http.StatusOK {
		t.Errorf("caller's new session: status %d, want 200", got)
	}
	if got := refresh(t, mux, newRefresh); got.Code != http.StatusOK {
		t.Errorf("caller's new refresh token: status %d, want 200", got.Code)
	}
}

func TestMe(t *testing.T) {
	mux := newMux(t)
	u := newUser(t)
	s := startSession(t, mux, u)

	t.Run("without a session", func(t *testing.T) {
		if got := do(t, mux, "GET", "/api/auth/me", nil); got.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", got.Code)
		}
	})
	t.Run("with a garbage session", func(t *testing.T) {
		if got := meStatus(t, mux, &http.Cookie{Name: "session", Value: "garbage"}); got != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", got)
		}
	})
	t.Run("with a session", func(t *testing.T) {
		rec := do(t, mux, "GET", "/api/auth/me", nil, s.session)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			ID       uuid.UUID `json:"id"`
			Username string    `json:"username"`
			Role     string    `json:"role"`
			Rank     int32     `json:"rank"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.ID != u.ID || body.Username != u.Username || body.Role != "user" || body.Rank < 1 {
			t.Errorf("body = %+v", body)
		}
	})
}

func TestSolves(t *testing.T) {
	mux := newMux(t)
	s := startSession(t, mux, newUser(t))

	tests := []struct {
		name   string
		cookie *http.Cookie
	}{
		{"logged out", nil},
		{"logged in without solves", s.session},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, mux, "GET", "/api/auth/solves", nil, tc.cookie)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
				t.Errorf("body = %q, want []", got)
			}
		})
	}
}

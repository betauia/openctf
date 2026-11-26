package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth/passwords"
	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

// These tests cover POST /api/auth/register. They use the helpers in auth_test.go.

const (
	usernameLengthMsg = "Username must be 5-32 characters"
	usernameCharsMsg  = "Username can't start or end with spaces or contain invisible characters"
	passwordMsg       = "Password must be at least 12 characters"
	emailLengthMsg    = "Email must be 5-255 characters"
	emailInvalidMsg   = "Please provide a valid email address"
	takenMsg          = "Username or email already taken"
)

// registration is what a client sends.
type registration struct{ username, email, password string }

// fresh returns a valid registration nobody has used.
func fresh() registration {
	name := "reg" + testdb.Unique()
	return registration{username: name, email: name + "@example.com", password: "a good long password"}
}

func (r registration) body() map[string]string {
	return map[string]string{"username": r.username, "email": r.email, "password": r.password}
}

func register(t *testing.T, mux http.Handler, r registration, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, mux, "POST", "/api/auth/register", r.body(), cookies...)
}

// usersWith counts users whose username (ignoring case) or email matches.
func usersWith(t *testing.T, r registration) int {
	return testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE lower(username) = lower($1) OR email = $2`, r.username, r.email)
}

// --- success ---

func TestRegisterCreatesTheAccountAndLogsItIn(t *testing.T) {
	mux := newMux(t)
	r := fresh()

	rec := register(t, mux, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var body userRef
	if err := jsonDecode(rec, &body); err != nil {
		t.Fatal(err)
	}
	if body.Username != r.username || body.ID == (uuid.UUID{}) {
		t.Errorf("body = %+v, want the new id and username %q", body, r.username)
	}
	s, refresh, device := cookie(rec, "session"), cookie(rec, "refresh"), cookie(rec, "device")
	if s == nil || refresh == nil || device == nil {
		t.Fatalf("cookies: session %v, refresh %v, device %v; want all set", s, refresh, device)
	}
	if id, _, ok := sessions.VerifyToken([]byte(testSecret), s.Value, time.Now()); !ok || id != body.ID {
		t.Errorf("session cookie does not verify for the new user %v", body.ID)
	}
	if id, ok := sessions.VerifyDeviceToken([]byte(testSecret), device.Value); !ok || id != body.ID {
		t.Errorf("device cookie does not verify for the new user %v", body.ID)
	}
	if !s.HttpOnly || !refresh.HttpOnly || !device.HttpOnly || refresh.Path != "/api/auth" {
		t.Errorf("cookie flags: session HttpOnly %v, refresh HttpOnly %v path %q, device HttpOnly %v",
			s.HttpOnly, refresh.HttpOnly, refresh.Path, device.HttpOnly)
	}
}

func TestRegisterStoresASafeDefaultAccount(t *testing.T) {
	mux := newMux(t)
	r := fresh()
	rec := register(t, mux, r)
	var body userRef
	if err := jsonDecode(rec, &body); err != nil {
		t.Fatal(err)
	}

	row := testdb.Pool.QueryRow(t.Context(), `SELECT username, email, role, score, banned, team_id, password_hash FROM users WHERE id = $1`, body.ID)
	var username, email, role, hash string
	var score int
	var banned bool
	var team *uuid.UUID
	if err := row.Scan(&username, &email, &role, &score, &banned, &team, &hash); err != nil {
		t.Fatalf("the account was not stored: %v", err)
	}

	if username != r.username || email != r.email {
		t.Errorf("stored %q %q, want %q %q", username, email, r.username, r.email)
	}
	if role != "user" || score != 0 || banned || team != nil {
		t.Errorf("role %q score %d banned %v team %v; want a plain user with no score, ban or team", role, score, banned, team)
	}
	if hash == r.password || strings.Contains(hash, r.password) {
		t.Error("the password is stored in the clear")
	}
	if !passwords.Verify(r.password, hash) || passwords.Verify("another password!", hash) {
		t.Error("the stored hash doesn't verify the chosen password, and only that password")
	}
}

func TestRegisteredUserCanUseTheAccountAtOnce(t *testing.T) {
	mux := newMux(t)
	r := fresh()
	rec := register(t, mux, r)
	session, refreshCookie := cookie(rec, "session"), cookie(rec, "refresh")

	if got := meStatus(t, mux, session); got != http.StatusOK {
		t.Errorf("/me with the new session = %d, want 200", got)
	}
	if got := refresh(t, mux, refreshCookie); got.Code != http.StatusOK {
		t.Errorf("refreshing the new session = %d, want 200", got.Code)
	}
	if got := login(t, mux, r.username, r.password); got.Code != http.StatusOK {
		t.Errorf("logging in afterwards = %d, want 200", got.Code)
	}
	if got := login(t, mux, r.username, "not the password"); got.Code != http.StatusUnauthorized {
		t.Errorf("logging in with another password = %d, want 401", got.Code)
	}
}

// Two requests can both pass the "already taken" check; the database's unique index then decides
// which one wins, and the loser must get the same 400, not a 500.
func TestConcurrentRegistrationsOfTheSameNameCreateOneAccount(t *testing.T) {
	mux := newMux(t)
	name := "race" + testdb.Unique()
	const attempts = 8

	codes := make([]int, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := registration{username: name, email: name + string(rune('a'+i)) + "@example.com", password: "a good long password"}
			rec := do(t, mux, "POST", "/api/auth/register", r.body())
			codes[i] = rec.Code
			if rec.Code == http.StatusBadRequest && detail(t, rec) != takenMsg {
				t.Errorf("attempt %d: detail = %q, want %q", i, detail(t, rec), takenMsg)
			}
		}()
	}
	close(start)
	wg.Wait()

	ok, refused := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusBadRequest:
			refused++
		}
	}
	if ok != 1 || refused != attempts-1 {
		t.Errorf("statuses = %v, want exactly one 200 and the rest 400", codes)
	}
	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE lower(username) = lower($1)`, name); n != 1 {
		t.Errorf("%d accounts named %q, want 1", n, name)
	}
}

func TestConcurrentRegistrationsOfTheSameEmailCreateOneAccount(t *testing.T) {
	mux := newMux(t)
	email := "shared" + testdb.Unique() + "@example.com"
	const attempts = 6

	codes := make([]int, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := registration{username: "mail" + testdb.Unique(), email: email, password: "a good long password"}
			codes[i] = do(t, mux, "POST", "/api/auth/register", r.body()).Code
		}()
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		} else if c != http.StatusBadRequest {
			t.Errorf("unexpected status %d in %v", c, codes)
		}
	}
	if ok != 1 {
		t.Errorf("statuses = %v, want exactly one 200", codes)
	}
	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE email = $1`, email); n != 1 {
		t.Errorf("%d accounts with the email, want 1", n)
	}
}

// --- validation ---

func TestRegisterUsernameLength(t *testing.T) {
	mux := newMux(t)
	unique := testdb.Unique()
	tests := []struct {
		name     string
		username string
		ok       bool
	}{
		{"empty", "", false},
		{"4 characters", unique[:4], false},
		{"4 multi-byte characters (8 bytes)", "éééé", false},
		{"5 characters", unique[:5], true},
		{"32 characters", unique + strings.Repeat("y", 12), true},
		{"32 multi-byte characters (64 bytes)", unique + strings.Repeat("é", 12), true},
		{"33 characters", unique + strings.Repeat("z", 13), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := fresh()
			r.username = tc.username

			rec := register(t, mux, r)

			if tc.ok {
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
				}
				return
			}
			expectRefused(t, rec, usernameLengthMsg)
			if n := usersWith(t, r); n != 0 {
				t.Errorf("%d users created by a refused registration", n)
			}
		})
	}
}

func TestRegisterUsernameCharacters(t *testing.T) {
	mux := newMux(t)
	tests := []struct {
		name     string
		username string
		ok       bool
	}{
		{"a leading space", " abcdef", false},
		{"a trailing space", "abcdef ", false},
		{"a leading tab", "\tabcdef", false},
		{"a trailing newline", "abcdef\n", false},
		{"a zero-width space inside", "abc​def", false},
		{"a zero-width space at the end", "abcdef​", false},
		{"a control character", "abc\x01def", false},
		{"a tab inside", "abc\tdef", false},
		{"a space inside", "abc " + testdb.Unique(), true},
		{"accented letters", "Zoë" + testdb.Unique(), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := fresh()
			r.username = tc.username

			rec := register(t, mux, r)

			if tc.ok {
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
				}
				return
			}
			expectRefused(t, rec, usernameCharsMsg)
			if n := usersWith(t, r); n != 0 {
				t.Errorf("%d users created by a refused registration", n)
			}
		})
	}
}

func TestRegisterPasswordLength(t *testing.T) {
	mux := newMux(t)
	tests := []struct {
		name     string
		password string
		ok       bool
	}{
		{"empty", "", false},
		{"11 characters", "elevenchars", false},
		{"11 multi-byte characters (22 bytes)", strings.Repeat("å", 11), false},
		{"12 characters", "twelve chars", true},
		{"12 multi-byte characters", strings.Repeat("å", 12), true},
		{"spaces count and aren't trimmed", "eleven chars ", true},
		{"a very long password", strings.Repeat("long", 200), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := fresh()
			r.password = tc.password

			rec := register(t, mux, r)

			if tc.ok {
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
				}
				if got := login(t, mux, r.username, tc.password); got.Code != http.StatusOK {
					t.Errorf("logging in with the chosen password = %d, want 200", got.Code)
				}
				return
			}
			expectRefused(t, rec, passwordMsg)
			if n := usersWith(t, r); n != 0 {
				t.Errorf("%d users created by a refused registration", n)
			}
		})
	}
}

func TestRegisterEmail(t *testing.T) {
	mux := newMux(t)
	local := testdb.Unique()
	tests := []struct {
		name   string
		email  string
		detail string // empty when the email must be accepted
	}{
		{"empty", "", emailLengthMsg},
		{"3 characters", "a@b", emailLengthMsg},
		{"4 characters", "a@b.", emailLengthMsg},
		{"256 characters", strings.Repeat("x", 256-len("@example.com")) + "@example.com", emailLengthMsg},
		{"5 characters but no address", "abcde", emailInvalidMsg},
		{"no @", "missing-at.example.com", emailInvalidMsg},
		{"no domain", "someone@", emailInvalidMsg},
		{"no local part", "@example.com", emailInvalidMsg},
		{"two @", "two@@example.com", emailInvalidMsg},
		{"spaces in the address", "some one@example.com", emailInvalidMsg},
		{"a name before the address", "Bob Builder <" + local + "@example.com>", emailInvalidMsg},
		{"angle brackets only", "<" + local + "@example.com>", emailInvalidMsg},
		{"a trailing comment", local + "@example.com (a comment)", emailInvalidMsg},
		{"surrounding spaces", " " + local + "@example.com ", emailInvalidMsg},
		{"an ordinary address", local + "@example.com", ""},
		{"a plus tag", local + "+tag@example.com", ""},
		{"a subdomain", local + "@mail.example.co.uk", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := fresh()
			r.email = tc.email

			rec := register(t, mux, r)

			if tc.detail == "" {
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
				}
				return
			}
			expectRefused(t, rec, tc.detail)
			if n := testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE lower(username) = lower($1)`, r.username); n != 0 {
				t.Errorf("%d users created by a refused registration", n)
			}
		})
	}
}

// Emails are stored lowercase, so the same mailbox written in another case is the same account.
func TestRegisterStoresTheEmailInLowercase(t *testing.T) {
	mux := newMux(t)
	r := fresh()
	mixed := r
	mixed.email = strings.ToUpper(r.username[:4]) + r.username[4:] + "@Example.COM"

	rec := register(t, mux, mixed)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got := testdb.QueryRow[string](t, `SELECT email FROM users WHERE username = $1`, r.username); got != r.email {
		t.Errorf("stored %q, want %q", got, r.email)
	}
}

func TestRegisterRefusesATakenEmailInAnyCase(t *testing.T) {
	mux := newMux(t)
	first := fresh()
	register(t, mux, first)
	second := fresh()
	second.email = strings.ToUpper(first.email)

	expectRefused(t, register(t, mux, second), takenMsg)

	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE lower(email) = lower($1)`, first.email); n != 1 {
		t.Errorf("%d accounts with the email, want 1", n)
	}
}

// The checks run in a fixed order, so a request with several mistakes is told about the first.
func TestRegisterReportsTheFirstProblemFirst(t *testing.T) {
	mux := newMux(t)
	r := registration{username: "abc", email: "ab", password: "short"}

	expectRefused(t, register(t, mux, r), usernameLengthMsg)
	r.username = "abcdef "
	expectRefused(t, register(t, mux, r), usernameCharsMsg)
	r.username = "reg" + testdb.Unique()
	expectRefused(t, register(t, mux, r), passwordMsg)
	r.password = "a good long password"
	expectRefused(t, register(t, mux, r), emailLengthMsg)
	r.email = "nonsense-without-at"
	expectRefused(t, register(t, mux, r), emailInvalidMsg)
	r.email = r.username + "@example.com"
	if rec := register(t, mux, r); rec.Code != http.StatusOK {
		t.Errorf("with every problem fixed, status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
}

func TestRegisterRefusesAnUnreadableBody(t *testing.T) {
	mux := newMux(t)

	tests := map[string]any{
		"a list":                     []string{"not", "an", "object"},
		"a username that isn't text": map[string]any{"username": 12345, "email": "a@example.com", "password": "a good long password"},
		"a password that isn't text": map[string]any{"username": "reg" + testdb.Unique(), "email": "a@example.com", "password": []int{1}},
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			rec := do(t, mux, "POST", "/api/auth/register", body)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 (body %s)", rec.Code, rec.Body)
			}
		})
	}
	t.Run("no body at all", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/auth/register", nil))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", rec.Code)
		}
	})
}

// --- duplicates ---

func TestRegisterRefusesATakenUsernameInAnyCase(t *testing.T) {
	mux := newMux(t)
	existing := newUser(t)
	for _, name := range []string{existing.Username, strings.ToUpper(existing.Username), strings.ToLower(existing.Username)} {
		r := fresh()
		r.username = name

		rec := register(t, mux, r)

		expectRefused(t, rec, takenMsg)
		if n := testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE email = $1`, r.email); n != 0 {
			t.Errorf("registering %q created a user with the new email", name)
		}
	}
}

func TestRegisterRefusesATakenEmail(t *testing.T) {
	mux := newMux(t)
	existing := newUser(t)
	r := fresh()
	r.email = existing.Email

	rec := register(t, mux, r)

	expectRefused(t, rec, takenMsg)
	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE lower(username) = lower($1)`, r.username); n != 0 {
		t.Error("a user was created although the email was taken")
	}
}

// The answer is the same for a taken username and a taken email, so it doesn't say which exists.
func TestRegisterDoesNotSayWhichOfUsernameAndEmailWasTaken(t *testing.T) {
	mux := newMux(t)
	existing := newUser(t)
	byName, byEmail := fresh(), fresh()
	byName.username = existing.Username
	byEmail.email = existing.Email

	a, b := register(t, mux, byName), register(t, mux, byEmail)

	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Errorf("a taken username answers %d %q, a taken email answers %d %q; want identical answers", a.Code, a.Body, b.Code, b.Body)
	}
}

func TestARefusedRegistrationLeavesTheExistingAccountAlone(t *testing.T) {
	mux := newMux(t)
	existing := newUser(t)
	r := fresh()
	r.username = existing.Username
	r.password = "an attacker's password"

	register(t, mux, r)

	if got := login(t, mux, existing.Username, testdb.Password); got.Code != http.StatusOK {
		t.Errorf("the existing user can no longer log in: %d", got.Code)
	}
	if got := login(t, mux, existing.Username, r.password); got.Code != http.StatusUnauthorized {
		t.Errorf("the attacker's password logs in: %d", got.Code)
	}
}

// --- gating ---

func TestRegisterIsClosedWhenRegistrationIsOff(t *testing.T) {
	mux := newMux(t)
	testdb.ChangeSettings(t, `registration = false`)
	r := fresh()

	rec := register(t, mux, r)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body %s)", rec.Code, rec.Body)
	}
	if got := detail(t, rec); got != "Registration is closed" {
		t.Errorf("detail = %q, want Registration is closed", got)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("a refused registration set cookies %v", rec.Result().Cookies())
	}
	if n := usersWith(t, r); n != 0 {
		t.Errorf("%d users created while registration is closed", n)
	}
}

// Closed registration is checked before the body is read, so a bad request is also just a 403.
func TestRegisterWhenClosedDoesNotLookAtTheBody(t *testing.T) {
	mux := newMux(t)
	testdb.ChangeSettings(t, `registration = false`)

	for name, body := range map[string]any{
		"an invalid registration": map[string]string{"username": "a", "email": "b", "password": "c"},
		"a list":                  []int{1, 2, 3},
		"nothing":                 nil,
	} {
		t.Run(name, func(t *testing.T) {
			rec := do(t, mux, "POST", "/api/auth/register", body)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403 (body %s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestRegisterWorksAgainWhenRegistrationReopens(t *testing.T) {
	mux := newMux(t)
	testdb.ChangeSettings(t, `registration = false`)
	if rec := register(t, mux, fresh()); rec.Code != http.StatusForbidden {
		t.Fatalf("closed: status = %d, want 403", rec.Code)
	}

	testdb.Exec(t, `UPDATE settings SET registration = true`)

	if rec := register(t, mux, fresh()); rec.Code != http.StatusOK {
		t.Errorf("reopened: status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
}

// Like login, registering with a session cookie should say you are already logged in. Today the
// handler creates a second account and replaces the cookies. This test is expected to fail until
// the handler refuses it.
func TestRegisterWhileLoggedInIs409(t *testing.T) {
	mux := newMux(t)
	user := newUser(t)
	s := startSession(t, mux, user)
	r := fresh()

	rec := register(t, mux, r, s.session)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	if got := detail(t, rec); got != "Already logged in" {
		t.Errorf("detail = %q, want Already logged in", got)
	}
	if n := usersWith(t, r); n != 0 {
		t.Errorf("%d accounts created for a logged-in user", n)
	}
}

// expectRefused checks a 400 with the given detail.
func expectRefused(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if got := detail(t, rec); got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("a refused registration set cookies %v", rec.Result().Cookies())
	}
}

func jsonDecode(rec *httptest.ResponseRecorder, v any) error {
	return json.Unmarshal(rec.Body.Bytes(), v)
}

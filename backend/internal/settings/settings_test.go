package settings

import (
	"net/http"
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

// newEnv returns the settings routes on a fresh Handler, so the in-memory theme and rules start
// from the defaults. THEME is read when the Handler is made.
func newEnv(t *testing.T) *testenv.Env {
	t.Helper()
	return testenv.New(t, func(mux *http.ServeMux, a *auth.Auth) { New(testdb.Pool, a).Register(mux) })
}

// baseline puts the settings row in a known state and restores the original when the test ends.
func baseline(t *testing.T) (start, end time.Time) {
	t.Helper()
	start = time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	end = time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	testdb.ChangeSettings(t, `ctf_name = 'Baseline CTF', ctf_start = $1, ctf_end = $2, registration = true, max_team_size = 5`, start, end)
	return start, end
}

func put(t *testing.T, e *testenv.Env, u *testdb.User, body map[string]any) Settings {
	t.Helper()
	return testenv.Decode[Settings](t, e.Do(t, u, "PUT", "/api/admin/settings", body))
}

func current(t *testing.T, e *testenv.Env) Settings {
	t.Helper()
	return testenv.Decode[Settings](t, e.Get(t, nil, "/api/settings"))
}

// --- views ---

func TestPublicSettingsNeedNoLogin(t *testing.T) {
	e := newEnv(t)
	start, end := baseline(t)

	got := current(t, e)

	if got.CTFName != "Baseline CTF" || !got.Registration || got.MaxTeamSize != 5 {
		t.Errorf("settings = %+v", got)
	}
	if got.CTFStart == nil || !got.CTFStart.Equal(start) || got.CTFEnd == nil || !got.CTFEnd.Equal(end) {
		t.Errorf("window = %v to %v, want %v to %v", got.CTFStart, got.CTFEnd, start, end)
	}
	if got.Theme != Themes[0] || got.Rules != "" {
		t.Errorf("theme %q rules %q, want the first theme and no rules", got.Theme, got.Rules)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("updated_at is not set")
	}
}

func TestAdminSettingsAreOnlyForAdmins(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	player, maintainer, admin := testdb.NewUser(t), testdb.NewMaintainer(t), testdb.NewAdmin(t)

	testenv.Expect(t, e.Get(t, nil, "/api/admin/settings"), http.StatusUnauthorized, "Not authenticated")
	testenv.Expect(t, e.Get(t, player, "/api/admin/settings"), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, e.Get(t, maintainer, "/api/admin/settings"), http.StatusForbidden, "Not allowed")

	got := testenv.Decode[Settings](t, e.Get(t, admin, "/api/admin/settings"))
	if want := current(t, e); got.CTFName != want.CTFName || got.MaxTeamSize != want.MaxTeamSize || got.Theme != want.Theme {
		t.Errorf("the admin view %+v differs from the public one %+v", got, want)
	}
}

func TestOnlyAdminsCanChangeSettings(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	body := map[string]any{"ctf_name": "Hijacked", "max_team_size": 99, "theme": "betauia", "rules": "mine"}

	testenv.Expect(t, e.Do(t, nil, "PUT", "/api/admin/settings", body), http.StatusUnauthorized, "Not authenticated")
	testenv.Expect(t, e.Do(t, testdb.NewUser(t), "PUT", "/api/admin/settings", body), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, e.Do(t, testdb.NewMaintainer(t), "PUT", "/api/admin/settings", body), http.StatusForbidden, "Not allowed")

	if got := current(t, e); got.CTFName != "Baseline CTF" || got.MaxTeamSize != 5 || got.Theme != Themes[0] || got.Rules != "" {
		t.Errorf("settings changed to %+v", got)
	}
}

// --- updating ---

func TestUpdatingOneFieldLeavesTheOthersAlone(t *testing.T) {
	admin := testdb.NewAdmin(t)
	newStart := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Second)
	newEnd := time.Now().Add(5 * time.Hour).UTC().Truncate(time.Second)

	tests := []struct {
		name  string
		body  map[string]any
		check func(t *testing.T, got Settings, before Settings)
	}{
		{"ctf_name", map[string]any{"ctf_name": "Renamed CTF"}, func(t *testing.T, got, _ Settings) {
			if got.CTFName != "Renamed CTF" {
				t.Errorf("name = %q", got.CTFName)
			}
		}},
		{"ctf_start", map[string]any{"ctf_start": newStart}, func(t *testing.T, got, _ Settings) {
			if !got.CTFStart.Equal(newStart) {
				t.Errorf("start = %v, want %v", got.CTFStart, newStart)
			}
		}},
		{"ctf_end", map[string]any{"ctf_end": newEnd}, func(t *testing.T, got, _ Settings) {
			if !got.CTFEnd.Equal(newEnd) {
				t.Errorf("end = %v, want %v", got.CTFEnd, newEnd)
			}
		}},
		{"registration", map[string]any{"registration": false}, func(t *testing.T, got, _ Settings) {
			if got.Registration {
				t.Error("registration is still open")
			}
		}},
		{"max_team_size", map[string]any{"max_team_size": 8}, func(t *testing.T, got, _ Settings) {
			if got.MaxTeamSize != 8 {
				t.Errorf("max team size = %d", got.MaxTeamSize)
			}
		}},
		{"theme", map[string]any{"theme": "betauia"}, func(t *testing.T, got, _ Settings) {
			if got.Theme != "betauia" {
				t.Errorf("theme = %q", got.Theme)
			}
		}},
		{"rules", map[string]any{"rules": "# No attacking the platform"}, func(t *testing.T, got, _ Settings) {
			if got.Rules != "# No attacking the platform" {
				t.Errorf("rules = %q", got.Rules)
			}
		}},
		{"nothing at all", map[string]any{}, func(*testing.T, Settings, Settings) {}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseline(t)
			e := newEnv(t)
			before := current(t, e)

			got := put(t, e, admin, tc.body)

			tc.check(t, got, before)
			// Everything the request didn't mention is as it was, in the response and afterwards.
			for label, s := range map[string]Settings{"response": got, "next read": current(t, e)} {
				if _, ok := tc.body["ctf_name"]; !ok && s.CTFName != before.CTFName {
					t.Errorf("%s: name changed to %q", label, s.CTFName)
				}
				if _, ok := tc.body["registration"]; !ok && s.Registration != before.Registration {
					t.Errorf("%s: registration changed to %v", label, s.Registration)
				}
				if _, ok := tc.body["max_team_size"]; !ok && s.MaxTeamSize != before.MaxTeamSize {
					t.Errorf("%s: max team size changed to %d", label, s.MaxTeamSize)
				}
				if _, ok := tc.body["ctf_start"]; !ok && !s.CTFStart.Equal(*before.CTFStart) {
					t.Errorf("%s: start changed to %v", label, s.CTFStart)
				}
				if _, ok := tc.body["ctf_end"]; !ok && !s.CTFEnd.Equal(*before.CTFEnd) {
					t.Errorf("%s: end changed to %v", label, s.CTFEnd)
				}
				if _, ok := tc.body["theme"]; !ok && s.Theme != before.Theme {
					t.Errorf("%s: theme changed to %q", label, s.Theme)
				}
				if _, ok := tc.body["rules"]; !ok && s.Rules != before.Rules {
					t.Errorf("%s: rules changed to %q", label, s.Rules)
				}
			}
		})
	}
}

func TestUpdatingSeveralFieldsAtOnce(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	start := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)
	end := time.Now().Add(6 * time.Hour).UTC().Truncate(time.Second)

	got := put(t, e, testdb.NewAdmin(t), map[string]any{
		"ctf_name": "Everything CTF", "ctf_start": start, "ctf_end": end,
		"registration": false, "max_team_size": 3, "theme": "betauia", "rules": "be nice",
	})

	if got.CTFName != "Everything CTF" || got.Registration || got.MaxTeamSize != 3 || got.Theme != "betauia" || got.Rules != "be nice" ||
		!got.CTFStart.Equal(start) || !got.CTFEnd.Equal(end) {
		t.Errorf("settings = %+v", got)
	}
	if again := current(t, e); again.CTFName != got.CTFName || again.MaxTeamSize != 3 || again.Theme != "betauia" || again.Rules != "be nice" {
		t.Errorf("the next read shows %+v, want the update", again)
	}
}

func TestChangingTheWindowChangesTheCTFPhase(t *testing.T) {
	e := newEnv(t)
	baseline(t) // running
	admin := testdb.NewAdmin(t)
	phase := func() string {
		return testdb.QueryRow[string](t, `SELECT CASE WHEN now() < ctf_start THEN 'not_started' WHEN now() >= ctf_end THEN 'ended' ELSE 'running' END FROM settings`)
	}

	put(t, e, admin, map[string]any{"ctf_start": time.Now().Add(time.Hour), "ctf_end": time.Now().Add(2 * time.Hour)})
	if got := phase(); got != "not_started" {
		t.Errorf("phase = %q, want not_started", got)
	}
	put(t, e, admin, map[string]any{"ctf_start": time.Now().Add(-2 * time.Hour), "ctf_end": time.Now().Add(-time.Hour)})
	if got := phase(); got != "ended" {
		t.Errorf("phase = %q, want ended", got)
	}
}

// updated_at is part of the response so clients can tell when settings last changed. The update
// query doesn't set it, so it stays at its original value. This test is expected to fail until it does.
func TestUpdatingSettingsMovesUpdatedAt(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	testdb.Exec(t, `UPDATE settings SET updated_at = now() - interval '1 hour'`)
	before := current(t, e).UpdatedAt

	got := put(t, e, testdb.NewAdmin(t), map[string]any{"ctf_name": "Touched CTF"})

	if !got.UpdatedAt.After(before) {
		t.Errorf("updated_at = %v after the update, was %v: it should move forward", got.UpdatedAt, before)
	}
}

// --- validation ---

func TestInvalidUpdatesAreRefusedAndChangeNothing(t *testing.T) {
	e := newEnv(t)
	start, end := baseline(t)
	admin := testdb.NewAdmin(t)
	long := strings.Repeat("n", 65)

	tests := []struct {
		name   string
		body   any
		status int
		detail string
	}{
		{"an empty name", map[string]any{"ctf_name": ""}, 400, "CTF name must be 1-64 characters"},
		{"a blank name", map[string]any{"ctf_name": "   \t"}, 400, "CTF name must be 1-64 characters"},
		{"a name over 64 characters", map[string]any{"ctf_name": long}, 400, "CTF name must be 1-64 characters"},
		{"an unknown theme", map[string]any{"theme": "neon"}, 400, "Unknown theme"},
		{"a team size of zero", map[string]any{"max_team_size": 0}, 400, "Max team size must be at least 1"},
		{"a negative team size", map[string]any{"max_team_size": -3}, 400, "Max team size must be at least 1"},
		{"a start after the end", map[string]any{"ctf_start": end.Add(time.Hour), "ctf_end": end}, 400, "CTF start must be before CTF end"},
		{"a start equal to the end", map[string]any{"ctf_start": end, "ctf_end": end}, 400, "CTF start must be before CTF end"},
		{"only a start after the stored end", map[string]any{"ctf_start": end.Add(time.Minute)}, 400, "CTF start must be before CTF end"},
		{"only an end before the stored start", map[string]any{"ctf_end": start.Add(-time.Minute)}, 400, "CTF start must be before CTF end"},
		{"an end equal to the stored start", map[string]any{"ctf_end": start}, 400, "CTF start must be before CTF end"},
		{"a name that is not text", map[string]any{"ctf_name": 5}, 422, "Invalid request body"},
		{"a team size that is not a number", map[string]any{"max_team_size": "many"}, 422, "Invalid request body"},
		{"a date that is not a date", map[string]any{"ctf_start": "tomorrow"}, 422, "Invalid request body"},
		{"a body that is not JSON", nil, 422, "Invalid request body"},
		// Valid fields in the same request must not be half applied.
		{"a valid name next to a bad team size", map[string]any{"ctf_name": "Half CTF", "max_team_size": 0}, 400, "Max team size must be at least 1"},
		{"a valid theme and rules next to a bad window", map[string]any{"theme": "betauia", "rules": "half", "ctf_start": end.Add(time.Hour)}, 400, "CTF start must be before CTF end"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := current(t, e)

			testenv.Expect(t, e.Do(t, admin, "PUT", "/api/admin/settings", tc.body), tc.status, tc.detail)

			after := current(t, e)
			if after.CTFName != before.CTFName || after.MaxTeamSize != before.MaxTeamSize || after.Theme != before.Theme ||
				after.Rules != before.Rules || !after.CTFStart.Equal(*before.CTFStart) || !after.CTFEnd.Equal(*before.CTFEnd) {
				t.Errorf("settings changed from %+v to %+v", before, after)
			}
		})
	}
}

func TestNameIsTrimmedAndSixtyFourCharactersAreAllowed(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	admin := testdb.NewAdmin(t)

	if got := put(t, e, admin, map[string]any{"ctf_name": "  Padded CTF \n"}); got.CTFName != "Padded CTF" {
		t.Errorf("name = %q, want it trimmed", got.CTFName)
	}
	max := strings.Repeat("n", 64)
	if got := put(t, e, admin, map[string]any{"ctf_name": max}); got.CTFName != max {
		t.Errorf("a 64 character name was stored as %q", got.CTFName)
	}
	if got := put(t, e, admin, map[string]any{"max_team_size": 1}); got.MaxTeamSize != 1 {
		t.Errorf("a team size of 1 was stored as %d", got.MaxTeamSize)
	}
}

func TestEveryThemeIsAccepted(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	admin := testdb.NewAdmin(t)

	for _, theme := range Themes {
		if got := put(t, e, admin, map[string]any{"theme": theme}); got.Theme != theme {
			t.Errorf("theme = %q, want %q", got.Theme, theme)
		}
	}
}

// --- theme and rules live in memory ---

func TestThemeAndRulesPersistAcrossRequestsButNotAcrossHandlers(t *testing.T) {
	e := newEnv(t)
	baseline(t)
	put(t, e, testdb.NewAdmin(t), map[string]any{"theme": "betauia", "rules": "# Rules"})

	got := current(t, e)
	if got.Theme != "betauia" || got.Rules != "# Rules" {
		t.Errorf("a later request shows theme %q rules %q, want the update", got.Theme, got.Rules)
	}
	if fresh := current(t, newEnv(t)); fresh.Theme != Themes[0] || fresh.Rules != "" {
		t.Errorf("a new handler shows theme %q rules %q: they must not be stored in the database", fresh.Theme, fresh.Rules)
	}
}

func TestTheThemeEnvironmentVariableSetsTheStartingTheme(t *testing.T) {
	baseline(t)
	tests := map[string]string{
		"a known theme":               Themes[len(Themes)-1],
		"an unknown theme":            Themes[0],
		"an empty value":              Themes[0],
		"a theme with different case": Themes[0],
	}
	values := map[string]string{
		"a known theme":               Themes[len(Themes)-1],
		"an unknown theme":            "neon",
		"an empty value":              "",
		"a theme with different case": strings.ToUpper(Themes[len(Themes)-1]),
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("THEME", values[name])

			if got := current(t, newEnv(t)).Theme; got != want {
				t.Errorf("THEME=%q starts as %q, want %q", values[name], got, want)
			}
		})
	}
}

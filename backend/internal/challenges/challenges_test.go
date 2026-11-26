package challenges

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
	"github.com/betauia/openctf/backend/internal/testutil/testenv"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

type published struct {
	event string
	data  any
}

// harness is the routes under test with a temporary files directory and a recorder for the
// events the handler publishes.
type harness struct {
	*testenv.Env
	dir string // the files directory; its parent is for files that must never be served

	mu     sync.Mutex
	events []published
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{dir: filepath.Join(t.TempDir(), "files")}
	if err := os.Mkdir(h.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	h.Env = testenv.New(t, func(mux *http.ServeMux, a *auth.Auth) {
		New(testdb.Pool, a, h.dir, h.publish).Register(mux)
	})
	return h
}

func (h *harness) publish(event string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, published{event, data})
}

func (h *harness) solveEvents(t *testing.T) []solveEvent {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []solveEvent
	for _, e := range h.events {
		if e.event != "solve" {
			t.Errorf("unexpected event %q", e.event)
			continue
		}
		out = append(out, e.data.(solveEvent))
	}
	return out
}

// newPlayer returns a user who is the captain of a new team.
func newPlayer(t *testing.T) (*testdb.User, uuid.UUID) {
	t.Helper()
	u := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, u)
	return u, team
}

func hideTeam(t *testing.T, team uuid.UUID) {
	t.Helper()
	testdb.Exec(t, `UPDATE teams SET hidden = true WHERE id = $1`, team)
}

func addSolveRow(t *testing.T, u *testdb.User, team, challenge uuid.UUID, ago time.Duration) {
	t.Helper()
	testdb.Exec(t, `INSERT INTO solves (user_id, team_id, challenge_id, solved_at) VALUES ($1, $2, $3, now() - make_interval(secs => $4))`,
		u.ID, team, challenge, ago.Seconds())
}

func score(t *testing.T, u *testdb.User) int {
	return testdb.QueryRow[int](t, `SELECT score FROM users WHERE id = $1`, u.ID)
}

func solveCount(t *testing.T, challenge uuid.UUID) int {
	return testdb.QueryRow[int](t, `SELECT solves FROM challenges WHERE id = $1`, challenge)
}

func solveRows(t *testing.T, challenge uuid.UUID) int {
	return testdb.QueryRow[int](t, `SELECT count(*) FROM solves WHERE challenge_id = $1`, challenge)
}

type listed struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Points    int       `json:"points"`
	Category  string    `json:"category"`
	Solves    int       `json:"solves"`
	IsVisible bool      `json:"is_visible"`
}

func idsOf(list []listed) []uuid.UUID {
	out := make([]uuid.UUID, len(list))
	for i, c := range list {
		out[i] = c.ID
	}
	return out
}

// --- files on disk ---

func (h *harness) writeFile(t *testing.T, rel, content string) {
	t.Helper()
	path := filepath.Join(h.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addFileRow inserts a challenge_files row without touching the disk.
func addFileRow(t *testing.T, challenge uuid.UUID, filename, filepath string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	testdb.Exec(t, `INSERT INTO challenge_files (id, challenge_id, filename, filepath) VALUES ($1, $2, $3, $4)`, id, challenge, filename, filepath)
	return id
}

// addFile writes a file under the files directory and records it for the challenge.
func (h *harness) addFile(t *testing.T, challenge uuid.UUID, filename, content string) uuid.UUID {
	t.Helper()
	rel := challenge.String() + "/" + filename
	h.writeFile(t, rel, content)
	return addFileRow(t, challenge, filename, rel)
}

// --- reading challenges ---

func TestListShowsOnlyVisibleChallengesAndNeverFlags(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	admin := testdb.NewAdmin(t)
	cat := "cat" + testdb.Unique()
	later := testdb.AddChallenge(t, testdb.Challenge{Title: "later", Category: cat + "-b", Points: 300, Flags: []string{"flag{shown}"}})
	earlier := testdb.AddChallenge(t, testdb.Challenge{Title: "earlier", Category: cat + "-a"})
	hidden := testdb.AddChallenge(t, testdb.Challenge{Title: "hidden", Category: cat + "-c", Hidden: true})

	rec := h.Get(t, p, "/api/challenges")

	var mine []uuid.UUID
	for _, c := range testenv.Decode[[]listed](t, rec) {
		if strings.HasPrefix(c.Category, cat) {
			mine = append(mine, c.ID)
		}
	}
	if want := []uuid.UUID{earlier, later}; !slices.Equal(mine, want) {
		t.Errorf("challenges = %v, want %v: visible only, sorted by category", mine, want)
	}
	if strings.Contains(rec.Body.String(), "flag{shown}") || strings.Contains(rec.Body.String(), `"flags"`) {
		t.Errorf("the list leaks flags: %s", rec.Body)
	}
	// The player route never lists hidden challenges, not even to an admin; the admin route does.
	if slices.Contains(idsOf(testenv.Decode[[]listed](t, h.Get(t, admin, "/api/challenges"))), hidden) {
		t.Error("an admin sees a hidden challenge in the player list")
	}
}

func TestListRequiresLogin(t *testing.T) {
	h := newHarness(t)

	testenv.Expect(t, h.Get(t, nil, "/api/challenges"), http.StatusUnauthorized, "Not authenticated")
}

func TestGetChallenge(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	admin := testdb.NewAdmin(t)
	shown := testdb.AddChallenge(t, testdb.Challenge{Title: "shown", Points: 250, Flags: []string{"flag{one}"}})
	hidden := testdb.AddChallenge(t, testdb.Challenge{Title: "hidden", Hidden: true})
	path := func(id uuid.UUID) string { return "/api/challenges/" + id.String() }

	t.Run("visible challenge", func(t *testing.T) {
		rec := h.Get(t, p, path(shown))

		got := testenv.Decode[listed](t, rec)
		if got.ID != shown || got.Title != "shown" || got.Points != 250 || !got.IsVisible {
			t.Errorf("challenge = %+v", got)
		}
		if strings.Contains(rec.Body.String(), "flag{one}") {
			t.Errorf("the challenge leaks a flag: %s", rec.Body)
		}
	})
	t.Run("hidden challenge is not found for a player", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, p, path(hidden)), http.StatusNotFound, "Challenge not found")
	})
	t.Run("an admin can read a hidden challenge", func(t *testing.T) {
		got := testenv.Decode[listed](t, h.Get(t, admin, path(hidden)))
		if got.ID != hidden || got.IsVisible {
			t.Errorf("challenge = %+v, want the hidden one", got)
		}
	})
	t.Run("logged out", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, nil, path(shown)), http.StatusUnauthorized, "Not authenticated")
		testenv.Expect(t, h.Get(t, nil, path(hidden)), http.StatusUnauthorized, "Not authenticated")
	})
	t.Run("unknown and malformed ids", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, p, path(uuid.NewV7())), http.StatusNotFound, "Challenge not found")
		testenv.Expect(t, h.Get(t, p, "/api/challenges/not-a-uuid"), http.StatusUnprocessableEntity, "Invalid id")
	})
}

func TestPlayersCannotReadChallengesBeforeTheCTFStarts(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	admin := testdb.NewAdmin(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	list, one := "/api/challenges", "/api/challenges/"+c.String()
	testdb.DelayCTF(t)

	testenv.Expect(t, h.Get(t, p, list), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, h.Get(t, p, one), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, h.Get(t, p, one+"/files"), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, h.Get(t, p, one+"/solves"), http.StatusForbidden, "Not allowed")
	// Admins and maintainers can prepare the challenges before it starts.
	testenv.Expect(t, h.Get(t, admin, one), http.StatusOK, "")
}

func TestPlayersCanStillReadChallengesAfterTheCTFEnds(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	testdb.EndCTF(t)

	testenv.Expect(t, h.Get(t, p, "/api/challenges"), http.StatusOK, "")
	testenv.Expect(t, h.Get(t, p, "/api/challenges/"+c.String()), http.StatusOK, "")
}

type solverRow struct {
	UserID   uuid.UUID  `json:"uid"`
	Username string     `json:"username"`
	TeamID   *uuid.UUID `json:"team_id"`
	Team     *string    `json:"team"`
}

func TestViewSolvesListsSolversOldestFirstAndHidesHiddenTeams(t *testing.T) {
	h := newHarness(t)
	viewer, _ := newPlayer(t)
	first, firstTeam := newPlayer(t)
	second, secondTeam := newPlayer(t)
	third, thirdTeam := newPlayer(t)
	hideTeam(t, secondTeam)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	addSolveRow(t, third, thirdTeam, c, time.Hour)
	addSolveRow(t, first, firstTeam, c, 3*time.Hour)
	addSolveRow(t, second, secondTeam, c, 2*time.Hour)

	got := testenv.Decode[[]solverRow](t, h.Get(t, viewer, "/api/challenges/"+c.String()+"/solves"))

	if len(got) != 3 {
		t.Fatalf("got %d solvers %+v, want 3", len(got), got)
	}
	for i, want := range []*testdb.User{first, second, third} {
		if got[i].UserID != want.ID || got[i].Username != want.Name {
			t.Errorf("solver %d = %v %q, want %v %q", i+1, got[i].UserID, got[i].Username, want.ID, want.Name)
		}
	}
	if got[0].TeamID == nil || *got[0].TeamID != firstTeam || got[0].Team == nil {
		t.Errorf("first solver's team = %v %v, want %v", got[0].TeamID, got[0].Team, firstTeam)
	}
	if got[1].TeamID != nil || got[1].Team != nil {
		t.Errorf("a hidden team is shown: %v %v", got[1].TeamID, got[1].Team)
	}
}

func TestViewSolvesRules(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	shown := testdb.AddChallenge(t, testdb.Challenge{})
	hidden := testdb.AddChallenge(t, testdb.Challenge{Hidden: true})

	got := testenv.Decode[[]solverRow](t, h.Get(t, p, "/api/challenges/"+shown.String()+"/solves"))
	if got == nil || len(got) != 0 {
		t.Errorf("a challenge without solves lists %v, want an empty list", got)
	}
	testenv.Expect(t, h.Get(t, p, "/api/challenges/"+hidden.String()+"/solves"), http.StatusNotFound, "Challenge not found")
	testenv.Expect(t, h.Get(t, nil, "/api/challenges/"+shown.String()+"/solves"), http.StatusUnauthorized, "Not authenticated")
}

// --- submitting flags ---

type result struct {
	Correct     bool       `json:"correct"`
	ChallengeID *uuid.UUID `json:"challenge_id"`
	Points      int        `json:"points"`
}

func (h *harness) submit(t *testing.T, u *testdb.User, flag string, challenge *uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]any{"flag": flag}
	if challenge != nil {
		body["challenge_id"] = *challenge
	}
	return h.Do(t, u, "POST", "/api/challenges/submit", body)
}

func TestSubmitRightWrongAndHiddenFlags(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	right := "flag{" + testdb.Unique() + "}"
	other := "flag{" + testdb.Unique() + "}"
	c := testdb.AddChallenge(t, testdb.Challenge{Points: 150, Flags: []string{right, "flag{second}" + testdb.Unique()}})
	otherChallenge := testdb.AddChallenge(t, testdb.Challenge{Flags: []string{other}})
	hiddenFlag := "flag{" + testdb.Unique() + "}"
	testdb.AddChallenge(t, testdb.Challenge{Hidden: true, Flags: []string{hiddenFlag}})
	wrong := result{}

	tests := []struct {
		name      string
		flag      string
		challenge *uuid.UUID
		want      result
	}{
		{"wrong flag", "flag{nope}", nil, wrong},
		{"flags are case sensitive", strings.ToUpper(right), nil, wrong},
		{"a flag of a hidden challenge counts as wrong", hiddenFlag, nil, wrong},
		{"a flag of another challenge, pinned to this one", other, &c, wrong},
		{"the right flag pinned to another challenge", right, &otherChallenge, wrong},
		// A solve changes state, so each correct case below uses a fresh player.
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := h.submit(t, p, tc.flag, tc.challenge)

			testenv.Expect(t, rec, http.StatusOK, "")
			if got := testenv.Decode[result](t, rec); got != tc.want {
				t.Errorf("result = %+v, want %+v", got, tc.want)
			}
		})
	}

	correct := []struct {
		name      string
		flag      string
		challenge *uuid.UUID
	}{
		{"the right flag", right, nil},
		{"the right flag pinned to its challenge", right, &c},
		{"surrounding whitespace is ignored", "  \t" + right + "\n ", nil},
	}
	for _, tc := range correct {
		t.Run(tc.name, func(t *testing.T) {
			solver, _ := newPlayer(t)

			got := testenv.Decode[result](t, h.submit(t, solver, tc.flag, tc.challenge))

			if !got.Correct || got.Points != 150 || got.ChallengeID == nil || *got.ChallengeID != c {
				t.Errorf("result = %+v, want correct with 150 points for %v", got, c)
			}
		})
	}
	t.Run("either of several flags works", func(t *testing.T) {
		solver, _ := newPlayer(t)
		second := testdb.QueryRow[string](t, `SELECT flags->>1 FROM challenges WHERE id = $1`, c)

		if got := testenv.Decode[result](t, h.submit(t, solver, second, nil)); !got.Correct {
			t.Errorf("result = %+v, want correct", got)
		}
	})
}

func TestSubmitRejectsAnEmptyFlag(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)

	for _, flag := range []string{"", "   ", "\n\t"} {
		testenv.Expect(t, h.submit(t, p, flag, nil), http.StatusBadRequest, "Flag is required")
	}
}

func TestFirstSolveScoresAndPublishesAnEvent(t *testing.T) {
	h := newHarness(t)
	p, team := newPlayer(t)
	teammate := testdb.NewUser(t)
	testdb.Exec(t, `UPDATE users SET team_id = $1 WHERE id = $2`, team, teammate.ID)
	flag := "flag{" + testdb.Unique() + "}"
	c := testdb.AddChallenge(t, testdb.Challenge{Title: "scored", Category: "web", Points: 400, Flags: []string{flag}})

	got := testenv.Decode[result](t, h.submit(t, p, flag, nil))

	if !got.Correct || got.Points != 400 {
		t.Fatalf("result = %+v, want correct with 400 points", got)
	}
	if score(t, p) != 400 {
		t.Errorf("solver's score = %d, want 400", score(t, p))
	}
	if score(t, teammate) != 0 {
		t.Errorf("teammate's score = %d, want it unchanged at 0", score(t, teammate))
	}
	if solveCount(t, c) != 1 || solveRows(t, c) != 1 {
		t.Errorf("solve count %d, solve rows %d; want 1 and 1", solveCount(t, c), solveRows(t, c))
	}
	if gotTeam := testdb.QueryRow[uuid.UUID](t, `SELECT team_id FROM solves WHERE challenge_id = $1`, c); gotTeam != team {
		t.Errorf("the solve belongs to team %v, want %v", gotTeam, team)
	}
	events := h.solveEvents(t)
	if len(events) != 1 {
		t.Fatalf("published %d events, want 1", len(events))
	}
	e := events[0]
	if e.ChallengeID != c || e.Challenge != "scored" || e.Category != "web" || e.Points != 400 || e.UserID != p.ID || e.Username != p.Name {
		t.Errorf("event = %+v", e)
	}
	if e.TeamID == nil || *e.TeamID != team || e.Team == nil || *e.Team == "" {
		t.Errorf("event team = %v %v, want %v", e.TeamID, e.Team, team)
	}
}

func TestSolveEventOmitsAHiddenTeam(t *testing.T) {
	h := newHarness(t)
	p, team := newPlayer(t)
	hideTeam(t, team)
	flag := "flag{" + testdb.Unique() + "}"
	testdb.AddChallenge(t, testdb.Challenge{Flags: []string{flag}})

	testenv.Expect(t, h.submit(t, p, flag, nil), http.StatusOK, "")

	events := h.solveEvents(t)
	if len(events) != 1 || events[0].TeamID != nil || events[0].Team != nil {
		t.Errorf("events = %+v, want one without a team", events)
	}
}

func TestOtherTeamsScoreTheSameChallengeSeparately(t *testing.T) {
	h := newHarness(t)
	a, _ := newPlayer(t)
	b, _ := newPlayer(t)
	flag := "flag{" + testdb.Unique() + "}"
	c := testdb.AddChallenge(t, testdb.Challenge{Points: 200, Flags: []string{flag}})

	testenv.Expect(t, h.submit(t, a, flag, nil), http.StatusOK, "")
	got := testenv.Decode[result](t, h.submit(t, b, flag, nil))

	if got.Points != 200 || score(t, a) != 200 || score(t, b) != 200 {
		t.Errorf("points %d, scores %d and %d; want 200 each", got.Points, score(t, a), score(t, b))
	}
	if solveCount(t, c) != 2 || len(h.solveEvents(t)) != 2 {
		t.Errorf("solve count %d, events %d; want 2 and 2", solveCount(t, c), len(h.solveEvents(t)))
	}
}

// A challenge solved by a team can't score again for that team, whoever on it submits the flag.
func TestResolvingDoesNotScoreAgain(t *testing.T) {
	h := newHarness(t)
	solver, team := newPlayer(t)
	teammate := testdb.NewUser(t)
	testdb.Exec(t, `UPDATE users SET team_id = $1 WHERE id = $2`, team, teammate.ID)
	flag := "flag{" + testdb.Unique() + "}"
	c := testdb.AddChallenge(t, testdb.Challenge{Points: 300, Flags: []string{flag}})
	testenv.Expect(t, h.submit(t, solver, flag, nil), http.StatusOK, "")

	for name, who := range map[string]*testdb.User{"the same user": solver, "a teammate": teammate} {
		t.Run(name, func(t *testing.T) {
			got := testenv.Decode[result](t, h.submit(t, who, flag, nil))

			if got.Points != 0 {
				t.Errorf("points = %d, want 0 for a challenge the team already solved", got.Points)
			}
		})
	}
	if score(t, solver) != 300 || score(t, teammate) != 0 {
		t.Errorf("scores %d and %d, want 300 and 0", score(t, solver), score(t, teammate))
	}
	if solveCount(t, c) != 1 || solveRows(t, c) != 1 || len(h.solveEvents(t)) != 1 {
		t.Errorf("solve count %d, rows %d, events %d; want 1 each", solveCount(t, c), solveRows(t, c), len(h.solveEvents(t)))
	}
}

// Submitting the flag of a challenge the team has already solved should say so. The handler doesn't
// yet: it answers like a first solve, with no points and no message. This test is expected to fail
// until it does.
func TestResolvingSaysTheChallengeWasAlreadySolved(t *testing.T) {
	h := newHarness(t)
	solver, team := newPlayer(t)
	teammate := testdb.NewUser(t)
	testdb.Exec(t, `UPDATE users SET team_id = $1 WHERE id = $2`, team, teammate.ID)
	flag := "flag{" + testdb.Unique() + "}"
	testdb.AddChallenge(t, testdb.Challenge{Flags: []string{flag}})
	testenv.Expect(t, h.submit(t, solver, flag, nil), http.StatusOK, "")

	for name, who := range map[string]*testdb.User{"the same user": solver, "a teammate": teammate} {
		t.Run(name, func(t *testing.T) {
			rec := h.submit(t, who, flag, nil)

			if body := strings.ToLower(rec.Body.String()); !strings.Contains(body, "already solved") {
				t.Errorf("response %s does not say the challenge has already been solved", rec.Body)
			}
		})
	}
}

func TestSubmitAccessGates(t *testing.T) {
	h := newHarness(t)
	flag := "flag{" + testdb.Unique() + "}"
	c := testdb.AddChallenge(t, testdb.Challenge{Flags: []string{flag}})

	tests := []struct {
		name   string
		status int
		detail string
		setup  func(t *testing.T) *testdb.User
	}{
		{"logged out", http.StatusUnauthorized, "Not authenticated",
			func(*testing.T) *testdb.User { return nil }},
		{"a user without a team", http.StatusForbidden, "Not allowed",
			func(t *testing.T) *testdb.User { return testdb.NewUser(t) }},
		{"before the CTF starts", http.StatusForbidden, "Not allowed",
			func(t *testing.T) *testdb.User { p, _ := newPlayer(t); testdb.DelayCTF(t); return p }},
		{"after the CTF ends", http.StatusForbidden, "Not allowed",
			func(t *testing.T) *testdb.User { p, _ := newPlayer(t); testdb.EndCTF(t); return p }},
		{"a banned team", http.StatusForbidden, "Not allowed",
			func(t *testing.T) *testdb.User {
				p, team := newPlayer(t)
				testdb.Exec(t, `UPDATE teams SET banned = true WHERE id = $1`, team)
				return p
			}},
		{"a user banned while logged in", http.StatusForbidden, "Not allowed",
			func(t *testing.T) *testdb.User {
				p, _ := newPlayer(t)
				h.Login(t, p)
				p.Ban(t)
				return p
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := tc.setup(t)

			// The right flag, so a refusal can only come from the gate.
			testenv.Expect(t, h.submit(t, u, flag, nil), tc.status, tc.detail)

			if solveRows(t, c) != 0 || solveCount(t, c) != 0 {
				t.Errorf("a refused submission still recorded a solve")
			}
			if len(h.solveEvents(t)) != 0 {
				t.Error("a refused submission published an event")
			}
		})
	}
}

func TestSubmitIsRateLimitedPerUser(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	bystander, _ := newPlayer(t)
	flag := "flag{" + testdb.Unique() + "}"
	c := testdb.AddChallenge(t, testdb.Challenge{Flags: []string{flag}})

	for i := 1; i <= 10; i++ {
		testenv.Expect(t, h.submit(t, p, "flag{wrong}", nil), http.StatusOK, "")
	}

	// The eleventh in a minute is refused, even with the right flag.
	testenv.Expect(t, h.submit(t, p, flag, nil), http.StatusTooManyRequests, "Too many flag submissions, try again in a minute")
	if solveRows(t, c) != 0 {
		t.Error("a rate limited submission recorded a solve")
	}
	// Another user has their own allowance.
	if got := testenv.Decode[result](t, h.submit(t, bystander, flag, nil)); !got.Correct {
		t.Errorf("another user's submission = %+v, want correct", got)
	}
}

// --- challenge files ---

func TestListFiles(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	zeta := h.addFile(t, c, "zeta.txt", "z")
	alpha := h.addFile(t, c, "alpha.txt", "a")
	empty := testdb.AddChallenge(t, testdb.Challenge{})

	got := testenv.Decode[[]struct {
		ID       uuid.UUID `json:"id"`
		Filename string    `json:"filename"`
	}](t, h.Get(t, p, "/api/challenges/"+c.String()+"/files"))

	if len(got) != 2 || got[0].ID != alpha || got[0].Filename != "alpha.txt" || got[1].ID != zeta {
		t.Errorf("files = %+v, want alpha.txt then zeta.txt", got)
	}
	rec := h.Get(t, p, "/api/challenges/"+empty.String()+"/files")
	testenv.Expect(t, rec, http.StatusOK, "")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("a challenge without files lists %s, want []", rec.Body)
	}
}

func TestDownloadServesTheFileWithSafeHeaders(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	const content = "0123456789"
	f := h.addFile(t, c, "data.bin", content)
	url := "/api/challenges/" + c.String() + "/files/" + f.String()

	rec := h.Get(t, p, url)

	testenv.Expect(t, rec, http.StatusOK, "")
	if rec.Body.String() != content {
		t.Errorf("body = %q, want %q", rec.Body, content)
	}
	disposition, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || params["filename"] != "data.bin" {
		t.Errorf("Content-Disposition = %q (%v), want an attachment named data.bin", rec.Header().Get("Content-Disposition"), err)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestDownloadSupportsRangeRequests(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	f := h.addFile(t, c, "data.bin", "0123456789")

	rec := h.DoWith(t, p, "GET", "/api/challenges/"+c.String()+"/files/"+f.String(), http.Header{"Range": {"bytes=2-5"}})

	testenv.Expect(t, rec, http.StatusPartialContent, "")
	if rec.Body.String() != "2345" {
		t.Errorf("body = %q, want %q", rec.Body, "2345")
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 2-5/10" {
		t.Errorf("Content-Range = %q, want bytes 2-5/10", got)
	}
}

func TestDownloadQuotesAwkwardFilenames(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	const name = `my "quoted" file; é.txt`
	h.writeFile(t, "awkward.txt", "x")
	f := addFileRow(t, c, name, "awkward.txt")

	rec := h.Get(t, p, "/api/challenges/"+c.String()+"/files/"+f.String())

	testenv.Expect(t, rec, http.StatusOK, "")
	_, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	if err != nil || params["filename"] != name {
		t.Errorf("Content-Disposition = %q (%v), want the filename %q to survive", rec.Header().Get("Content-Disposition"), err, name)
	}
}

func TestDownloadNotFoundCases(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	other := testdb.AddChallenge(t, testdb.Challenge{})
	otherFile := h.addFile(t, other, "other.txt", "other")
	missing := addFileRow(t, c, "gone.txt", c.String()+"/gone.txt")
	if err := os.MkdirAll(filepath.Join(h.dir, "a-directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := addFileRow(t, c, "dir.txt", "a-directory")
	base := "/api/challenges/" + c.String() + "/files/"

	tests := []struct {
		name, path, detail string
		status             int
	}{
		{"the file is missing on disk", base + missing.String(), "File not found", http.StatusNotFound},
		{"the path is a directory", base + dir.String(), "File not found", http.StatusNotFound},
		{"the file belongs to another challenge", base + otherFile.String(), "File not found", http.StatusNotFound},
		{"unknown file id", base + uuid.NewV7().String(), "File not found", http.StatusNotFound},
		{"malformed file id", base + "not-a-uuid", "Invalid fid", http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Expect(t, h.Get(t, p, tc.path), tc.status, tc.detail)
		})
	}
}

// A files row is trusted data, but a bad path in it must never serve anything outside the files
// directory. Today such a path ends in a 500; the point of the test is that nothing is served.
func TestDownloadNeverServesFilesOutsideTheFilesDirectory(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{})
	const secret = "TOP SECRET OUTSIDE THE FILES DIRECTORY"
	outside := filepath.Join(filepath.Dir(h.dir), "secret.txt")
	if err := os.WriteFile(outside, []byte(secret), 0o644); err != nil {
		t.Fatal(err)
	}
	h.writeFile(t, "sub/inside.txt", "inside")

	for name, path := range map[string]string{
		"dot-dot":        "../secret.txt",
		"nested dot-dot": "sub/../../secret.txt",
		"absolute path":  outside,
	} {
		t.Run(name, func(t *testing.T) {
			f := addFileRow(t, c, "x.txt", path)

			rec := h.Get(t, p, "/api/challenges/"+c.String()+"/files/"+f.String())

			if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), secret) {
				t.Errorf("status %d, body %q: a file outside the files directory was served", rec.Code, rec.Body)
			}
		})
	}
}

func TestFilesOfAHiddenChallengeAreNotFoundForPlayers(t *testing.T) {
	h := newHarness(t)
	p, _ := newPlayer(t)
	c := testdb.AddChallenge(t, testdb.Challenge{Hidden: true})
	f := h.addFile(t, c, "secret.txt", "secret")
	base := "/api/challenges/" + c.String() + "/files"

	testenv.Expect(t, h.Get(t, p, base), http.StatusNotFound, "Challenge not found")
	testenv.Expect(t, h.Get(t, p, base+"/"+f.String()), http.StatusNotFound, "Challenge not found")
	testenv.Expect(t, h.Get(t, nil, base), http.StatusUnauthorized, "Not authenticated")
}

// --- admin routes ---

func TestAdminListShowsHiddenChallengesAndMaintainersOnlyTheirOwn(t *testing.T) {
	h := newHarness(t)
	admin, maintainer, other := testdb.NewAdmin(t), testdb.NewMaintainer(t), testdb.NewMaintainer(t)
	player, _ := newPlayer(t)
	mineShown := testdb.AddChallenge(t, testdb.Challenge{Owner: maintainer})
	mineHidden := testdb.AddChallenge(t, testdb.Challenge{Owner: maintainer, Hidden: true})
	theirShown := testdb.AddChallenge(t, testdb.Challenge{Owner: other})
	theirHidden := testdb.AddChallenge(t, testdb.Challenge{Owner: other, Hidden: true})
	all := []uuid.UUID{mineShown, mineHidden, theirShown, theirHidden}
	only := func(u *testdb.User) []uuid.UUID {
		var got []uuid.UUID
		for _, id := range idsOf(testenv.Decode[[]listed](t, h.Get(t, u, "/api/admin/challenges"))) {
			if slices.Contains(all, id) {
				got = append(got, id)
			}
		}
		slices.SortFunc(got, uuid.UUID.Compare)
		return got
	}
	sorted := func(ids ...uuid.UUID) []uuid.UUID { slices.SortFunc(ids, uuid.UUID.Compare); return ids }

	if got := only(admin); !slices.Equal(got, sorted(all...)) {
		t.Errorf("admin sees %v, want all four", got)
	}
	if got := only(maintainer); !slices.Equal(got, sorted(mineShown, mineHidden)) {
		t.Errorf("maintainer sees %v, want only their own two", got)
	}
	testenv.Expect(t, h.Get(t, player, "/api/admin/challenges"), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, h.Get(t, nil, "/api/admin/challenges"), http.StatusUnauthorized, "Not authenticated")
}

func TestAdminGetChallenge(t *testing.T) {
	h := newHarness(t)
	admin, maintainer, other := testdb.NewAdmin(t), testdb.NewMaintainer(t), testdb.NewMaintainer(t)
	player, _ := newPlayer(t)
	mine := testdb.AddChallenge(t, testdb.Challenge{Owner: maintainer, Hidden: true})
	theirShown := testdb.AddChallenge(t, testdb.Challenge{Owner: other})
	theirHidden := testdb.AddChallenge(t, testdb.Challenge{Owner: other, Hidden: true})
	path := func(id uuid.UUID) string { return "/api/admin/challenges/" + id.String() }

	tests := []struct {
		name   string
		who    *testdb.User
		id     uuid.UUID
		status int
		detail string
	}{
		{"admin reads a hidden challenge", admin, theirHidden, http.StatusOK, ""},
		{"maintainer reads their own hidden challenge", maintainer, mine, http.StatusOK, ""},
		{"maintainer can't read another's visible challenge", maintainer, theirShown, http.StatusForbidden, "Not allowed"},
		// A hidden challenge doesn't reveal that it exists to someone who may not read it.
		{"maintainer can't see another's hidden challenge", maintainer, theirHidden, http.StatusNotFound, "Challenge not found"},
		{"player on a visible challenge", player, theirShown, http.StatusForbidden, "Not allowed"},
		// The route itself is closed to players, so they get the same answer whatever the challenge is.
		{"player on a hidden challenge", player, theirHidden, http.StatusForbidden, "Not allowed"},
		{"logged out", nil, theirShown, http.StatusUnauthorized, "Not authenticated"},
		{"unknown id", admin, uuid.NewV7(), http.StatusNotFound, "Challenge not found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Expect(t, h.Get(t, tc.who, path(tc.id)), tc.status, tc.detail)
		})
	}
	testenv.Expect(t, h.Get(t, admin, "/api/admin/challenges/not-a-uuid"), http.StatusUnprocessableEntity, "Invalid id")
}

func TestAdminFiles(t *testing.T) {
	h := newHarness(t)
	admin, maintainer, other := testdb.NewAdmin(t), testdb.NewMaintainer(t), testdb.NewMaintainer(t)
	player, _ := newPlayer(t)
	mine := testdb.AddChallenge(t, testdb.Challenge{Owner: maintainer, Hidden: true})
	theirs := testdb.AddChallenge(t, testdb.Challenge{Owner: other})
	myFile := h.addFile(t, mine, "mine.txt", "mine")
	theirFile := h.addFile(t, theirs, "theirs.txt", "theirs")
	list := func(c uuid.UUID) string { return "/api/admin/challenges/" + c.String() + "/files" }

	t.Run("an admin lists and downloads files of a hidden challenge", func(t *testing.T) {
		files := testenv.Decode[[]struct {
			ID uuid.UUID `json:"id"`
		}](t, h.Get(t, admin, list(mine)))
		if len(files) != 1 || files[0].ID != myFile {
			t.Errorf("files = %+v, want %v", files, myFile)
		}
		rec := h.Get(t, admin, list(mine)+"/"+myFile.String())
		testenv.Expect(t, rec, http.StatusOK, "")
		if rec.Body.String() != "mine" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("body %q, nosniff %q", rec.Body, rec.Header().Get("X-Content-Type-Options"))
		}
	})
	t.Run("a maintainer manages the files of their own challenge", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, maintainer, list(mine)), http.StatusOK, "")
		testenv.Expect(t, h.Get(t, maintainer, list(mine)+"/"+myFile.String()), http.StatusOK, "")
	})
	t.Run("a maintainer can't touch another's files", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, maintainer, list(theirs)), http.StatusForbidden, "Not allowed")
		testenv.Expect(t, h.Get(t, maintainer, list(theirs)+"/"+theirFile.String()), http.StatusForbidden, "Not allowed")
	})
	t.Run("players and visitors are refused", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, player, list(theirs)), http.StatusForbidden, "Not allowed")
		testenv.Expect(t, h.Get(t, player, list(mine)), http.StatusForbidden, "Not allowed")
		testenv.Expect(t, h.Get(t, nil, list(theirs)), http.StatusUnauthorized, "Not authenticated")
	})
	t.Run("file ids are checked against the challenge", func(t *testing.T) {
		testenv.Expect(t, h.Get(t, admin, list(mine)+"/"+theirFile.String()), http.StatusNotFound, "File not found")
		testenv.Expect(t, h.Get(t, admin, list(mine)+"/not-a-uuid"), http.StatusUnprocessableEntity, "Invalid fid")
	})
}

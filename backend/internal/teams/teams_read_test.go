package teams

import (
	"context"
	"encoding/json"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
	"github.com/betauia/openctf/backend/internal/testutil/testenv"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"
)

// These tests cover the public team views, the team stats, both scoreboards and user profiles.
//
// The database is shared by every test in the package, and the listings are global, so a test
// that needs to be at the top of a listing gives its teams a score above anything else (the
// bands below) and removes the challenges it created when it ends. That keeps the other tests'
// assumptions about the top of the lists valid.

// addChallenge inserts a challenge and deletes it, with its solves, when the test ends.
func addChallenge(t *testing.T, category, title string, points int, visible bool) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	testdb.Exec(t, `INSERT INTO challenges (id, title, points, category, is_visible) VALUES ($1, $2, $3, $4, $5)`,
		id, title, points, category, visible)
	t.Cleanup(func() { _, _ = testdb.Pool.Exec(context.Background(), `DELETE FROM challenges WHERE id = $1`, id) })
	return id
}

// solveChallenge records that by solved the challenge for team the given time ago.
func solveChallenge(t *testing.T, challenge, team uuid.UUID, by *testdb.User, ago time.Duration) {
	t.Helper()
	testdb.Exec(t, `INSERT INTO solves (user_id, team_id, challenge_id, solved_at) VALUES ($1, $2, $3, now() - make_interval(secs => $4))`,
		by.ID, team, challenge, ago.Seconds())
}

// addSolve creates a visible challenge worth points and has by solve it for team ago ago.
func addSolve(t *testing.T, team uuid.UUID, by *testdb.User, points int, ago time.Duration) uuid.UUID {
	t.Helper()
	c := addChallenge(t, "cat"+testdb.Unique(), "ch"+testdb.Unique(), points, true)
	solveChallenge(t, c, team, by, ago)
	return c
}

// topTeam creates a team whose score is exactly points, so its position in a listing is known.
func topTeam(t *testing.T, points int) (id uuid.UUID, captain *testdb.User) {
	t.Helper()
	captain = testdb.NewUser(t)
	id, _ = testdb.NewTeam(t, captain)
	addSolve(t, id, captain, points, 0)
	return id, captain
}

func renameTeam(t *testing.T, id uuid.UUID, name string) {
	t.Helper()
	testdb.Exec(t, `UPDATE teams SET name = $1 WHERE id = $2`, name, id)
}

func hideTeam(t *testing.T, id uuid.UUID) {
	t.Helper()
	testdb.Exec(t, `UPDATE teams SET hidden = true WHERE id = $1`, id)
}

// setScore sets a user's score and resets it to 0 when the test ends.
func setScore(t *testing.T, p *testdb.User, score int) {
	t.Helper()
	testdb.Exec(t, `UPDATE users SET score = $1 WHERE id = $2`, score, p.ID)
	t.Cleanup(func() {
		_, _ = testdb.Pool.Exec(context.Background(), `UPDATE users SET score = 0 WHERE id = $1`, p.ID)
	})
}

func ids(teams []team) []uuid.UUID {
	out := make([]uuid.UUID, len(teams))
	for i, tm := range teams {
		out[i] = tm.ID
	}
	return out
}

func ranks(teams []team) []int {
	out := make([]int, len(teams))
	for i, tm := range teams {
		out[i] = tm.Rank
	}
	return out
}

// --- public team list ---

func TestTeamListRanksTeamsAndTiesShareARank(t *testing.T) {
	e := newEnv(t)
	a, _ := topTeam(t, 70_000_003)
	b, _ := topTeam(t, 70_000_002)
	c, _ := topTeam(t, 70_000_002)
	d, _ := topTeam(t, 70_000_001)
	hidden, _ := topTeam(t, 80_000_000) // the highest score of all, but it isn't listed or counted
	hideTeam(t, hidden)
	suffix := testdb.Unique()
	renameTeam(t, b, "tie-b-"+suffix) // the tie is ordered by name
	renameTeam(t, c, "tie-c-"+suffix)

	t.Run("first page", func(t *testing.T) {
		got := decodeTeams(t, e.Get(t, nil, "/api/teams?limit=4"))

		if want := []uuid.UUID{a, b, c, d}; !slices.Equal(ids(got), want) {
			t.Fatalf("teams = %v, want %v", ids(got), want)
		}
		if want := []int{1, 2, 2, 4}; !slices.Equal(ranks(got), want) {
			t.Errorf("ranks = %v, want %v: ties share a rank and the next rank skips", ranks(got), want)
		}
	})
	t.Run("rank does not depend on the page", func(t *testing.T) {
		if got := decodeTeams(t, e.Get(t, nil, "/api/teams?limit=2&offset=1")); !slices.Equal(ranks(got), []int{2, 2}) {
			t.Errorf("ranks of the tied teams on page 2 = %v, want [2 2]", ranks(got))
		}
		got := decodeTeams(t, e.Get(t, nil, "/api/teams?limit=1&offset=3"))
		if len(got) != 1 || got[0].ID != d || got[0].Rank != 4 {
			t.Errorf("fourth team = %+v, want %v with rank 4", got, d)
		}
	})
}

func TestTeamListNeverIncludesInviteCodes(t *testing.T) {
	e := newEnv(t)
	topTeam(t, 71_000_001)

	var teams []map[string]any
	if err := json.Unmarshal(e.Get(t, nil, "/api/teams?limit=100").Body.Bytes(), &teams); err != nil {
		t.Fatal(err)
	}

	if len(teams) == 0 {
		t.Fatal("no teams listed")
	}
	for _, tm := range teams {
		if _, ok := tm["invite_code"]; ok {
			t.Fatalf("the public list leaks an invite code: %v", tm)
		}
	}
}

func TestTeamListLeavesOutHiddenTeams(t *testing.T) {
	e := newEnv(t)
	visible, _ := topTeam(t, 72_000_002)
	hidden, _ := topTeam(t, 72_000_001)
	hideTeam(t, hidden)

	got := ids(decodeTeams(t, e.Get(t, nil, "/api/teams?limit=100")))

	if !slices.Contains(got, visible) || slices.Contains(got, hidden) {
		t.Errorf("visible listed = %v, hidden listed = %v; want true, false", slices.Contains(got, visible), slices.Contains(got, hidden))
	}
}

func TestTeamListPagination(t *testing.T) {
	e := newEnv(t)
	const limitMsg = "limit must be an integer between 1 and 100"
	const offsetMsg = "offset must be an integer between 0 and 2147483647"
	first, _ := topTeam(t, 73_000_003)
	second, _ := topTeam(t, 73_000_002)
	third, _ := topTeam(t, 73_000_001)

	if got := ids(decodeTeams(t, e.Get(t, nil, "/api/teams?limit=2"))); !slices.Equal(got, []uuid.UUID{first, second}) {
		t.Errorf("limit=2: got %v, want the top two", got)
	}
	if got := ids(decodeTeams(t, e.Get(t, nil, "/api/teams?limit=2&offset=1"))); !slices.Equal(got, []uuid.UUID{second, third}) {
		t.Errorf("limit=2&offset=1: got %v, want the second and third", got)
	}
	if got := decodeTeams(t, e.Get(t, nil, "/api/teams?limit=1&offset=2000000000")); len(got) != 0 {
		t.Errorf("an offset past the end returned %d teams, want none", len(got))
	}
	if got := len(decodeTeams(t, e.Get(t, nil, "/api/teams"))); got == 0 || got > 50 {
		t.Errorf("default page has %d teams, want 1-50", got)
	}

	for query, detail := range map[string]string{
		"?limit=0": limitMsg, "?limit=101": limitMsg, "?limit=abc": limitMsg,
		"?offset=-1": offsetMsg, "?offset=abc": offsetMsg, "?offset=2147483648": offsetMsg,
	} {
		t.Run(query, func(t *testing.T) {
			testenv.Expect(t, e.Get(t, nil, "/api/teams"+query), http.StatusBadRequest, detail)
		})
	}
}

// --- public team profile ---

func TestTeamProfile(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	id, _ := testdb.NewTeam(t, captain, member)
	addSolve(t, id, captain, 95_000_000, 0)

	rec := e.Get(t, nil, "/api/teams/"+id.String())

	got := testenv.Decode[team](t, rec)
	if got.ID != id || got.CaptainID == nil || *got.CaptainID != captain.ID {
		t.Errorf("team %v captain %v, want %v and %v", got.ID, got.CaptainID, id, captain.ID)
	}
	if got.MemberCount != 2 || got.Score != 95_000_000 || got.Rank != 1 || got.Hidden || got.Banned {
		t.Errorf("members %d score %d rank %d hidden %v banned %v; want 2, 95000000, 1, false, false",
			got.MemberCount, got.Score, got.Rank, got.Hidden, got.Banned)
	}
	if strings.Contains(rec.Body.String(), "invite_code") {
		t.Errorf("the public profile leaks the invite code: %s", rec.Body)
	}
}

func TestTeamProfileHiddenUnknownAndMalformed(t *testing.T) {
	e := newEnv(t)
	member := testdb.NewUser(t)
	hidden, _ := testdb.NewTeam(t, member)
	hideTeam(t, hidden)

	// Not even the team's own member sees it through the public route.
	testenv.Expect(t, e.Get(t, member, "/api/teams/"+hidden.String()), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.Get(t, nil, "/api/teams/"+hidden.String()), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.Get(t, nil, "/api/teams/"+uuid.NewV7().String()), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.Get(t, nil, "/api/teams/not-a-uuid"), http.StatusUnprocessableEntity, "Invalid id")
}

// --- activity ---

type activityRow struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	Username    string    `json:"username"`
	Challenge   string    `json:"challenge"`
	Category    string    `json:"category"`
	Points      int       `json:"points"`
}

func TestTeamActivityListsVisibleSolvesNewestFirst(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	id, _ := testdb.NewTeam(t, captain, member)
	cat := "cat" + testdb.Unique()
	oldest := addChallenge(t, cat, "oldest", 100, true)
	middle := addChallenge(t, cat, "middle", 200, true)
	newest := addChallenge(t, cat, "newest", 300, true)
	invisible := addChallenge(t, cat, "invisible", 999, false)
	solveChallenge(t, oldest, id, captain, 3*time.Hour)
	solveChallenge(t, middle, id, member, 2*time.Hour)
	solveChallenge(t, newest, id, captain, time.Hour)
	solveChallenge(t, invisible, id, member, 10*time.Minute)

	got := testenv.Decode[[]activityRow](t, e.Get(t, nil, "/api/teams/"+id.String()+"/activity"))

	if len(got) != 3 {
		t.Fatalf("got %d entries %+v, want 3 (the invisible challenge is left out)", len(got), got)
	}
	want := []activityRow{
		{newest, captain.Name, "newest", cat, 300},
		{middle, member.Name, "middle", cat, 200},
		{oldest, captain.Name, "oldest", cat, 100},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTeamActivityIsAnEmptyListWithoutSolves(t *testing.T) {
	e := newEnv(t)
	id, _ := testdb.NewTeam(t, testdb.NewUser(t))

	rec := e.Get(t, nil, "/api/teams/"+id.String()+"/activity")

	testenv.Expect(t, rec, http.StatusOK, "")
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
}

// --- progress ---

type progressResponse struct {
	Total      int `json:"total"`
	Solved     int `json:"solved"`
	Categories []struct {
		Category string `json:"category"`
		Total    int    `json:"total"`
		Solved   int    `json:"solved"`
	} `json:"categories"`
}

func TestTeamProgressCountsVisibleChallengesPerCategory(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	id, _ := testdb.NewTeam(t, captain)
	prefix := "prog" + testdb.Unique()
	a, b := prefix+"-a", prefix+"-b"
	solveChallenge(t, addChallenge(t, a, "a1", 10, true), id, captain, time.Hour)
	addChallenge(t, a, "a2", 10, true)                                              // visible, not solved
	solveChallenge(t, addChallenge(t, a, "a-invisible", 10, false), id, captain, 0) // doesn't count at all
	solveChallenge(t, addChallenge(t, b, "b1", 10, true), id, captain, 0)
	idle, _ := testdb.NewTeam(t, testdb.NewUser(t))

	got := testenv.Decode[progressResponse](t, e.Get(t, nil, "/api/teams/"+id.String()+"/progress"))

	type counts struct{ total, solved int }
	mine := map[string]counts{}
	var order []string
	sumTotal, sumSolved := 0, 0
	for _, c := range got.Categories {
		sumTotal += c.Total
		sumSolved += c.Solved
		if strings.HasPrefix(c.Category, prefix) {
			mine[c.Category] = counts{c.Total, c.Solved}
			order = append(order, c.Category)
		}
	}
	if mine[a] != (counts{2, 1}) || mine[b] != (counts{1, 1}) {
		t.Errorf("categories = %v, want %s: 2 visible and 1 solved, %s: 1 and 1", mine, a, b)
	}
	if !slices.Equal(order, []string{a, b}) {
		t.Errorf("category order = %v, want alphabetical", order)
	}
	if got.Total != sumTotal || got.Solved != sumSolved || got.Solved != 2 {
		t.Errorf("overall %d/%d, categories add up to %d/%d; want them equal and 2 solved", got.Solved, got.Total, sumSolved, sumTotal)
	}

	other := testenv.Decode[progressResponse](t, e.Get(t, nil, "/api/teams/"+idle.String()+"/progress"))
	if other.Solved != 0 || other.Total != got.Total {
		t.Errorf("a team without solves has %d/%d, want 0/%d", other.Solved, other.Total, got.Total)
	}
}

// --- compare ---

type compareResponse struct {
	Categories []struct {
		Category string `json:"category"`
		Total    int    `json:"total"`
		Them     int    `json:"them"`
		Us       int    `json:"us"`
	} `json:"categories"`
	Missing []struct {
		ID     uuid.UUID `json:"id"`
		Title  string    `json:"title"`
		Points int       `json:"points"`
	} `json:"missing"`
}

func (c compareResponse) missingTitles() []string {
	var out []string
	for _, m := range c.Missing {
		out = append(out, m.Title)
	}
	return out
}

// compareSetup builds two teams: "them" solved A1 (shared with "us"), B1, B2, B3 and C1; "us" only A1.
func compareSetup(t *testing.T) (prefix string, them, us uuid.UUID, ourCaptain *testdb.User) {
	t.Helper()
	prefix = "cmp" + testdb.Unique()
	themCaptain := testdb.NewUser(t)
	ourCaptain = testdb.NewUser(t)
	them, _ = testdb.NewTeam(t, themCaptain)
	us, _ = testdb.NewTeam(t, ourCaptain)
	a1 := addChallenge(t, prefix+"-a", "A1", 100, true)
	addChallenge(t, prefix+"-a", "A2", 70, true) // solved by nobody
	solveChallenge(t, a1, them, themCaptain, time.Hour)
	solveChallenge(t, a1, us, ourCaptain, time.Hour)
	for _, c := range []struct {
		cat, title string
		points     int
	}{{"-b", "B3", 300}, {"-b", "B1", 500}, {"-b", "B2", 500}, {"-c", "C1", 50}} {
		solveChallenge(t, addChallenge(t, prefix+c.cat, c.title, c.points, true), them, themCaptain, time.Minute)
	}
	return prefix, them, us, ourCaptain
}

func TestCompareShowsCategoriesAndWhatWeAreMissing(t *testing.T) {
	e := newEnv(t)
	prefix, them, _, us := compareSetup(t)

	got := testenv.Decode[compareResponse](t, e.Get(t, us, "/api/teams/"+them.String()+"/compare"))

	type row struct{ total, them, us int }
	var order []string
	rows := map[string]row{}
	for _, c := range got.Categories {
		if strings.HasPrefix(c.Category, prefix) {
			order = append(order, strings.TrimPrefix(c.Category, prefix))
			rows[strings.TrimPrefix(c.Category, prefix)] = row{c.Total, c.Them, c.Us}
		}
	}
	want := map[string]row{"-a": {2, 1, 1}, "-b": {3, 3, 0}, "-c": {1, 1, 0}}
	for k, w := range want {
		if rows[k] != w {
			t.Errorf("category %s = %+v, want %+v", k, rows[k], w)
		}
	}
	// Most solved by them first; categories with the same count stay alphabetical.
	if !slices.Equal(order, []string{"-b", "-a", "-c"}) {
		t.Errorf("category order = %v, want [-b -a -c]", order)
	}
	// Challenges they solved and we haven't, worth the most first, then by title. A1 is shared.
	if want := []string{"B1", "B2", "B3", "C1"}; !slices.Equal(got.missingTitles(), want) {
		t.Errorf("missing = %v, want %v", got.missingTitles(), want)
	}
}

func TestCompareWithAUserWithoutATeam(t *testing.T) {
	e := newEnv(t)
	prefix, them, _, _ := compareSetup(t)
	loner := testdb.NewUser(t)

	got := testenv.Decode[compareResponse](t, e.Get(t, loner, "/api/teams/"+them.String()+"/compare"))

	for _, c := range got.Categories {
		if c.Us != 0 {
			t.Errorf("category %s: us = %d, want 0", c.Category, c.Us)
		}
	}
	// Nothing is shared now, so A1 is missing too.
	if want := []string{"B1", "B2", "B3", "A1", "C1"}; !slices.Equal(got.missingTitles(), want) {
		t.Errorf("missing = %v, want %v (in %s)", got.missingTitles(), want, prefix)
	}
}

func TestCompareWithYourOwnTeamMissesNothing(t *testing.T) {
	e := newEnv(t)
	_, them, _, _ := compareSetup(t)
	captain := testdb.NewUser(t)
	testdb.Exec(t, `UPDATE users SET team_id = $1 WHERE id = $2`, them, captain.ID)

	got := testenv.Decode[compareResponse](t, e.Get(t, captain, "/api/teams/"+them.String()+"/compare"))

	if len(got.Missing) != 0 {
		t.Errorf("missing = %v, want none", got.missingTitles())
	}
	for _, c := range got.Categories {
		if c.Us != c.Them {
			t.Errorf("category %s: us %d, them %d, want equal", c.Category, c.Us, c.Them)
		}
	}
}

func TestCompareRequiresLogin(t *testing.T) {
	e := newEnv(t)
	them, _ := testdb.NewTeam(t, testdb.NewUser(t))

	testenv.Expect(t, e.Get(t, nil, "/api/teams/"+them.String()+"/compare"), http.StatusUnauthorized, "Not authenticated")
}

func TestTeamStatsForHiddenUnknownAndMalformedTeams(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	hidden, _ := testdb.NewTeam(t, testdb.NewUser(t))
	hideTeam(t, hidden)

	for _, route := range []string{"activity", "progress", "compare"} {
		t.Run(route, func(t *testing.T) {
			testenv.Expect(t, e.Get(t, viewer, "/api/teams/"+hidden.String()+"/"+route), http.StatusNotFound, "Team not found")
			testenv.Expect(t, e.Get(t, viewer, "/api/teams/"+uuid.NewV7().String()+"/"+route), http.StatusNotFound, "Team not found")
			testenv.Expect(t, e.Get(t, viewer, "/api/teams/not-a-uuid/"+route), http.StatusUnprocessableEntity, "Invalid id")
		})
	}
}

func TestActivityAndProgressArePublic(t *testing.T) {
	e := newEnv(t)
	id, _ := testdb.NewTeam(t, testdb.NewUser(t))

	testenv.Expect(t, e.Get(t, nil, "/api/teams/"+id.String()+"/activity"), http.StatusOK, "")
	testenv.Expect(t, e.Get(t, nil, "/api/teams/"+id.String()+"/progress"), http.StatusOK, "")
}

// --- team scoreboard ---

type series struct {
	TeamID   uuid.UUID `json:"team_id"`
	TeamName string    `json:"team_name"`
	Points   []struct {
		Score int `json:"score"`
	} `json:"points"`
}

func (s series) scores() []int {
	var out []int
	for _, p := range s.Points {
		out = append(out, p.Score)
	}
	return out
}

func seriesFor(all []series, id uuid.UUID) (series, bool) {
	i := slices.IndexFunc(all, func(s series) bool { return s.TeamID == id })
	if i < 0 {
		return series{}, false
	}
	return all[i], true
}

func TestScoreboardShowsTheScoreAfterEachSolveInTimeOrder(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	firstCaptain, secondCaptain := testdb.NewUser(t), testdb.NewUser(t)
	first, _ := testdb.NewTeam(t, firstCaptain)
	second, _ := testdb.NewTeam(t, secondCaptain)
	// Inserted newest first: the series must follow the solve time, not the insertion order.
	addSolve(t, first, firstCaptain, 250, time.Hour)
	addSolve(t, first, firstCaptain, 100, 2*time.Hour)
	addSolve(t, first, firstCaptain, 50_000_000, 3*time.Hour)
	addSolve(t, second, secondCaptain, 40_000_000, time.Hour)

	all := testenv.Decode[[]series](t, e.Get(t, viewer, "/api/scoreboard"))

	s1, ok1 := seriesFor(all, first)
	s2, ok2 := seriesFor(all, second)
	if !ok1 || !ok2 {
		t.Fatalf("teams present: first %v, second %v; want both", ok1, ok2)
	}
	if want := []int{50_000_000, 50_000_100, 50_000_350}; !slices.Equal(s1.scores(), want) {
		t.Errorf("first team scores = %v, want %v", s1.scores(), want)
	}
	if want := []int{40_000_000}; !slices.Equal(s2.scores(), want) {
		t.Errorf("second team scores = %v, want %v", s2.scores(), want)
	}
	if slices.IndexFunc(all, func(s series) bool { return s.TeamID == first }) > slices.IndexFunc(all, func(s series) bool { return s.TeamID == second }) {
		t.Error("the higher scoring team is listed after the lower one")
	}
	if s1.TeamName == "" {
		t.Error("the series has no team name")
	}
}

func TestScoreboardLeavesOutHiddenTeamsAndTeamsWithoutVisibleSolves(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	shown, _ := topTeam(t, 51_000_000)
	hidden, _ := topTeam(t, 52_000_000)
	hideTeam(t, hidden)
	// Its only solve is of an invisible challenge: it scores, so it takes a place in the top
	// ten, but it has no visible points to draw.
	quiet := testdb.NewUser(t)
	quietTeam, _ := testdb.NewTeam(t, quiet)
	solveChallenge(t, addChallenge(t, "cat"+testdb.Unique(), "secret", 53_000_000, false), quietTeam, quiet, time.Hour)

	all := testenv.Decode[[]series](t, e.Get(t, viewer, "/api/scoreboard"))

	if _, ok := seriesFor(all, shown); !ok {
		t.Error("the visible team is missing")
	}
	if _, ok := seriesFor(all, hidden); ok {
		t.Error("a hidden team is on the scoreboard")
	}
	if _, ok := seriesFor(all, quietTeam); ok {
		t.Error("a team without visible solves is on the scoreboard")
	}
}

func TestScoreboardOnlyShowsTheTopTenTeams(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	var teams []uuid.UUID // lowest score first
	for i := 1; i <= 11; i++ {
		id, _ := topTeam(t, 60_000_000+i)
		teams = append(teams, id)
	}

	all := testenv.Decode[[]series](t, e.Get(t, viewer, "/api/scoreboard"))

	if len(all) != 10 {
		t.Fatalf("got %d teams, want 10", len(all))
	}
	for i, s := range all {
		if want := teams[len(teams)-1-i]; s.TeamID != want {
			t.Errorf("position %d = %v, want %v", i+1, s.TeamID, want)
		}
	}
	if _, ok := seriesFor(all, teams[0]); ok {
		t.Error("the eleventh team is on the scoreboard")
	}
}

func TestScoreboardRequiresLogin(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.Get(t, nil, "/api/scoreboard"), http.StatusUnauthorized, "Not authenticated")
}

// --- player board ---

type boardEntry struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int       `json:"score"`
	Rank     int       `json:"rank"`
}

func TestPlayerBoardRanksPlayersAndTiesShareARank(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	suffix := testdb.Unique()
	named := func(name string, score int) *testdb.User {
		p := testdb.NewUser(t)
		p.Name = name + suffix
		testdb.Exec(t, `UPDATE users SET username = $1 WHERE id = $2`, p.Name, p.ID)
		setScore(t, p, score)
		return p
	}
	a, b := named("pb-a-", 9_000_003), named("pb-b-", 9_000_002)
	c, d := named("pb-c-", 9_000_002), named("pb-d-", 9_000_001)
	banned := named("pb-banned-", 9_000_009)
	testdb.Exec(t, `UPDATE users SET banned = true WHERE id = $1`, banned.ID)
	onHiddenTeam := named("pb-hidden-", 9_000_008)
	hiddenTeam, _ := testdb.NewTeam(t, onHiddenTeam)
	hideTeam(t, hiddenTeam)

	got := testenv.Decode[[]boardEntry](t, e.Get(t, viewer, "/api/scoreboard/players?limit=4"))

	want := []struct {
		p    *testdb.User
		rank int
	}{{a, 1}, {b, 2}, {c, 2}, {d, 4}}
	if len(got) != len(want) {
		t.Fatalf("got %d players %+v, want 4 (banned and hidden-team players are left out)", len(got), got)
	}
	for i, w := range want {
		if got[i].ID != w.p.ID || got[i].Username != w.p.Name || got[i].Rank != w.rank {
			t.Errorf("position %d = %+v, want %s with rank %d", i+1, got[i], w.p.Name, w.rank)
		}
	}
	if got[0].Score != 9_000_003 {
		t.Errorf("score = %d, want 9000003", got[0].Score)
	}
}

func TestPlayerBoardLimit(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	top := testdb.NewUser(t)
	setScore(t, top, 9_100_000)
	const limitMsg = "limit must be an integer between 1 and 1000"

	if got := testenv.Decode[[]boardEntry](t, e.Get(t, viewer, "/api/scoreboard/players?limit=1")); len(got) != 1 || got[0].ID != top.ID {
		t.Errorf("limit=1 returned %+v, want only the top player", got)
	}
	if got := testenv.Decode[[]boardEntry](t, e.Get(t, viewer, "/api/scoreboard/players")); len(got) == 0 || len(got) > 100 {
		t.Errorf("the default page has %d players, want 1-100", len(got))
	}
	if got := testenv.Decode[[]boardEntry](t, e.Get(t, viewer, "/api/scoreboard/players?limit=1000")); len(got) > 1000 {
		t.Errorf("limit=1000 returned %d players", len(got))
	}
	for _, query := range []string{"?limit=0", "?limit=1001", "?limit=-1", "?limit=abc"} {
		t.Run(query, func(t *testing.T) {
			testenv.Expect(t, e.Get(t, viewer, "/api/scoreboard/players"+query), http.StatusBadRequest, limitMsg)
		})
	}
}

func TestPlayerBoardRequiresLogin(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.Get(t, nil, "/api/scoreboard/players"), http.StatusUnauthorized, "Not authenticated")
}

// --- user profile ---

type profileResponse struct {
	ID       uuid.UUID  `json:"id"`
	Username string     `json:"username"`
	Score    int        `json:"score"`
	TeamID   *uuid.UUID `json:"team_id"`
	Team     *string    `json:"team"`
	Solves   []struct {
		Challenge string `json:"challenge"`
		Points    int    `json:"points"`
	} `json:"solves"`
}

func TestProfileShowsTheUserTheirTeamAndTheirVisibleSolves(t *testing.T) {
	e := newEnv(t)
	viewer := testdb.NewUser(t)
	player, teammate := testdb.NewUser(t), testdb.NewUser(t)
	id, _ := testdb.NewTeam(t, player, teammate)
	setScore(t, player, 4242)
	cat := "cat" + testdb.Unique()
	solveChallenge(t, addChallenge(t, cat, "older", 100, true), id, player, 2*time.Hour)
	solveChallenge(t, addChallenge(t, cat, "newer", 200, true), id, player, time.Hour)
	solveChallenge(t, addChallenge(t, cat, "invisible", 300, false), id, player, time.Minute)
	solveChallenge(t, addChallenge(t, cat, "teammates", 400, true), id, teammate, time.Minute) // someone else's

	got := testenv.Decode[profileResponse](t, e.Get(t, viewer, "/api/users/"+player.ID.String()))

	if got.ID != player.ID || got.Username != player.Name || got.Score != 4242 {
		t.Errorf("profile = %v %q score %d, want %v %q 4242", got.ID, got.Username, got.Score, player.ID, player.Name)
	}
	if got.TeamID == nil || *got.TeamID != id || got.Team == nil || *got.Team == "" {
		t.Errorf("team = %v %v, want the player's team %v", got.TeamID, got.Team, id)
	}
	var titles []string
	for _, s := range got.Solves {
		titles = append(titles, s.Challenge)
	}
	if want := []string{"newer", "older"}; !slices.Equal(titles, want) {
		t.Errorf("solves = %v, want %v: only their own visible solves, newest first", titles, want)
	}
}

func TestProfileWithoutATeamOrSolves(t *testing.T) {
	e := newEnv(t)
	viewer, loner := testdb.NewUser(t), testdb.NewUser(t)

	rec := e.Get(t, viewer, "/api/users/"+loner.ID.String())

	got := testenv.Decode[profileResponse](t, rec)
	if got.TeamID != nil || got.Team != nil {
		t.Errorf("team = %v %v, want none", got.TeamID, got.Team)
	}
	if !strings.Contains(rec.Body.String(), `"solves":[]`) {
		t.Errorf("body = %s, want an empty solves list", rec.Body)
	}
}

func TestProfileLeavesOutAHiddenTeam(t *testing.T) {
	e := newEnv(t)
	viewer, member := testdb.NewUser(t), testdb.NewUser(t)
	id, _ := testdb.NewTeam(t, member)
	hideTeam(t, id)

	got := testenv.Decode[profileResponse](t, e.Get(t, viewer, "/api/users/"+member.ID.String()))

	if got.TeamID != nil || got.Team != nil {
		t.Errorf("team = %v %v, want a hidden team left out", got.TeamID, got.Team)
	}
}

func TestProfileOfABannedUserIsNotFound(t *testing.T) {
	e := newEnv(t)
	viewer, banned := testdb.NewUser(t), testdb.NewUser(t)
	testdb.Exec(t, `UPDATE users SET banned = true WHERE id = $1`, banned.ID)

	testenv.Expect(t, e.Get(t, viewer, "/api/users/"+banned.ID.String()), http.StatusNotFound, "User not found")
}

func TestProfileUnknownMalformedAndLoggedOut(t *testing.T) {
	e := newEnv(t)
	viewer, other := testdb.NewUser(t), testdb.NewUser(t)

	testenv.Expect(t, e.Get(t, viewer, "/api/users/"+uuid.NewV7().String()), http.StatusNotFound, "User not found")
	testenv.Expect(t, e.Get(t, viewer, "/api/users/not-a-uuid"), http.StatusUnprocessableEntity, "Invalid id")
	testenv.Expect(t, e.Get(t, nil, "/api/users/"+other.ID.String()), http.StatusUnauthorized, "Not authenticated")
}

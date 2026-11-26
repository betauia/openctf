package teams

import (
	"encoding/json"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
	"github.com/betauia/openctf/backend/internal/testutil/testenv"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"uuid"
)

// These tests cover the /api/admin/teams routes. They share the helpers in teams_test.go and use
// the default policy, so they also pin which roles may call each route.

func (e *env) adminUpdate(t *testing.T, p *testdb.User, id uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	return e.Do(t, p, "PATCH", "/api/admin/teams/"+id.String(), body)
}

func (e *env) adminDelete(t *testing.T, p *testdb.User, id uuid.UUID) *httptest.ResponseRecorder {
	return e.Do(t, p, "DELETE", "/api/admin/teams/"+id.String(), nil)
}

func (e *env) adminKick(t *testing.T, p *testdb.User, team, target uuid.UUID) *httptest.ResponseRecorder {
	return e.Do(t, p, "DELETE", "/api/admin/teams/"+team.String()+"/members/"+target.String(), nil)
}

func (e *env) adminGet(t *testing.T, p *testdb.User, id uuid.UUID) *httptest.ResponseRecorder {
	return e.Do(t, p, "GET", "/api/admin/teams/"+id.String(), nil)
}

func (e *env) adminList(t *testing.T, p *testdb.User, query string) *httptest.ResponseRecorder {
	return e.Do(t, p, "GET", "/api/admin/teams"+query, nil)
}

func decodeTeams(t *testing.T, rec *httptest.ResponseRecorder) []team {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var teams []team
	if err := json.Unmarshal(rec.Body.Bytes(), &teams); err != nil {
		t.Fatalf("body %q is not a list of teams: %v", rec.Body, err)
	}
	return teams
}

func decodeTeam(t *testing.T, rec *httptest.ResponseRecorder) team {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var tm team
	if err := json.Unmarshal(rec.Body.Bytes(), &tm); err != nil {
		t.Fatalf("body %q is not a team: %v", rec.Body, err)
	}
	return tm
}

func indexOf(teams []team, id uuid.UUID) int {
	return slices.IndexFunc(teams, func(tm team) bool { return tm.ID == id })
}

// solve gives a team a solve worth points, so it has a score. The challenge is removed, and the
// score with it, when the test ends.
func solve(t *testing.T, team uuid.UUID, by *testdb.User, points int) {
	t.Helper()
	solveChallenge(t, addChallenge(t, "test", "challenge"+testdb.Unique(), points, true), team, by, 0)
}

// scoredTeam creates a team with a score, which puts it near the top of a team listing.
func scoredTeam(t *testing.T, points int) (id uuid.UUID, captain *testdb.User, code string) {
	t.Helper()
	captain = testdb.NewUser(t)
	id, code = testdb.NewTeam(t, captain)
	solve(t, id, captain, points)
	return id, captain, code
}

type teamRow struct {
	name           string
	banned, hidden bool
	captain        uuid.UUID
}

func rowOf(t *testing.T, id uuid.UUID) teamRow {
	t.Helper()
	var r teamRow
	err := testdb.Pool.QueryRow(t.Context(), `SELECT name, banned, hidden, captain_id FROM teams WHERE id = $1`, id).
		Scan(&r.name, &r.banned, &r.hidden, &r.captain)
	if err != nil {
		t.Fatalf("read team %v: %v", id, err)
	}
	return r
}

func teamExists(t *testing.T, id uuid.UUID) bool {
	return testdb.QueryRow[int](t, `SELECT count(*) FROM teams WHERE id = $1`, id) == 1
}

// --- authorization ---

func TestAdminRoutesRequireAnAdmin(t *testing.T) {
	e := newEnv(t)
	captain, member, outsider := testdb.NewUser(t), testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)
	before := rowOf(t, team)
	id, uid := team.String(), member.ID.String()

	routes := []struct {
		name, method, path string
		body               any
	}{
		{"update", "PATCH", "/api/admin/teams/" + id, map[string]any{"name": "hijacked" + testdb.Unique(), "banned": true, "hidden": true}},
		{"delete", "DELETE", "/api/admin/teams/" + id, nil},
		{"kick", "DELETE", "/api/admin/teams/" + id + "/members/" + uid, nil},
		{"get one", "GET", "/api/admin/teams/" + id, nil},
		{"list", "GET", "/api/admin/teams", nil},
	}
	callers := []struct {
		name   string
		player *testdb.User
		status int
	}{
		{"anonymous", nil, http.StatusUnauthorized},
		{"user without a team", outsider, http.StatusForbidden},
		{"captain of that team", captain, http.StatusForbidden},
		{"member of that team", member, http.StatusForbidden},
	}
	for _, r := range routes {
		for _, c := range callers {
			t.Run(r.name+"/"+c.name, func(t *testing.T) {
				testenv.Expect(t, e.Do(t, c.player, r.method, r.path, r.body), c.status, "")
			})
		}
	}

	if after := rowOf(t, team); after != before {
		t.Errorf("team changed to %+v, want %+v", after, before)
	}
	if got := memberCount(t, team); got != 2 {
		t.Errorf("member count = %d, want 2", got)
	}
}

// --- update ---

func TestAdminUpdateChangesOnlyTheGivenFields(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)

	// Each case sends some fields and names the row it expects afterwards.
	tests := []struct {
		name string
		body func(name string, captain uuid.UUID) map[string]any
		want func(before teamRow, name string, captain uuid.UUID) teamRow
	}{
		{"name",
			func(name string, _ uuid.UUID) map[string]any { return map[string]any{"name": name} },
			func(b teamRow, name string, _ uuid.UUID) teamRow { b.name = name; return b }},
		{"banned",
			func(string, uuid.UUID) map[string]any { return map[string]any{"banned": true} },
			func(b teamRow, _ string, _ uuid.UUID) teamRow { b.banned = true; return b }},
		{"hidden",
			func(string, uuid.UUID) map[string]any { return map[string]any{"hidden": true} },
			func(b teamRow, _ string, _ uuid.UUID) teamRow { b.hidden = true; return b }},
		{"captain",
			func(_ string, c uuid.UUID) map[string]any { return map[string]any{"captain_id": c} },
			func(b teamRow, _ string, c uuid.UUID) teamRow { b.captain = c; return b }},
		{"everything at once",
			func(name string, c uuid.UUID) map[string]any {
				return map[string]any{"name": name, "banned": true, "hidden": true, "captain_id": c}
			},
			func(b teamRow, name string, c uuid.UUID) teamRow {
				return teamRow{name: name, banned: true, hidden: true, captain: c}
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			captain, member := testdb.NewUser(t), testdb.NewUser(t)
			team, _ := testdb.NewTeam(t, captain, member)
			before := rowOf(t, team)
			name := "renamed" + testdb.Unique()

			testenv.Expect(t, e.adminUpdate(t, admin, team, tc.body(name, member.ID)), http.StatusOK, "")

			if got, want := rowOf(t, team), tc.want(before, name, member.ID); got != want {
				t.Errorf("after = %+v, want %+v", got, want)
			}
		})
	}
}

func TestAdminUpdateCanUnbanAndUnhide(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	team, _ := testdb.NewTeam(t, testdb.NewUser(t))
	testenv.Expect(t, e.adminUpdate(t, admin, team, map[string]any{"banned": true, "hidden": true}), http.StatusOK, "")

	testenv.Expect(t, e.adminUpdate(t, admin, team, map[string]any{"banned": false, "hidden": false}), http.StatusOK, "")

	if r := rowOf(t, team); r.banned || r.hidden {
		t.Errorf("after = %+v, want neither banned nor hidden", r)
	}
}

func TestAdminUpdateRejectsACaptainWhoIsNotAMember(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain)
	otherTeamMember := testdb.NewUser(t)
	testdb.NewTeam(t, otherTeamMember)

	tests := map[string]uuid.UUID{
		"member of another team": otherTeamMember.ID,
		"user without a team":    testdb.NewUser(t).ID,
		"unknown user":           uuid.NewV7(),
	}
	for name, candidate := range tests {
		t.Run(name, func(t *testing.T) {
			before := rowOf(t, team)

			// The rename is in the same request: a refused captain must not leave it half applied.
			rec := e.adminUpdate(t, admin, team, map[string]any{"name": "halfway" + testdb.Unique(), "captain_id": candidate})

			testenv.Expect(t, rec, http.StatusBadRequest, "The new captain must be a member of the team")
			if after := rowOf(t, team); after != before {
				t.Errorf("team changed to %+v, want %+v", after, before)
			}
		})
	}
}

func TestAdminUpdateNameRules(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	team, _ := testdb.NewTeam(t, testdb.NewUser(t))
	taken := "Taken" + testdb.Unique()
	testenv.Expect(t, e.create(t, testdb.NewUser(t), taken), http.StatusOK, "")
	before := rowOf(t, team)

	tests := []struct {
		name   string
		body   map[string]any
		detail string
	}{
		{"nothing to update", map[string]any{}, "Nothing to update"},
		{"too short", map[string]any{"name": "ab"}, "Team name must be 3-32 characters"},
		{"too long", map[string]any{"name": strings.Repeat("a", 33)}, "Team name must be 3-32 characters"},
		{"leading space", map[string]any{"name": " abc"}, "Team name can't start or end with spaces or contain invisible characters"},
		{"zero-width space", map[string]any{"name": "ab​c"}, "Team name can't start or end with spaces or contain invisible characters"},
		{"taken ignoring case", map[string]any{"name": strings.ToUpper(taken)}, "Team name already taken"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Expect(t, e.adminUpdate(t, admin, team, tc.body), http.StatusBadRequest, tc.detail)

			if after := rowOf(t, team); after != before {
				t.Errorf("team changed to %+v, want %+v", after, before)
			}
		})
	}
}

func TestAdminUpdateUnknownAndMalformedTeam(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	body := map[string]any{"name": "whatever" + testdb.Unique()}

	testenv.Expect(t, e.adminUpdate(t, admin, uuid.NewV7(), body), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.Do(t, admin, "PATCH", "/api/admin/teams/not-a-uuid", body), http.StatusUnprocessableEntity, "Invalid id")
}

func TestBanningATeamStopsNewMembersFromJoining(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	team, code := testdb.NewTeam(t, testdb.NewUser(t))
	testenv.Expect(t, e.join(t, testdb.NewUser(t), code), http.StatusOK, "") // joinable before the ban

	testenv.Expect(t, e.adminUpdate(t, admin, team, map[string]any{"banned": true}), http.StatusOK, "")

	late := testdb.NewUser(t)
	testenv.Expect(t, e.join(t, late, code), http.StatusForbidden, "Not allowed")
	if teamOf(t, late) != nil {
		t.Error("a refused join still added the user")
	}
	testenv.Expect(t, e.adminUpdate(t, admin, team, map[string]any{"banned": false}), http.StatusOK, "")
	testenv.Expect(t, e.join(t, late, code), http.StatusOK, "")
}

func TestHidingATeamRemovesItFromPublicViewsButNotAdminViews(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	id, _, code := scoredTeam(t, 1000) // a score keeps it on the first page of the listing
	publicList := func() []team { return decodeTeams(t, e.Do(t, nil, "GET", "/api/teams?limit=100", nil)) }
	publicGet := func() *httptest.ResponseRecorder { return e.Do(t, nil, "GET", "/api/teams/"+id.String(), nil) }

	if indexOf(publicList(), id) < 0 || publicGet().Code != http.StatusOK {
		t.Fatal("the team isn't publicly visible before it is hidden, so this test proves nothing")
	}
	testenv.Expect(t, e.adminUpdate(t, admin, id, map[string]any{"hidden": true}), http.StatusOK, "")

	if indexOf(publicList(), id) >= 0 {
		t.Error("a hidden team is still in the public list")
	}
	testenv.Expect(t, publicGet(), http.StatusNotFound, "Team not found")
	if i := indexOf(decodeTeams(t, e.adminList(t, admin, "?limit=100")), id); i < 0 {
		t.Error("a hidden team is missing from the admin list")
	}
	got := decodeTeam(t, e.adminGet(t, admin, id))
	if !got.Hidden || got.InviteCode == nil || *got.InviteCode != code {
		t.Errorf("admin view = hidden %v, invite code %v; want hidden with code %q", got.Hidden, got.InviteCode, code)
	}
}

// --- delete ---

func TestAdminDeleteRemovesTheTeamAndFreesItsMembers(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.adminDelete(t, admin, team), http.StatusOK, "")

	if teamExists(t, team) {
		t.Error("the team still exists")
	}
	if teamOf(t, captain) != nil || teamOf(t, member) != nil {
		t.Error("the members are still attached to the deleted team")
	}
	testenv.Expect(t, e.adminGet(t, admin, team), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.join(t, testdb.NewUser(t), code), http.StatusNotFound, "Invalid invite code")
	// The freed members can start over.
	testenv.Expect(t, e.create(t, captain, "again"+testdb.Unique()), http.StatusOK, "")
}

func TestAdminDeleteUnknownAndMalformedTeam(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)

	testenv.Expect(t, e.adminDelete(t, admin, uuid.NewV7()), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.Do(t, admin, "DELETE", "/api/admin/teams/not-a-uuid", nil), http.StatusUnprocessableEntity, "Invalid id")
}

// --- kick ---

func TestAdminKickRemovesAMember(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.adminKick(t, admin, team, member.ID), http.StatusOK, "")

	if teamOf(t, member) != nil {
		t.Error("the member is still in the team")
	}
	if got := memberCount(t, team); got != 1 {
		t.Errorf("member count = %d, want 1", got)
	}
	if got := captainOf(t, team); got != captain.ID {
		t.Errorf("captain = %v, want it unchanged (%v)", got, captain.ID)
	}
}

// UUIDv7 ids sort by creation time, so the oldest remaining account takes over.
func TestAdminKickingTheCaptainPromotesTheOldestMember(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain, older, newer := testdb.NewUser(t), testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, newer, older) // listed newest first; the order here must not matter

	testenv.Expect(t, e.adminKick(t, admin, team, captain.ID), http.StatusOK, "")

	if got := captainOf(t, team); got != older.ID {
		t.Errorf("captain = %v, want the oldest remaining member %v", got, older.ID)
	}
	if got := memberCount(t, team); got != 2 {
		t.Errorf("member count = %d, want 2", got)
	}
}

func TestAdminKickingTheLastMemberDeletesTheTeam(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain := testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain)

	testenv.Expect(t, e.adminKick(t, admin, team, captain.ID), http.StatusOK, "")

	if teamExists(t, team) {
		t.Error("the emptied team still exists")
	}
	if teamOf(t, captain) != nil {
		t.Error("the kicked user is still attached to a team")
	}
	testenv.Expect(t, e.join(t, testdb.NewUser(t), code), http.StatusNotFound, "Invalid invite code")
}

func TestAdminKickNotFoundCases(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain)
	otherTeamMember := testdb.NewUser(t)
	otherTeam, _ := testdb.NewTeam(t, otherTeamMember)

	tests := []struct {
		name         string
		team, target uuid.UUID
	}{
		{"member of another team", team, otherTeamMember.ID},
		{"user without a team", team, testdb.NewUser(t).ID},
		{"unknown user", team, uuid.NewV7()},
		{"unknown team", uuid.NewV7(), captain.ID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Expect(t, e.adminKick(t, admin, tc.team, tc.target), http.StatusNotFound, "Member not found")
		})
	}
	if got := teamOf(t, otherTeamMember); got == nil || *got != otherTeam {
		t.Error("a member of another team was removed from it")
	}
	if got := memberCount(t, team); got != 1 {
		t.Errorf("member count = %d, want 1", got)
	}
}

func TestAdminKickMalformedIDs(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	valid := uuid.NewV7().String()

	testenv.Expect(t, e.Do(t, admin, "DELETE", "/api/admin/teams/not-a-uuid/members/"+valid, nil), http.StatusUnprocessableEntity, "Invalid id")
	testenv.Expect(t, e.Do(t, admin, "DELETE", "/api/admin/teams/"+valid+"/members/not-a-uuid", nil), http.StatusUnprocessableEntity, "Invalid uid")
}

// RosterFrozen stops everyone but admins from changing teams once the CTF has ended.
func TestAdminCanKickAfterTheCTFEnds(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)
	testdb.EndCTF(t)

	testenv.Expect(t, e.kick(t, captain, member), http.StatusForbidden, "Not allowed") // the captain can't
	testenv.Expect(t, e.adminKick(t, admin, team, member.ID), http.StatusOK, "")       // an admin can

	if teamOf(t, member) != nil {
		t.Error("the member is still in the team")
	}
}

// --- read ---

func TestAdminGetShowsPrivateFields(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain, member)
	solve(t, team, captain, 250)
	testenv.Expect(t, e.adminUpdate(t, admin, team, map[string]any{"banned": true, "hidden": true}), http.StatusOK, "")

	got := decodeTeam(t, e.adminGet(t, admin, team))

	if got.ID != team || got.CaptainID == nil || *got.CaptainID != captain.ID {
		t.Errorf("team %v captain %v, want %v and %v", got.ID, got.CaptainID, team, captain.ID)
	}
	if got.InviteCode == nil || *got.InviteCode != code {
		t.Errorf("invite code = %v, want %q", got.InviteCode, code)
	}
	if !got.Banned || !got.Hidden || got.MemberCount != 2 || got.Score != 250 {
		t.Errorf("banned %v hidden %v members %d score %d; want true true 2 250", got.Banned, got.Hidden, got.MemberCount, got.Score)
	}
}

func TestAdminGetUnknownAndMalformedTeam(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)

	testenv.Expect(t, e.adminGet(t, admin, uuid.NewV7()), http.StatusNotFound, "Team not found")
	testenv.Expect(t, e.Do(t, admin, "GET", "/api/admin/teams/not-a-uuid", nil), http.StatusUnprocessableEntity, "Invalid id")
}

func TestAdminListIncludesHiddenTeamsAndInviteCodes(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	visible, _, visibleCode := scoredTeam(t, 1100)
	hidden, _, hiddenCode := scoredTeam(t, 1200)
	testenv.Expect(t, e.adminUpdate(t, admin, hidden, map[string]any{"hidden": true}), http.StatusOK, "")

	teams := decodeTeams(t, e.adminList(t, admin, "?limit=100"))

	for _, want := range []struct {
		id   uuid.UUID
		code string
	}{{visible, visibleCode}, {hidden, hiddenCode}} {
		i := indexOf(teams, want.id)
		if i < 0 {
			t.Errorf("team %v is missing from the admin list", want.id)
			continue
		}
		if teams[i].InviteCode == nil || *teams[i].InviteCode != want.code {
			t.Errorf("team %v invite code = %v, want %q", want.id, teams[i].InviteCode, want.code)
		}
	}
}

func TestAdminListIsSortedByScoreThenName(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	high, _, _ := scoredTeam(t, 2_000_000)
	mid, _, _ := scoredTeam(t, 1_000_000)
	tieA, _, _ := scoredTeam(t, 500_000)
	tieB, _, _ := scoredTeam(t, 500_000)
	suffix := testdb.Unique()
	testdb.Exec(t, `UPDATE teams SET name = $1 WHERE id = $2`, "tie-a-"+suffix, tieA)
	testdb.Exec(t, `UPDATE teams SET name = $1 WHERE id = $2`, "tie-b-"+suffix, tieB)

	teams := decodeTeams(t, e.adminList(t, admin, "?limit=100"))

	var order []int
	for _, id := range []uuid.UUID{high, mid, tieA, tieB} {
		i := indexOf(teams, id)
		if i < 0 {
			t.Fatalf("team %v is missing from the list", id)
		}
		order = append(order, i)
	}
	if !slices.IsSorted(order) {
		t.Errorf("positions = %v, want the high score first, then the middle, then the tie ordered by name", order)
	}
	if teams[order[0]].Score != 2_000_000 {
		t.Errorf("score = %d, want 2000000", teams[order[0]].Score)
	}
}

func TestAdminListPagination(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	// Scores above every other test's, so these three are the top of the list.
	first, _, _ := scoredTeam(t, 3_000_003)
	second, _, _ := scoredTeam(t, 3_000_002)
	third, _, _ := scoredTeam(t, 3_000_001)

	ids := func(query string) []uuid.UUID {
		var out []uuid.UUID
		for _, tm := range decodeTeams(t, e.adminList(t, admin, query)) {
			out = append(out, tm.ID)
		}
		return out
	}

	if got := ids("?limit=2"); !slices.Equal(got, []uuid.UUID{first, second}) {
		t.Errorf("limit=2: got %v, want the top two", got)
	}
	if got := ids("?limit=2&offset=1"); !slices.Equal(got, []uuid.UUID{second, third}) {
		t.Errorf("limit=2&offset=1: got %v, want the second and third", got)
	}
	if got := ids("?limit=1&offset=2"); !slices.Equal(got, []uuid.UUID{third}) {
		t.Errorf("limit=1&offset=2: got %v, want the third", got)
	}
	if got := ids("?limit=100&offset=2000000000"); len(got) != 0 {
		t.Errorf("an offset past the end returned %d teams, want none", len(got))
	}
	if got := len(ids("")); got == 0 || got > 50 {
		t.Errorf("default page has %d teams, want 1-50", got)
	}
	if got := len(ids("?limit=100")); got > 100 {
		t.Errorf("limit=100 returned %d teams", got)
	}
}

func TestAdminListRejectsBadPagination(t *testing.T) {
	e := newEnv(t)
	admin := testdb.NewAdmin(t)
	const limitMsg = "limit must be an integer between 1 and 100"
	const offsetMsg = "offset must be an integer between 0 and 2147483647"

	tests := []struct {
		query, detail string
	}{
		{"?limit=0", limitMsg},
		{"?limit=101", limitMsg},
		{"?limit=-1", limitMsg},
		{"?limit=abc", limitMsg},
		{"?limit=1.5", limitMsg},
		{"?offset=-1", offsetMsg},
		{"?offset=abc", offsetMsg},
		{"?offset=2147483648", offsetMsg},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			testenv.Expect(t, e.adminList(t, admin, tc.query), http.StatusBadRequest, tc.detail)
		})
	}
	for _, query := range []string{"?limit=1", "?limit=100", "?offset=0"} {
		if rec := e.adminList(t, admin, query); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", query, rec.Code)
		}
	}
}

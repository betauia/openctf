package teams

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
	"github.com/betauia/openctf/backend/internal/testutil/testenv"
)

const testSecret = "test-secret-key"

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// env is the auth and team routes on a mux, using the default policy file.
type env struct{ *testenv.Env }

func newEnv(t *testing.T) *env {
	t.Helper()
	return &env{testenv.New(t, func(mux *http.ServeMux, a *auth.Auth) { New(testdb.Pool, a).Register(mux) })}
}

// bannedPlayer returns a user who was logged in and then banned. A banned user can't log in, so
// this is the only way to have one send requests.
func (e *env) bannedPlayer(t *testing.T) *testdb.User {
	t.Helper()
	p := testdb.NewUser(t)
	e.Login(t, p)
	testdb.Exec(t, `UPDATE users SET banned = true WHERE id = $1`, p.ID)
	return p
}

func memberCount(t *testing.T, team uuid.UUID) int {
	return testdb.QueryRow[int](t, `SELECT count(*) FROM users WHERE team_id = $1`, team)
}

func captainOf(t *testing.T, team uuid.UUID) uuid.UUID {
	return testdb.QueryRow[uuid.UUID](t, `SELECT captain_id FROM teams WHERE id = $1`, team)
}

func teamOf(t *testing.T, p *testdb.User) *uuid.UUID {
	return testdb.QueryRow[*uuid.UUID](t, `SELECT team_id FROM users WHERE id = $1`, p.ID)
}

func (e *env) create(t *testing.T, p *testdb.User, name string) *httptest.ResponseRecorder {
	return e.Do(t, p, "POST", "/api/teams", map[string]string{"name": name})
}

func (e *env) join(t *testing.T, p *testdb.User, code string) *httptest.ResponseRecorder {
	return e.Do(t, p, "POST", "/api/teams/join", map[string]string{"invite_code": code})
}

func (e *env) leave(t *testing.T, p *testdb.User) *httptest.ResponseRecorder {
	return e.Do(t, p, "POST", "/api/teams/mine/leave", nil)
}

func (e *env) kick(t *testing.T, p, target *testdb.User) *httptest.ResponseRecorder {
	return e.Do(t, p, "DELETE", "/api/teams/mine/members/"+target.ID.String(), nil)
}

func (e *env) transfer(t *testing.T, p *testdb.User, to uuid.UUID) *httptest.ResponseRecorder {
	return e.Do(t, p, "PUT", "/api/teams/mine/captain", map[string]uuid.UUID{"user_id": to})
}

func (e *env) update(t *testing.T, p *testdb.User, body map[string]any) *httptest.ResponseRecorder {
	return e.Do(t, p, "PATCH", "/api/teams/mine", body)
}

type mineResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	CaptainID   *uuid.UUID `json:"captain_id"`
	InviteCode  string     `json:"invite_code"`
	MemberCount int        `json:"member_count"`
	Rank        int        `json:"rank"`
	Members     []struct {
		ID       uuid.UUID `json:"id"`
		Username string    `json:"username"`
	} `json:"members"`
}

func (e *env) mine(t *testing.T, p *testdb.User) mineResponse {
	t.Helper()
	rec := e.Do(t, p, "GET", "/api/teams/mine", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/teams/mine: status %d, body %s", rec.Code, rec.Body)
	}
	var m mineResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// --- create ---

func TestCreateTeamMakesTheCallerCaptainAndMember(t *testing.T) {
	e := newEnv(t)
	p := testdb.NewUser(t)
	name := "team" + testdb.Unique()

	rec := e.create(t, p, name)

	testenv.Expect(t, rec, http.StatusOK, "")
	var body struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	m := e.mine(t, p)
	if m.ID != body.ID || m.Name != name {
		t.Errorf("team = %v %q, want %v %q", m.ID, m.Name, body.ID, name)
	}
	if m.CaptainID == nil || *m.CaptainID != p.ID {
		t.Errorf("captain = %v, want %v", m.CaptainID, p.ID)
	}
	if m.MemberCount != 1 || len(m.Members) != 1 || m.Members[0].ID != p.ID {
		t.Errorf("members = %+v (count %d), want only the creator", m.Members, m.MemberCount)
	}
	if m.InviteCode == "" {
		t.Error("the creator can't see the invite code")
	}
}

func TestCreateTeamRejectsInvalidNames(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name, team, detail string
	}{
		{"empty", "", "Team name must be 3-32 characters"},
		{"too short", "ab", "Team name must be 3-32 characters"},
		{"too long", strings.Repeat("a", 33), "Team name must be 3-32 characters"},
		{"leading space", " abc", "Team name can't start or end with spaces or contain invisible characters"},
		{"trailing space", "abc ", "Team name can't start or end with spaces or contain invisible characters"},
		{"zero-width space", "ab​c", "Team name can't start or end with spaces or contain invisible characters"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := testdb.NewUser(t)

			testenv.Expect(t, e.create(t, p, tc.team), http.StatusBadRequest, tc.detail)

			if teamOf(t, p) != nil {
				t.Error("a rejected create still put the user in a team")
			}
		})
	}
}

func TestCreateTeamAcceptsBoundaryLengths(t *testing.T) {
	e := newEnv(t)
	for _, n := range []int{3, 32} {
		name := (testdb.Unique() + strings.Repeat("y", 32))[:n]

		testenv.Expect(t, e.create(t, testdb.NewUser(t), name), http.StatusOK, "")
	}
}

func TestCreateTeamNameIsUniqueIgnoringCase(t *testing.T) {
	e := newEnv(t)
	name := "Team" + testdb.Unique()
	testenv.Expect(t, e.create(t, testdb.NewUser(t), name), http.StatusOK, "")

	second := testdb.NewUser(t)
	testenv.Expect(t, e.create(t, second, strings.ToUpper(name)), http.StatusBadRequest, "Team name already taken")

	if teamOf(t, second) != nil {
		t.Error("the failed create still put the user in a team")
	}
}

func TestCreateTeamWhenAlreadyInATeamIsDenied(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	testdb.NewTeam(t, captain)

	testenv.Expect(t, e.create(t, captain, "team"+testdb.Unique()), http.StatusForbidden, "Not allowed")
}

func TestCreateTeamRequiresLogin(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.create(t, nil, "team"+testdb.Unique()), http.StatusUnauthorized, "Not authenticated")
}

// --- join ---

func TestJoinWithInviteCode(t *testing.T) {
	e := newEnv(t)
	captain, joiner := testdb.NewUser(t), testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain)

	testenv.Expect(t, e.join(t, joiner, code), http.StatusOK, "")

	if got := teamOf(t, joiner); got == nil || *got != team {
		t.Errorf("joiner's team = %v, want %v", got, team)
	}
	m := e.mine(t, joiner)
	if m.MemberCount != 2 || len(m.Members) != 2 {
		t.Errorf("members = %d (count %d), want 2", len(m.Members), m.MemberCount)
	}
}

func TestJoinWithUnknownOrEmptyInviteCode(t *testing.T) {
	e := newEnv(t)
	testdb.NewTeam(t, testdb.NewUser(t))

	for name, code := range map[string]string{"unknown": "not-a-code-" + testdb.Unique(), "empty": ""} {
		t.Run(name, func(t *testing.T) {
			p := testdb.NewUser(t)

			testenv.Expect(t, e.join(t, p, code), http.StatusNotFound, "Invalid invite code")

			if teamOf(t, p) != nil {
				t.Error("a failed join put the user in a team")
			}
		})
	}
}

func TestJoinWhenAlreadyInATeam(t *testing.T) {
	e := newEnv(t)
	member := testdb.NewUser(t)
	testdb.NewTeam(t, member)
	_, otherCode := testdb.NewTeam(t, testdb.NewUser(t))

	testenv.Expect(t, e.join(t, member, otherCode), http.StatusBadRequest, "You are already in a team")
}

func TestJoinRequiresLogin(t *testing.T) {
	e := newEnv(t)
	_, code := testdb.NewTeam(t, testdb.NewUser(t))

	testenv.Expect(t, e.join(t, nil, code), http.StatusUnauthorized, "Not authenticated")
}

func TestJoinFullTeam(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name    string
		maxSize int
		members int // members besides the captain
		want    int
	}{
		{"one below the limit", 3, 1, http.StatusOK},
		{"at the limit", 3, 2, http.StatusForbidden},
		{"above the limit", 2, 3, http.StatusForbidden},
		{"limit of zero is unlimited", 0, 6, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testdb.SetMaxTeamSize(t, tc.maxSize)
			var rest []*testdb.User
			for range tc.members {
				rest = append(rest, testdb.NewUser(t))
			}
			team, code := testdb.NewTeam(t, testdb.NewUser(t), rest...)
			joiner := testdb.NewUser(t)

			rec := e.join(t, joiner, code)

			testenv.Expect(t, rec, tc.want, "")
			if tc.want == http.StatusForbidden {
				if teamOf(t, joiner) != nil {
					t.Error("a refused join still added the user")
				}
				if got := memberCount(t, team); got != tc.members+1 {
					t.Errorf("member count = %d, want %d", got, tc.members+1)
				}
			}
		})
	}
}

func TestJoinBannedTeamIsDenied(t *testing.T) {
	e := newEnv(t)
	team, code := testdb.NewTeam(t, testdb.NewUser(t))
	testdb.Exec(t, `UPDATE teams SET banned = true WHERE id = $1`, team)
	joiner := testdb.NewUser(t)

	testenv.Expect(t, e.join(t, joiner, code), http.StatusForbidden, "Not allowed")

	if teamOf(t, joiner) != nil {
		t.Error("a refused join still added the user")
	}
}

func TestBannedUserCannotJoin(t *testing.T) {
	e := newEnv(t)
	_, code := testdb.NewTeam(t, testdb.NewUser(t))
	banned := e.bannedPlayer(t)

	testenv.Expect(t, e.join(t, banned, code), http.StatusForbidden, "Not allowed")

	if teamOf(t, banned) != nil {
		t.Error("a refused join still added the user")
	}
}

func TestBannedUserCannotCreateATeam(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.create(t, e.bannedPlayer(t), "team"+testdb.Unique()), http.StatusForbidden, "Not allowed")
}

func TestRosterIsFrozenAfterTheCTFEnds(t *testing.T) {
	e := newEnv(t)
	captain, member, joiner := testdb.NewUser(t), testdb.NewUser(t), testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain, member)
	testdb.EndCTF(t)

	t.Run("join", func(t *testing.T) {
		testenv.Expect(t, e.join(t, joiner, code), http.StatusForbidden, "Not allowed")
	})
	t.Run("leave", func(t *testing.T) {
		testenv.Expect(t, e.leave(t, member), http.StatusForbidden, "Not allowed")
	})
	t.Run("kick", func(t *testing.T) {
		testenv.Expect(t, e.kick(t, captain, member), http.StatusForbidden, "Not allowed")
	})
	if got := memberCount(t, team); got != 2 {
		t.Errorf("member count = %d, want 2: the roster changed while frozen", got)
	}
}

func TestAdminsCanStillJoinAfterTheCTFEnds(t *testing.T) {
	e := newEnv(t)
	_, code := testdb.NewTeam(t, testdb.NewUser(t))
	testdb.EndCTF(t)

	testenv.Expect(t, e.join(t, testdb.NewAdmin(t), code), http.StatusOK, "")
}

// --- leave ---

func TestMemberCanLeave(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.leave(t, member), http.StatusOK, "")

	if teamOf(t, member) != nil {
		t.Error("the member is still in the team")
	}
	if got := memberCount(t, team); got != 1 {
		t.Errorf("member count = %d, want 1", got)
	}
	testenv.Expect(t, e.Do(t, member, "GET", "/api/teams/mine", nil), http.StatusNotFound, "You are not in a team")
}

func TestCaptainCannotLeaveWhileOthersRemain(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.leave(t, captain), http.StatusForbidden, "Not allowed")

	if got := teamOf(t, captain); got == nil || *got != team {
		t.Error("the captain left although other members remain")
	}
}

func TestCaptainCanLeaveAfterTransferringCaptaincy(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)
	testenv.Expect(t, e.transfer(t, captain, member.ID), http.StatusOK, "")

	testenv.Expect(t, e.leave(t, captain), http.StatusOK, "")

	if got := captainOf(t, team); got != member.ID {
		t.Errorf("captain = %v, want %v", got, member.ID)
	}
	if got := memberCount(t, team); got != 1 {
		t.Errorf("member count = %d, want 1", got)
	}
}

func TestLastMemberLeavingDeletesTheTeam(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain)

	testenv.Expect(t, e.leave(t, captain), http.StatusOK, "")

	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM teams WHERE id = $1`, team); n != 0 {
		t.Errorf("the empty team still exists (%d rows)", n)
	}
	testenv.Expect(t, e.join(t, testdb.NewUser(t), code), http.StatusNotFound, "Invalid invite code")
}

func TestLeaveWithoutATeam(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.leave(t, testdb.NewUser(t)), http.StatusBadRequest, "You are not in a team")
}

// --- kick ---

func TestCaptainCanKickAMember(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.kick(t, captain, member), http.StatusOK, "")

	if teamOf(t, member) != nil {
		t.Error("the kicked member is still in the team")
	}
	if got := memberCount(t, team); got != 1 {
		t.Errorf("member count = %d, want 1", got)
	}
}

func TestKickingYourselfIsRejected(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, testdb.NewUser(t))

	testenv.Expect(t, e.kick(t, captain, captain), http.StatusBadRequest, "Use leave to leave your own team")

	if got := memberCount(t, team); got != 2 {
		t.Errorf("member count = %d, want 2", got)
	}
}

func TestKickingSomeoneOutsideTheTeamIsNotFound(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain)
	otherTeamMember := testdb.NewUser(t)
	testdb.NewTeam(t, otherTeamMember)

	tests := map[string]*testdb.User{
		"member of another team": otherTeamMember,
		"user without a team":    testdb.NewUser(t),
		"unknown user":           {ID: uuid.NewV7()},
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			testenv.Expect(t, e.kick(t, captain, target), http.StatusNotFound, "Member not found")
		})
	}
	if teamOf(t, otherTeamMember) == nil {
		t.Error("a member of another team was removed from it")
	}
	if got := memberCount(t, team); got != 1 {
		t.Errorf("member count = %d, want 1", got)
	}
}

func TestPlainMemberCannotKick(t *testing.T) {
	e := newEnv(t)
	captain, member, victim := testdb.NewUser(t), testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member, victim)

	testenv.Expect(t, e.kick(t, member, victim), http.StatusForbidden, "Not allowed")

	if got := memberCount(t, team); got != 3 {
		t.Errorf("member count = %d, want 3", got)
	}
}

func TestKickWithoutATeam(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.kick(t, testdb.NewUser(t), testdb.NewUser(t)), http.StatusBadRequest, "You are not in a team")
}

func TestKickRequiresLogin(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.kick(t, nil, testdb.NewUser(t)), http.StatusUnauthorized, "Not authenticated")
}

// --- transfer captain ---

func TestCaptainCanTransferCaptaincy(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.transfer(t, captain, member.ID), http.StatusOK, "")

	if got := captainOf(t, team); got != member.ID {
		t.Errorf("captain = %v, want %v", got, member.ID)
	}
	if m := e.mine(t, member); m.CaptainID == nil || *m.CaptainID != member.ID {
		t.Errorf("/mine reports captain %v, want %v", m.CaptainID, member.ID)
	}
	// The old captain lost the captain's powers and the new one gained them.
	testenv.Expect(t, e.update(t, captain, map[string]any{"regenerate_invite": true}), http.StatusForbidden, "Not allowed")
	testenv.Expect(t, e.update(t, member, map[string]any{"regenerate_invite": true}), http.StatusOK, "")
}

func TestTransferToSomeoneOutsideTheTeamIsNotFound(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain)
	otherTeamMember := testdb.NewUser(t)
	testdb.NewTeam(t, otherTeamMember)

	tests := map[string]uuid.UUID{
		"member of another team": otherTeamMember.ID,
		"user without a team":    testdb.NewUser(t).ID,
		"unknown user":           uuid.NewV7(),
	}
	for name, to := range tests {
		t.Run(name, func(t *testing.T) {
			testenv.Expect(t, e.transfer(t, captain, to), http.StatusNotFound, "Member not found")
		})
	}
	if got := captainOf(t, team); got != captain.ID {
		t.Errorf("captain = %v, want it unchanged (%v)", got, captain.ID)
	}
}

func TestPlainMemberCannotTransferCaptaincy(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.transfer(t, member, member.ID), http.StatusForbidden, "Not allowed")

	if got := captainOf(t, team); got != captain.ID {
		t.Errorf("captain = %v, want it unchanged (%v)", got, captain.ID)
	}
}

func TestTransferWithoutATeam(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.transfer(t, testdb.NewUser(t), uuid.NewV7()), http.StatusBadRequest, "You are not in a team")
}

// --- update ---

func TestCaptainCanRenameTheTeam(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	testdb.NewTeam(t, captain)
	name := "renamed" + testdb.Unique()

	testenv.Expect(t, e.update(t, captain, map[string]any{"name": name}), http.StatusOK, "")

	if m := e.mine(t, captain); m.Name != name {
		t.Errorf("name = %q, want %q", m.Name, name)
	}
}

func TestRegeneratingTheInviteCodeInvalidatesTheOldOne(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	_, oldCode := testdb.NewTeam(t, captain)

	rec := e.update(t, captain, map[string]any{"regenerate_invite": true})

	testenv.Expect(t, rec, http.StatusOK, "")
	var body struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.InviteCode == "" || body.InviteCode == oldCode {
		t.Fatalf("new invite code = %q, want a different non-empty code than %q", body.InviteCode, oldCode)
	}
	if got := e.mine(t, captain).InviteCode; got != body.InviteCode {
		t.Errorf("/mine invite code = %q, want %q", got, body.InviteCode)
	}
	testenv.Expect(t, e.join(t, testdb.NewUser(t), oldCode), http.StatusNotFound, "Invalid invite code")
	testenv.Expect(t, e.join(t, testdb.NewUser(t), body.InviteCode), http.StatusOK, "")
}

func TestRenameWithoutRegeneratingKeepsTheInviteCode(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	_, code := testdb.NewTeam(t, captain)

	rec := e.update(t, captain, map[string]any{"name": "renamed" + testdb.Unique()})

	testenv.Expect(t, rec, http.StatusOK, "")
	if strings.Contains(rec.Body.String(), "invite_code") {
		t.Errorf("the response includes an invite code although none was regenerated: %s", rec.Body)
	}
	if got := e.mine(t, captain).InviteCode; got != code {
		t.Errorf("invite code = %q, want it unchanged (%q)", got, code)
	}
}

func TestUpdateCanRenameAndRegenerateTogether(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	_, oldCode := testdb.NewTeam(t, captain)
	name := "both" + testdb.Unique()

	testenv.Expect(t, e.update(t, captain, map[string]any{"name": name, "regenerate_invite": true}), http.StatusOK, "")

	m := e.mine(t, captain)
	if m.Name != name || m.InviteCode == oldCode {
		t.Errorf("name %q, invite %q; want the new name and a new code", m.Name, m.InviteCode)
	}
}

func TestUpdateWithNothingToChange(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	testdb.NewTeam(t, captain)

	for name, body := range map[string]map[string]any{
		"empty body":              {},
		"regenerate set to false": {"regenerate_invite": false},
	} {
		t.Run(name, func(t *testing.T) {
			testenv.Expect(t, e.update(t, captain, body), http.StatusBadRequest, "Nothing to update")
		})
	}
}

func TestUpdateRejectsInvalidNames(t *testing.T) {
	e := newEnv(t)
	captain := testdb.NewUser(t)
	testdb.NewTeam(t, captain)

	testenv.Expect(t, e.update(t, captain, map[string]any{"name": "ab"}), http.StatusBadRequest, "Team name must be 3-32 characters")
	testenv.Expect(t, e.update(t, captain, map[string]any{"name": " abc"}), http.StatusBadRequest,
		"Team name can't start or end with spaces or contain invisible characters")
}

func TestUpdateRejectsATakenName(t *testing.T) {
	e := newEnv(t)
	name := "Taken" + testdb.Unique()
	testenv.Expect(t, e.create(t, testdb.NewUser(t), name), http.StatusOK, "")
	captain := testdb.NewUser(t)
	testdb.NewTeam(t, captain)

	testenv.Expect(t, e.update(t, captain, map[string]any{"name": strings.ToLower(name)}), http.StatusBadRequest, "Team name already taken")
}

func TestPlainMemberCannotUpdateTheTeam(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	_, code := testdb.NewTeam(t, captain, member)

	testenv.Expect(t, e.update(t, member, map[string]any{"name": "renamed" + testdb.Unique(), "regenerate_invite": true}), http.StatusForbidden, "Not allowed")

	if got := e.mine(t, captain).InviteCode; got != code {
		t.Errorf("invite code changed to %q, want %q", got, code)
	}
}

func TestUpdateWithoutATeam(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.update(t, testdb.NewUser(t), map[string]any{"name": "abc"}), http.StatusBadRequest, "You are not in a team")
}

// --- mine ---

func TestMineListsTheMembers(t *testing.T) {
	e := newEnv(t)
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, code := testdb.NewTeam(t, captain, member)

	m := e.mine(t, member)

	if m.ID != team || m.InviteCode != code || m.MemberCount != 2 {
		t.Errorf("team %v code %q count %d, want %v %q 2", m.ID, m.InviteCode, m.MemberCount, team, code)
	}
	if m.CaptainID == nil || *m.CaptainID != captain.ID {
		t.Errorf("captain = %v, want %v", m.CaptainID, captain.ID)
	}
	if m.Rank < 1 {
		t.Errorf("rank = %d, want at least 1", m.Rank)
	}
	got := map[uuid.UUID]string{}
	for _, mem := range m.Members {
		got[mem.ID] = mem.Username
	}
	if got[captain.ID] != captain.Name || got[member.ID] != member.Name || len(got) != 2 {
		t.Errorf("members = %v, want %s and %s", got, captain.Name, member.Name)
	}
}

func TestMineWithoutATeam(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.Do(t, testdb.NewUser(t), "GET", "/api/teams/mine", nil), http.StatusNotFound, "You are not in a team")
}

func TestMineRequiresLogin(t *testing.T) {
	e := newEnv(t)

	testenv.Expect(t, e.Do(t, nil, "GET", "/api/teams/mine", nil), http.StatusUnauthorized, "Not authenticated")
}

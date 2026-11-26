package policy

import "testing"

// These tests pin what internal/policies/default.json lets each kind of user do. They build the
// subject, resource and context the way the auth package does, so every attribute a deny
// statement reads is always present.

func defaultEngine(t *testing.T) *Engine {
	t.Helper()
	ps, err := LoadFile("../policies/default.json")
	if err != nil {
		t.Fatalf("load default policy: %v", err)
	}
	return NewEngine(ps)
}

// who describes a subject; the zero value is a logged out visitor.
type who struct {
	id, role           string
	banned, teamBanned bool
	team               string // the id of their team, empty for none
	captain            bool
}

func (w who) subject() Subject {
	s := Subject{ID: w.id, Attributes: map[string]any{
		"role": w.role, "banned": w.banned, "has_team": w.team != "", "team_id": w.team,
		"is_captain": w.captain, "team_banned": w.teamBanned,
	}}
	if w.role != "" {
		s.Roles = []string{w.role}
	}
	return s
}

var (
	visitor    = who{}
	player     = who{id: "u1", role: "user"}
	member     = who{id: "u1", role: "user", team: "t1"}
	captain    = who{id: "u1", role: "user", team: "t1", captain: true}
	maintainer = who{id: "m1", role: "maintainer"}
	admin      = who{id: "a1", role: "admin"}
)

func (w who) banned_() who      { w.banned = true; return w }
func (w who) teamIsBanned() who { w.teamBanned = true; return w }

// ctx builds the request context for a CTF phase.
func ctx(phase string, mods ...func(map[string]any)) map[string]any {
	c := map[string]any{"ctf_phase": phase, "max_team_size": 5, "registration": true, "admin_count": 2}
	for _, m := range mods {
		m(c)
	}
	return c
}

func maxTeamSize(n int) func(map[string]any) {
	return func(c map[string]any) { c["max_team_size"] = n }
}
func registration(open bool) func(map[string]any) {
	return func(c map[string]any) { c["registration"] = open }
}
func adminCount(n int) func(map[string]any) { return func(c map[string]any) { c["admin_count"] = n } }

// challengeRes is a challenge as the challenges package describes it to the policy.
func challengeRes(visible bool, owner string) Resource {
	return Resource{ID: "c1", Attributes: map[string]any{"visible": visible, "owner_id": owner}}
}

// teamRes is a team as the teams package describes it to the policy.
func teamRes(id string, members int, banned bool) Resource {
	return Resource{ID: id, Attributes: map[string]any{"member_count": members, "banned": banned, "hidden": false}}
}

// route is what a route-level Authorise passes before any resource is loaded.
var route = Resource{}

type rule struct {
	name   string
	who    who
	action string
	path   string
	res    Resource
	ctx    map[string]any
	want   bool
}

func runRules(t *testing.T, rules []rule) {
	t.Helper()
	e := defaultEngine(t)
	for _, r := range rules {
		t.Run(r.name, func(t *testing.T) {
			res := r.res
			res.Path = r.path
			d := e.Evaluate(Request{Subject: r.who.subject(), Action: r.action, Resource: res, Context: r.ctx})

			if d.Allowed != r.want {
				t.Errorf("%s on %s allowed = %v, want %v (%s, policy %q statement %q)",
					r.action, r.path, d.Allowed, r.want, d.Reason, d.PolicyID, d.StatementID)
			}
		})
	}
}

func TestDefaultPolicyVisitors(t *testing.T) {
	running := ctx("running")
	runRules(t, []rule{
		{"may read a public profile", visitor, "user:read_public", "/api/users/u9", route, running, true},
		{"may read the team list", visitor, "team:read_public", "/api/teams", route, running, true},
		{"may read a public team", visitor, "team:read_public", "/api/teams/t1", teamRes("t1", 2, false), running, true},
		{"may register while registration is open", visitor, "user:register", "/api/auth/register", route, running, true},
		{"may not register when it is closed", visitor, "user:register", "/api/auth/register", route, ctx("running", registration(false)), false},
		{"may not read challenges", visitor, "challenge:read", "/api/challenges/c1", challengeRes(true, ""), running, false},
		{"may not list challenges", visitor, "challenge:read", "/api/challenges", route, running, false},
		{"may not submit flags", visitor, "challenge:submit", "/api/challenges/submit", route, running, false},
		{"may not create a team", visitor, "team:create", "/api/teams", route, running, false},
		{"may not read their private team", visitor, "team:read_private", "/api/teams/mine", teamRes("t1", 2, false), running, false},
		{"may not read the settings", visitor, "settings:read", "/api/admin/settings", route, running, false},
		{"may not read an admin team view", visitor, "team:read_private", "/api/admin/teams", route, running, false},
		{"may not stream the live feed", visitor, "challenge:view_solves", "/api/live", route, running, false},
	})
}

func TestDefaultPolicyBannedUsersCanOnlyReadPublicData(t *testing.T) {
	running := ctx("running")
	banned := member.banned_()
	runRules(t, []rule{
		{"banned: may read a public profile", banned, "user:read_public", "/api/users/u9", route, running, true},
		{"banned: may read public teams", banned, "team:read_public", "/api/teams", route, running, true},
		{"banned: may not read challenges", banned, "challenge:read", "/api/challenges/c1", challengeRes(true, ""), running, false},
		{"banned: may not submit", banned, "challenge:submit", "/api/challenges/submit", route, running, false},
		{"banned: may not log in or refresh", banned, "user:login", "/api/auth/refresh", route, running, false},
		{"banned: may not read their own account", banned, "user:read_private", "/api/auth/me", route, running, false},
		{"banned: may not leave their team", banned, "team:leave", "/api/teams/mine/leave", teamRes("t1", 2, false), running, false},
		{"banned: may not create a team", player.banned_(), "team:create", "/api/teams", route, running, false},
		{"banned: may not join a team", player.banned_(), "team:join", "/api/teams/join", teamRes("t2", 1, false), running, false},
		{"banned admin: may not use admin routes", admin.banned_(), "settings:read", "/api/admin/settings", route, running, false},
		{"banned admin: may still read public data", admin.banned_(), "team:read_public", "/api/teams", route, running, true},
	})
}

func TestDefaultPolicyBannedTeamsCannotPlayButCanStillRead(t *testing.T) {
	running := ctx("running")
	m := captain.teamIsBanned()
	runRules(t, []rule{
		{"banned team: may not submit", m, "challenge:submit", "/api/challenges/submit", route, running, false},
		{"banned team: may not start an instance", m, "instance:start", "/api/challenges/c1/instance", Resource{Attributes: map[string]any{"visible": true, "needs_instance": true}}, running, false},
		{"banned team: captain may not update the team", m, "team:update", "/api/teams/mine", teamRes("t1", 2, false), running, false},
		{"banned team: captain may not kick", m, "team:kick", "/api/teams/mine/members/u2", teamRes("t1", 2, false), running, false},
		{"banned team: may not leave", m.teamIsBanned(), "team:leave", "/api/teams/mine/leave", teamRes("t1", 1, false), running, false},
		{"banned team: may read challenges", m, "challenge:read", "/api/challenges/c1", challengeRes(true, ""), running, true},
		{"banned team: may read their team", m, "team:read_private", "/api/teams/mine", teamRes("t1", 2, false), running, true},
	})
}

func TestDefaultPolicyPlayersFollowTheCTFPhase(t *testing.T) {
	visible, hidden := challengeRes(true, ""), challengeRes(false, "")
	submit := "/api/challenges/submit"
	runRules(t, []rule{
		// Before the CTF starts nothing about the challenges is open to players.
		{"not started: no challenge read", member, "challenge:read", "/api/challenges/c1", visible, ctx("not_started"), false},
		{"not started: no list", member, "challenge:read", "/api/challenges", route, ctx("not_started"), false},
		{"not started: no download", member, "challenge:download", "/api/challenges/c1/files", visible, ctx("not_started"), false},
		{"not started: no solves list", member, "challenge:view_solves", "/api/challenges/c1/solves", visible, ctx("not_started"), false},
		{"not started: no submit", member, "challenge:submit", submit, route, ctx("not_started"), false},
		// While it runs they can read, download and submit.
		{"running: read", member, "challenge:read", "/api/challenges/c1", visible, ctx("running"), true},
		{"running: list", member, "challenge:read", "/api/challenges", route, ctx("running"), true},
		{"running: download", member, "challenge:download", "/api/challenges/c1/files", visible, ctx("running"), true},
		{"running: solves list", member, "challenge:view_solves", "/api/challenges/c1/solves", visible, ctx("running"), true},
		{"running: submit with a team", member, "challenge:submit", submit, route, ctx("running"), true},
		{"running: submit to a visible challenge", member, "challenge:submit", submit, visible, ctx("running"), true},
		{"running: no submit without a team", player, "challenge:submit", submit, route, ctx("running"), false},
		{"running: hidden challenge read", member, "challenge:read", "/api/challenges/c1", hidden, ctx("running"), false},
		{"running: hidden challenge submit", member, "challenge:submit", submit, hidden, ctx("running"), false},
		// Afterwards they can read but not submit.
		{"ended: read", member, "challenge:read", "/api/challenges/c1", visible, ctx("ended"), true},
		{"ended: download", member, "challenge:download", "/api/challenges/c1/files", visible, ctx("ended"), true},
		{"ended: no submit", member, "challenge:submit", submit, route, ctx("ended"), false},
		// Instances only run while the CTF does, and only for your own team's.
		{"running: start an instance", member, "instance:start", "/api/challenges/c1/instance",
			Resource{Attributes: map[string]any{"visible": true, "needs_instance": true}}, ctx("running"), true},
		{"ended: no instance start", member, "instance:start", "/api/challenges/c1/instance",
			Resource{Attributes: map[string]any{"visible": true, "needs_instance": true}}, ctx("ended"), false},
		{"running: stop your team's instance", member, "instance:stop", "/api/challenges/c1/instance",
			Resource{Attributes: map[string]any{"team_id": "t1"}}, ctx("running"), true},
		{"running: not another team's instance", member, "instance:stop", "/api/challenges/c1/instance",
			Resource{Attributes: map[string]any{"team_id": "t2"}}, ctx("running"), false},
		// Admins and maintainers can work on challenges before the CTF.
		{"not started: admin reads", admin, "challenge:read", "/api/challenges/c1", hidden, ctx("not_started"), true},
		{"not started: maintainer reads", maintainer, "challenge:read", "/api/challenges/c1", visible, ctx("not_started"), true},
	})
}

func TestDefaultPolicyTeamRules(t *testing.T) {
	running := ctx("running")
	runRules(t, []rule{
		{"may create a team without one", player, "team:create", "/api/teams", route, running, true},
		{"may not create a second team", member, "team:create", "/api/teams", route, running, false},

		{"join a team with room", player, "team:join", "/api/teams/join", teamRes("t2", 4, false), running, true},
		{"join before the CTF starts", player, "team:join", "/api/teams/join", teamRes("t2", 1, false), ctx("not_started"), true},
		{"may not join a full team", player, "team:join", "/api/teams/join", teamRes("t2", 5, false), running, false},
		{"may not join an over full team", player, "team:join", "/api/teams/join", teamRes("t2", 9, false), running, false},
		{"a team size of 0 is unlimited", player, "team:join", "/api/teams/join", teamRes("t2", 50, false), ctx("running", maxTeamSize(0)), true},
		{"may not join a banned team", player, "team:join", "/api/teams/join", teamRes("t2", 1, true), running, false},
		{"may not join while on a team", member, "team:join", "/api/teams/join", teamRes("t2", 1, false), running, false},
		{"may not join after the CTF ends", player, "team:join", "/api/teams/join", teamRes("t2", 1, false), ctx("ended"), false},
		{"an admin may still join after it ends", admin, "team:join", "/api/teams/join", teamRes("t2", 1, false), ctx("ended"), true},

		{"a member may read their team", member, "team:read_private", "/api/teams/mine", teamRes("t1", 2, false), running, true},
		{"not another team", member, "team:read_private", "/api/teams/mine", teamRes("t2", 2, false), running, false},
		{"a member may leave", member, "team:leave", "/api/teams/mine/leave", teamRes("t1", 3, false), running, true},
		{"a member may not leave after the CTF ends", member, "team:leave", "/api/teams/mine/leave", teamRes("t1", 3, false), ctx("ended"), false},
		{"a captain may not leave a team with others", captain, "team:leave", "/api/teams/mine/leave", teamRes("t1", 3, false), running, false},
		{"a captain may leave when alone", captain, "team:leave", "/api/teams/mine/leave", teamRes("t1", 1, false), running, true},

		{"the captain may update the team", captain, "team:update", "/api/teams/mine", teamRes("t1", 3, false), running, true},
		{"the captain may kick", captain, "team:kick", "/api/teams/mine/members/u2", teamRes("t1", 3, false), running, true},
		{"the captain may transfer captaincy", captain, "team:transfer_captain", "/api/teams/mine/captain", teamRes("t1", 3, false), running, true},
		{"a member may not update the team", member, "team:update", "/api/teams/mine", teamRes("t1", 3, false), running, false},
		{"a member may not kick", member, "team:kick", "/api/teams/mine/members/u2", teamRes("t1", 3, false), running, false},
		{"a member may not transfer captaincy", member, "team:transfer_captain", "/api/teams/mine/captain", teamRes("t1", 3, false), running, false},
		{"the captain of another team may not update this one", captain, "team:update", "/api/teams/mine", teamRes("t2", 3, false), running, false},
		{"the captain may not kick after the CTF ends", captain, "team:kick", "/api/teams/mine/members/u2", teamRes("t1", 3, false), ctx("ended"), false},
		{"an admin captain may kick after it ends", who{id: "a1", role: "admin", team: "t1", captain: true}, "team:kick", "/api/admin/teams/t1/members/u2", teamRes("t1", 3, false), ctx("ended"), true},
	})
}

func TestDefaultPolicyMaintainers(t *testing.T) {
	running := ctx("running")
	mine, theirs := challengeRes(false, "m1"), challengeRes(false, "m2")
	runRules(t, []rule{
		{"may create challenges", maintainer, "challenge:create", "/api/admin/challenges", route, running, true},
		{"may update their own", maintainer, "challenge:update", "/api/admin/challenges/c1", mine, running, true},
		{"may read the secrets of their own", maintainer, "challenge:read_secrets", "/api/admin/challenges/c1", mine, running, true},
		{"may manage the files of their own", maintainer, "challenge:manage_files", "/api/admin/challenges/c1/files", mine, running, true},
		{"may not update another's", maintainer, "challenge:update", "/api/admin/challenges/c1", theirs, running, false},
		{"may not read another's secrets", maintainer, "challenge:read_secrets", "/api/admin/challenges/c1", theirs, running, false},
		{"may not manage another's files", maintainer, "challenge:manage_files", "/api/admin/challenges/c1/files", theirs, running, false},
		{"may use the admin routes before loading a challenge", maintainer, "challenge:read_secrets", "/api/admin/challenges", route, running, true},
		{"may not delete, not even their own", maintainer, "challenge:delete", "/api/admin/challenges/c1", mine, running, false},
		{"may read every challenge", maintainer, "challenge:read", "/api/challenges/c1", theirs, running, true},
		{"may read instances", maintainer, "instance:read", "/api/admin/instances", route, running, true},
		{"may stop instances", maintainer, "instance:stop", "/api/admin/instances/i1", route, running, true},
		{"may not reset instances", maintainer, "instance:reset", "/api/admin/instances/i1", route, running, false},
		{"may not extend instances", maintainer, "instance:extend", "/api/admin/instances/i1", route, running, false},
		{"may not read settings", maintainer, "settings:read", "/api/admin/settings", route, running, false},
		{"may not change settings", maintainer, "settings:update", "/api/admin/settings", route, running, false},
		{"may not use the team admin routes", maintainer, "team:read_private", "/api/admin/teams", route, running, false},
		{"may not ban a team", maintainer, "team:ban", "/api/admin/teams/t1", teamRes("t1", 2, false), running, false},
		{"may not list users", maintainer, "user:list", "/api/admin/users", route, running, false},
	})
}

func TestDefaultPolicyAdmins(t *testing.T) {
	running := ctx("running")
	someones := challengeRes(false, "m2")
	runRules(t, []rule{
		{"may create challenges", admin, "challenge:create", "/api/admin/challenges", route, running, true},
		{"may update anyone's", admin, "challenge:update", "/api/admin/challenges/c1", someones, running, true},
		{"may read anyone's secrets", admin, "challenge:read_secrets", "/api/admin/challenges/c1", someones, running, true},
		{"may manage anyone's files", admin, "challenge:manage_files", "/api/admin/challenges/c1/files", someones, running, true},
		{"may delete challenges", admin, "challenge:delete", "/api/admin/challenges/c1", someones, running, true},
		{"may read settings", admin, "settings:read", "/api/admin/settings", route, running, true},
		{"may change settings", admin, "settings:update", "/api/admin/settings", route, running, true},
		{"may read the team admin views", admin, "team:read_private", "/api/admin/teams", route, running, true},
		{"may update a team", admin, "team:update", "/api/admin/teams/t1", teamRes("t1", 2, false), running, true},
		{"may ban a team", admin, "team:ban", "/api/admin/teams/t1", teamRes("t1", 2, false), running, true},
		{"may move a team's captain", admin, "team:transfer_captain", "/api/admin/teams/t1", teamRes("t1", 2, false), running, true},
		{"may kick a member", admin, "team:kick", "/api/admin/teams/t1/members/u2", teamRes("t1", 2, false), running, true},
		{"may delete a team", admin, "team:delete", "/api/admin/teams/t1", teamRes("t1", 2, false), running, true},
		{"may list users", admin, "user:list", "/api/admin/users", route, running, true},
		{"may read submissions", admin, "submission:read", "/api/admin/submissions", route, running, true},
		{"may read the audit log", admin, "audit:read", "/api/admin/audit-log", route, running, true},
		{"may reset instances", admin, "instance:reset", "/api/admin/instances/i1", route, running, true},
		{"may extend instances", admin, "instance:extend", "/api/admin/instances/i1", route, running, true},
	})
}

func TestDefaultPolicyAdminsCannotLockThemselvesOut(t *testing.T) {
	userRes := func(id, role string) Resource {
		return Resource{ID: id, Attributes: map[string]any{"role": role}}
	}
	runRules(t, []rule{
		{"may ban another user", admin, "user:ban", "/api/admin/users/u5", userRes("u5", "user"), ctx("running"), true},
		{"may not ban themselves", admin, "user:ban", "/api/admin/users/a1", userRes("a1", "admin"), ctx("running"), false},
		{"may not change their own role", admin, "user:set_role", "/api/admin/users/a1", userRes("a1", "admin"), ctx("running"), false},
		{"may not delete themselves", admin, "user:delete", "/api/admin/users/a1", userRes("a1", "admin"), ctx("running"), false},
		{"may demote another admin while there are two", admin, "user:set_role", "/api/admin/users/a2", userRes("a2", "admin"), ctx("running", adminCount(2)), true},
		{"may not demote the last admin", admin, "user:set_role", "/api/admin/users/a2", userRes("a2", "admin"), ctx("running", adminCount(1)), false},
		{"may not ban the last admin", admin, "user:ban", "/api/admin/users/a2", userRes("a2", "admin"), ctx("running", adminCount(1)), false},
		{"may not delete the last admin", admin, "user:delete", "/api/admin/users/a2", userRes("a2", "admin"), ctx("running", adminCount(1)), false},
		{"the last admin rule is only about admins", admin, "user:set_role", "/api/admin/users/u5", userRes("u5", "user"), ctx("running", adminCount(1)), true},
	})
}

func TestDefaultPolicyPlayersHaveNoAdminActions(t *testing.T) {
	running := ctx("running")
	runRules(t, []rule{
		{"settings:read", member, "settings:read", "/api/admin/settings", route, running, false},
		{"settings:update", member, "settings:update", "/api/admin/settings", route, running, false},
		{"challenge:create", member, "challenge:create", "/api/admin/challenges", route, running, false},
		{"challenge:read_secrets", member, "challenge:read_secrets", "/api/admin/challenges", route, running, false},
		{"challenge:delete", member, "challenge:delete", "/api/admin/challenges/c1", challengeRes(true, ""), running, false},
		{"team:read_private on the admin route", member, "team:read_private", "/api/admin/teams", route, running, false},
		{"team:ban", member, "team:ban", "/api/admin/teams/t1", teamRes("t1", 2, false), running, false},
		{"user:list", member, "user:list", "/api/admin/users", route, running, false},
		{"audit:read", member, "audit:read", "/api/admin/audit-log", route, running, false},
	})
}

func TestDefaultPolicySelfServiceAuthRoutes(t *testing.T) {
	running := ctx("running")
	runRules(t, []rule{
		{"a user may log in", player, "user:login", "/api/auth/login", route, running, true},
		{"a user may refresh", player, "user:login", "/api/auth/refresh", route, running, true},
		{"a user may read their account", player, "user:read_private", "/api/auth/me", route, running, true},
		{"a user may change their password", player, "user:update", "/api/auth/password", route, running, true},
		{"a visitor may not read an account", visitor, "user:read_private", "/api/auth/me", route, running, false},
		{"any role may stream the live feed", player, "challenge:view_solves", "/api/live", route, running, true},
		{"a maintainer may stream the live feed", maintainer, "challenge:view_solves", "/api/live", route, running, true},
		{"an admin may stream the live feed", admin, "challenge:view_solves", "/api/live", route, running, true},
	})
}

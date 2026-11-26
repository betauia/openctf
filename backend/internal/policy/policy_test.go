package policy

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

// --- patterns and subjects ---

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern, value string
		want           bool
	}{
		{"*", "", true},
		{"*", "/api/anything", true},
		{"/api/teams", "/api/teams", true},
		{"/api/teams", "/api/teams/", false},
		{"/api/teams", "/api/team", false},
		{"", "", true},
		{"", "x", false},
		// A * matches any run of characters, including "/".
		{"/api/teams/*", "/api/teams/abc", true},
		{"/api/teams/*", "/api/teams/abc/members/def", true},
		{"/api/teams/*", "/api/teams/", true},
		{"/api/teams/*", "/api/teams", false},
		{"/api/teams/*", "/api/team", false},
		{"*/members/*", "/api/teams/x/members/y", true},
		{"*/members/*", "/api/teams/x/memberz/y", false},
		{"/api/admin/teams/*/members/*", "/api/admin/teams/abc/members/def", true},
		{"/api/admin/teams/*/members/*", "/api/admin/teams/abc/leave", false},
		{"team:*", "team:join", true},
		{"team:*", "challenge:join", false},
		{"*:read", "team:read", true},
		{"*:read", "team:read_private", false},
		// The pieces between the stars must appear in order, without overlapping.
		{"a*b*c", "abc", true},
		{"a*b*c", "a--b--c", true},
		{"a*b*c", "acb", false},
		{"a*b*c", "ab", false},
		{"a*a", "a", false},
		{"a*a", "aa", true},
		{"**", "anything", true},
		// Matching is case sensitive.
		{"/api/Teams", "/api/teams", false},
	}
	for _, tc := range tests {
		t.Run(tc.pattern+" vs "+tc.value, func(t *testing.T) {
			if got := matchPattern(tc.pattern, tc.value); got != tc.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
			}
		})
	}
}

func TestMatchAnyNeedsOnePatternToMatchOneValue(t *testing.T) {
	if !matchAny([]string{"x", "/api/*"}, "/other", "/api/teams") {
		t.Error("a later pattern matching a later value must count")
	}
	if matchAny([]string{"x", "y"}, "a", "b") {
		t.Error("nothing matches, yet matchAny said so")
	}
	if matchAny(nil, "a") || matchAny([]string{"*"}) {
		t.Error("no patterns or no values can't match")
	}
}

func TestMatchSubject(t *testing.T) {
	admin := Subject{ID: "u-1", Roles: []string{"admin", "maintainer"}}
	visitor := Subject{}

	tests := []struct {
		name     string
		patterns []string
		subject  Subject
		want     bool
	}{
		{"everyone matches a visitor", []string{"*"}, visitor, true},
		{"everyone matches a user", []string{"*"}, admin, true},
		{"a role", []string{"role:admin"}, admin, true},
		{"any of a user's roles", []string{"role:maintainer"}, admin, true},
		{"a role they lack", []string{"role:user"}, admin, false},
		{"a role pattern doesn't match a visitor", []string{"role:*"}, visitor, false},
		{"a specific user", []string{"user:u-1"}, admin, true},
		{"another user", []string{"user:u-2"}, admin, false},
		{"one of several patterns", []string{"role:user", "user:u-1"}, admin, true},
		{"a name is not a role", []string{"admin"}, admin, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchSubject(tc.patterns, tc.subject); got != tc.want {
				t.Errorf("matchSubject(%v) = %v, want %v", tc.patterns, got, tc.want)
			}
		})
	}
}

// --- conditions ---

// request is a request with one attribute of each kind set.
func request() Request {
	return Request{
		Subject: Subject{ID: "u-1", Attributes: map[string]any{
			"role": "user", "banned": false, "team_id": "t-1", "level": 7, "ratio": 0.5,
		}},
		Action: "team:join",
		Resource: Resource{Path: "/api/teams/join", ID: "t-1", Attributes: map[string]any{
			"member_count": 3, "banned": true, "name": "Hackers",
		}},
		Context: map[string]any{"ctf_phase": "running", "max_team_size": 5, "admin_count": json.Number("2")},
	}
}

func TestConditionOperators(t *testing.T) {
	tests := []struct {
		name string
		c    Condition
		want bool
	}{
		{"Equals strings", Condition{"Equals", "context.ctf_phase", "running"}, true},
		{"Equals differs", Condition{"Equals", "context.ctf_phase", "ended"}, false},
		{"Equals bools", Condition{"Equals", "resource.banned", true}, true},
		{"Equals int with float", Condition{"Equals", "subject.level", 7.0}, true},
		{"Equals int with json.Number", Condition{"Equals", "context.max_team_size", json.Number("5")}, true},
		{"Equals json.Number with int", Condition{"Equals", "context.admin_count", 2}, true},
		{"Equals numbers differ", Condition{"Equals", "subject.level", 8}, false},
		{"NotEquals", Condition{"NotEquals", "context.ctf_phase", "ended"}, true},
		{"NotEquals fails when equal", Condition{"NotEquals", "context.ctf_phase", "running"}, false},
		{"In a []any list", Condition{"In", "context.ctf_phase", []any{"running", "ended"}}, true},
		{"In a []string list", Condition{"In", "context.ctf_phase", []string{"running", "ended"}}, true},
		{"In without the value", Condition{"In", "context.ctf_phase", []any{"ended"}}, false},
		{"In an empty list", Condition{"In", "context.ctf_phase", []any{}}, false},
		{"NotIn", Condition{"NotIn", "request.action", []string{"user:read_public"}}, true},
		{"NotIn fails when present", Condition{"NotIn", "request.action", []string{"team:join"}}, false},
		{"In numbers", Condition{"In", "subject.level", []any{json.Number("7"), json.Number("9")}}, true},
		{"LessThan", Condition{"LessThan", "resource.member_count", 5}, true},
		{"LessThan is strict", Condition{"LessThan", "resource.member_count", 3}, false},
		{"LessThan json.Number", Condition{"LessThan", "context.admin_count", json.Number("3")}, true},
		{"LessThan float", Condition{"LessThan", "subject.ratio", 0.75}, true},
		{"GreaterThan", Condition{"GreaterThan", "resource.member_count", 1}, true},
		{"GreaterThan is strict", Condition{"GreaterThan", "resource.member_count", 3}, false},
		{"request.action", Condition{"Equals", "request.action", "team:join"}, true},
		{"resource.id", Condition{"Equals", "resource.id", "t-1"}, true},
		{"resource.path", Condition{"Equals", "resource.path", "/api/teams/join"}, true},
		{"subject.id", Condition{"Equals", "subject.id", "u-1"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evaluateCondition(tc.c, request())

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("evaluateCondition(%+v) = %v, want %v", tc.c, got, tc.want)
			}
		})
	}
}

func TestConditionReferencesToOtherAttributes(t *testing.T) {
	tests := []struct {
		name string
		c    Condition
		want bool
	}{
		{"subject attribute equals resource", Condition{"Equals", "resource.id", "${subject.team_id}"}, true},
		{"a reference that differs", Condition{"Equals", "resource.id", "${subject.role}"}, false},
		{"compare numbers by reference", Condition{"LessThan", "resource.member_count", "${context.max_team_size}"}, true},
		{"a list from an attribute is not a list", Condition{"In", "subject.role", "${resource.name}"}, false},
		{"a reference to a missing attribute fails", Condition{"Equals", "resource.id", "${subject.nope}"}, false},
		{"a malformed reference is just text", Condition{"Equals", "resource.name", "${Hackers"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evaluateCondition(tc.c, request())

			if tc.name == "a list from an attribute is not a list" {
				if err == nil {
					t.Error("want an error: an attribute holding a string is not a list")
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("got %v, %v; want %v, nil", got, err, tc.want)
			}
		})
	}
}

func TestAMissingKeyFailsTheConditionWithoutAnError(t *testing.T) {
	for _, key := range []string{"subject.nope", "resource.nope", "context.nope", "request.nope", "nodot", "other.scope", ""} {
		for _, op := range []string{"Equals", "NotEquals", "In", "NotIn", "LessThan", "GreaterThan"} {
			got, err := evaluateCondition(Condition{op, key, []any{1}}, request())

			if err != nil || got {
				t.Errorf("%s on %q = %v, %v; want false, nil (a missing attribute never satisfies a condition, even NotEquals)", op, key, got, err)
			}
		}
	}
}

func TestConditionTypeErrors(t *testing.T) {
	tests := map[string]Condition{
		"In needs a list":               {"In", "context.ctf_phase", "running"},
		"NotIn needs a list":            {"NotIn", "context.ctf_phase", 5},
		"LessThan needs numbers":        {"LessThan", "context.ctf_phase", 5},
		"GreaterThan needs numbers":     {"GreaterThan", "resource.member_count", "many"},
		"LessThan needs a numeric json": {"LessThan", "resource.member_count", json.Number("x")},
		"an unknown operator":           {"Matches", "context.ctf_phase", "running"},
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := evaluateCondition(c, request()); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestEveryConditionMustHold(t *testing.T) {
	yes := Condition{"Equals", "context.ctf_phase", "running"}
	no := Condition{"Equals", "context.ctf_phase", "ended"}
	broken := Condition{"In", "context.ctf_phase", "running"}

	if ok, err := evaluateConditions(nil, request()); !ok || err != nil {
		t.Errorf("no conditions = %v, %v; want true, nil", ok, err)
	}
	if ok, _ := evaluateConditions([]Condition{yes, yes}, request()); !ok {
		t.Error("all true conditions must hold")
	}
	if ok, _ := evaluateConditions([]Condition{yes, no}, request()); ok {
		t.Error("one false condition must fail them all")
	}
	if ok, err := evaluateConditions([]Condition{broken, yes}, request()); ok || err == nil {
		t.Errorf("a broken condition = %v, %v; want false and an error", ok, err)
	}
}

// --- loading ---

func load(t *testing.T, json string) ([]Policy, error) {
	t.Helper()
	return Load(strings.NewReader(json))
}

// policyJSON wraps one statement in a policy.
func policyJSON(statement string) string {
	return `[{"id":"p","statements":[` + statement + `]}]`
}

const validStatement = `{"sid":"s","effect":"allow","subjects":["role:user"],"actions":["team:join"],"resources":["/api/teams/join"]}`

func TestLoadAcceptsAValidFile(t *testing.T) {
	ps, err := load(t, policyJSON(validStatement))
	if err != nil {
		t.Fatal(err)
	}

	if len(ps) != 1 || ps[0].ID != "p" || len(ps[0].Statements) != 1 || ps[0].Statements[0].Sid != "s" {
		t.Errorf("policies = %+v", ps)
	}
}

func TestLoadAcceptsWildcardsAndGoodConditions(t *testing.T) {
	statement := `{"sid":"s","effect":"deny","subjects":["*"],"actions":["team:*","*"],"resources":["*","/api/*"],
		"conditions":[{"operator":"Equals","key":"subject.banned","value":true},
		              {"operator":"Equals","key":"resource.id","value":"${subject.team_id}"},
		              {"operator":"NotIn","key":"request.action","value":["user:read_public"]}]}`

	if _, err := load(t, policyJSON(statement)); err != nil {
		t.Errorf("a valid file was rejected: %v", err)
	}
}

func TestLoadRejectsMalformedPolicies(t *testing.T) {
	with := func(old, new string) string { return policyJSON(strings.Replace(validStatement, old, new, 1)) }
	tests := []struct {
		name, input, wantInError string
	}{
		{"not JSON", `nope`, "decode policies"},
		{"not a list", `{"id":"p"}`, "decode policies"},
		{"an unknown field", with(`"effect"`, `"efect"`), "unknown field"},
		{"a missing policy id", `[{"statements":[]}]`, "missing id"},
		{"a duplicate policy id", `[{"id":"p","statements":[]},{"id":"p","statements":[]}]`, "duplicate policy id"},
		{"an invalid effect", with(`"allow"`, `"permit"`), "invalid effect"},
		{"no effect", with(`"effect":"allow",`, ``), "invalid effect"},
		{"no subjects", with(`"subjects":["role:user"]`, `"subjects":[]`), "required"},
		{"no actions", with(`"actions":["team:join"]`, `"actions":[]`), "required"},
		{"no resources", with(`"resources":["/api/teams/join"]`, `"resources":[]`), "required"},
		{"an action that matches nothing", with(`team:join`, `team:jion`), "matches no known action"},
		{"a wildcard action that matches nothing", with(`team:join`, `nope:*`), "matches no known action"},
		{"a resource that isn't a path", with(`/api/teams/join`, `api/teams/join`), "must be"},
		{"a resource that is a URL", with(`/api/teams/join`, `https://x/api`), "must be"},
		{"an unknown operator", policyJSON(strings.Replace(validStatement, `}`, `,"conditions":[{"operator":"Matches","key":"subject.x","value":1}]}`, 1)), "unknown operator"},
		{"a bad condition key", policyJSON(strings.Replace(validStatement, `}`, `,"conditions":[{"operator":"Equals","key":"user.x","value":1}]}`, 1)), "must start with"},
		{"a bad reference value", policyJSON(strings.Replace(validStatement, `}`, `,"conditions":[{"operator":"Equals","key":"subject.x","value":"${user.x}"}]}`, 1)), "must reference"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(t, tc.input)

			if err == nil {
				t.Fatal("the file was accepted")
			}
			if !strings.Contains(err.Error(), tc.wantInError) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantInError)
			}
		})
	}
}

func TestLoadedNumbersCompareAgainstAttributes(t *testing.T) {
	statement := `{"sid":"s","effect":"allow","subjects":["*"],"actions":["team:join"],"resources":["/api/teams/join"],
		"conditions":[{"operator":"LessThan","key":"resource.member_count","value":5}]}`
	ps, err := load(t, policyJSON(statement))
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine(ps)
	req := func(members int) Request {
		return Request{Action: "team:join", Resource: Resource{Path: "/api/teams/join", Attributes: map[string]any{"member_count": members}}}
	}

	if !e.Evaluate(req(4)).Allowed || e.Evaluate(req(5)).Allowed {
		t.Error("a number in the file must compare against an int attribute")
	}
}

func TestLoadFile(t *testing.T) {
	path := t.TempDir() + "/policies.json"
	if err := writeFile(path, policyJSON(validStatement)); err != nil {
		t.Fatal(err)
	}

	if ps, err := LoadFile(path); err != nil || len(ps) != 1 {
		t.Errorf("LoadFile = %v, %v; want one policy", ps, err)
	}
	if _, err := LoadFile(t.TempDir() + "/missing.json"); err == nil {
		t.Error("a missing file was accepted")
	}
}

// --- decisions ---

// engine returns an engine with the given statements in one policy.
func engine(statements ...Statement) *Engine {
	return NewEngine([]Policy{{ID: "p", Statements: statements}})
}

func stmt(sid string, effect Effect, conds ...Condition) Statement {
	return Statement{Sid: sid, Effect: effect, Subjects: []string{"role:user"}, Actions: []string{"team:join"},
		Resources: []string{"/api/teams/join"}, Conditions: conds}
}

func joinRequest() Request {
	return Request{Subject: Subject{ID: "u-1", Roles: []string{"user"}}, Action: "team:join", Resource: Resource{Path: "/api/teams/join"}}
}

func TestNothingMatchingIsRefused(t *testing.T) {
	d := engine().Evaluate(joinRequest())

	if d.Allowed || !strings.Contains(d.Reason, "default deny") || d.PolicyID != "" || d.StatementID != "" {
		t.Errorf("decision = %+v, want a default deny that names no statement", d)
	}
}

func TestAnAllowStatementAllowsAndNamesItself(t *testing.T) {
	d := engine(stmt("Join", Allow)).Evaluate(joinRequest())

	if !d.Allowed || d.PolicyID != "p" || d.StatementID != "Join" {
		t.Errorf("decision = %+v, want allowed by p/Join", d)
	}
}

func TestADenyStatementBeatsAnAllowInEitherOrder(t *testing.T) {
	for name, statements := range map[string][]Statement{
		"deny first":  {stmt("No", Deny), stmt("Yes", Allow)},
		"allow first": {stmt("Yes", Allow), stmt("No", Deny)},
	} {
		t.Run(name, func(t *testing.T) {
			d := engine(statements...).Evaluate(joinRequest())

			if d.Allowed || d.Reason != "explicit deny" || d.StatementID != "No" {
				t.Errorf("decision = %+v, want an explicit deny by No", d)
			}
		})
	}
}

func TestADenyInAnotherPolicyStillWins(t *testing.T) {
	e := NewEngine([]Policy{
		{ID: "player", Statements: []Statement{stmt("Yes", Allow)}},
		{ID: "system", Statements: []Statement{stmt("No", Deny)}},
	})

	if d := e.Evaluate(joinRequest()); d.Allowed || d.PolicyID != "system" {
		t.Errorf("decision = %+v, want denied by the system policy", d)
	}
}

func TestTheFirstMatchingAllowIsTheOneReported(t *testing.T) {
	d := engine(stmt("First", Allow), stmt("Second", Allow)).Evaluate(joinRequest())

	if !d.Allowed || d.StatementID != "First" {
		t.Errorf("decision = %+v, want First", d)
	}
}

func TestAStatementNeedsSubjectActionAndResourceToMatch(t *testing.T) {
	tests := map[string]func(*Request){
		"another subject":  func(r *Request) { r.Subject.Roles = []string{"admin"} },
		"another action":   func(r *Request) { r.Action = "team:leave" },
		"another resource": func(r *Request) { r.Resource.Path = "/api/other" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			req := joinRequest()
			change(&req)

			if engine(stmt("Join", Allow)).Evaluate(req).Allowed {
				t.Error("the statement applied although it doesn't match")
			}
		})
	}
}

func TestConditionsDecideWhetherAStatementApplies(t *testing.T) {
	onlyWhenRunning := Condition{"Equals", "context.ctf_phase", "running"}
	req := func(phase string) Request {
		r := joinRequest()
		r.Context = map[string]any{"ctf_phase": phase}
		return r
	}

	if !engine(stmt("Join", Allow, onlyWhenRunning)).Evaluate(req("running")).Allowed {
		t.Error("an allow with a true condition must allow")
	}
	if engine(stmt("Join", Allow, onlyWhenRunning)).Evaluate(req("ended")).Allowed {
		t.Error("an allow with a false condition must not apply")
	}
	if d := engine(stmt("Join", Allow), stmt("Frozen", Deny, Condition{"Equals", "context.ctf_phase", "ended"})).Evaluate(req("ended")); d.Allowed {
		t.Errorf("a deny whose condition holds must refuse: %+v", d)
	}
}

// A statement with a broken condition is skipped, so a broken deny never blocks and a broken allow
// never grants.
func TestAConditionErrorNeverGrantsAccess(t *testing.T) {
	broken := Condition{"In", "context.ctf_phase", "running"} // In needs a list
	req := joinRequest()
	req.Context = map[string]any{"ctf_phase": "running"}

	if engine(stmt("Join", Allow, broken)).Evaluate(req).Allowed {
		t.Error("an allow with a broken condition granted access")
	}
	if !engine(stmt("Join", Allow), stmt("No", Deny, broken)).Evaluate(req).Allowed {
		t.Error("a deny with a broken condition blocked an allow; it should be skipped")
	}
}

func TestADenyThatReadsAMissingAttributeDoesNotApply(t *testing.T) {
	deny := stmt("NoBanned", Deny, Condition{"Equals", "subject.banned", true})

	// This is why every attribute a deny statement reads must always be set.
	if !engine(stmt("Join", Allow), deny).Evaluate(joinRequest()).Allowed {
		t.Error("a deny reading a missing attribute applied")
	}
}

func TestSetPoliciesSwapsTheRules(t *testing.T) {
	e := engine(stmt("Join", Allow))
	if !e.Evaluate(joinRequest()).Allowed {
		t.Fatal("setup: the request should be allowed")
	}

	e.SetPolicies(nil)

	if e.Evaluate(joinRequest()).Allowed {
		t.Error("the old policies are still in force")
	}
	e.SetPolicies([]Policy{{ID: "new", Statements: []Statement{stmt("Back", Allow)}}})
	if d := e.Evaluate(joinRequest()); !d.Allowed || d.PolicyID != "new" {
		t.Errorf("decision = %+v, want allowed by the new policy", d)
	}
}

func TestEvaluateAndSetPoliciesCanRunConcurrently(t *testing.T) {
	e := engine(stmt("Join", Allow))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 200 {
				e.Evaluate(joinRequest())
			}
		}()
		go func() {
			defer wg.Done()
			for range 200 {
				e.SetPolicies([]Policy{{ID: "p", Statements: []Statement{stmt("Join", Allow)}}})
			}
		}()
	}
	wg.Wait()
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }

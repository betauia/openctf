package policy

// Effect says whether a statement allows or denies the requests it matches.
type Effect string

const (
	Allow Effect = "allow"
	Deny  Effect = "deny"
)

// Policy is a named group of statements, loaded from JSON.
type Policy struct {
	ID          string      `json:"id"`
	Description string      `json:"description,omitempty"`
	Statements  []Statement `json:"statements"`
}

// Statement applies to a request when its subject, action and resource patterns match and
// every condition holds. Patterns may use * as a wildcard.
type Statement struct {
	Sid        string      `json:"sid"`
	Effect     Effect      `json:"effect"`
	Subjects   []string    `json:"subjects"`  // "user:<id>", "role:<name>" or "*"
	Actions    []string    `json:"actions"`   // e.g. "team:join", "team:*" or "*"
	Resources  []string    `json:"resources"` // API paths, e.g. "/api/teams/mine/*"
	Conditions []Condition `json:"conditions,omitempty"`
}

// Condition compares the attribute named by Key ("subject.x", "resource.x" or "context.x")
// with Value. A string Value of the form "${scope.x}" is replaced by that attribute.
type Condition struct {
	Operator string `json:"operator"`
	Key      string `json:"key"`
	Value    any    `json:"value"`
}

// Subject is who performs the request. A logged out visitor has an empty ID and no roles.
type Subject struct {
	ID         string
	Roles      []string
	Attributes map[string]any
}

// Resource is what the request acts on. Path is the request path that statements' Resources
// match against. Every attribute a deny statement reads must be set, because a missing
// attribute makes the condition fail and the deny not apply.
type Resource struct {
	Path       string
	ID         string
	Attributes map[string]any
}

// Request is everything a decision depends on.
type Request struct {
	Subject  Subject
	Action   string
	Resource Resource
	Context  map[string]any
}

// Decision is the outcome of Evaluate, with the statement that decided it.
type Decision struct {
	Allowed     bool
	Reason      string
	PolicyID    string
	StatementID string
}

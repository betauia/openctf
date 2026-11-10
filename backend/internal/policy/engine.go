package policy

import (
	"sync"
)

// Engine evaluates requests against a set of policies. It is safe for concurrent use, and
// SetPolicies swaps the policies without a restart.
type Engine struct {
	mu       sync.RWMutex
	policies []Policy
}

func NewEngine(policies []Policy) *Engine {
	return &Engine{policies: policies}
}

func (e *Engine) SetPolicies(p []Policy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies = p
}

// Evaluate decides req. Nothing matching means refused (default deny), and any matching deny
// statement refuses even when an allow statement also matches (deny overrides).
func (e *Engine) Evaluate(req Request) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var allow *Decision
	for _, p := range e.policies {
		for _, s := range p.Statements {
			if !matchSubject(s.Subjects, req.Subject) ||
				!matchAny(s.Actions, req.Action) ||
				!matchAny(s.Resources, req.Resource.Path) {
				continue
			}
			ok, err := evaluateConditions(s.Conditions, req)
			if err != nil || !ok {
				continue // a broken condition never grants access
			}
			if s.Effect == Deny {
				return Decision{Allowed: false, Reason: "explicit deny", PolicyID: p.ID, StatementID: s.Sid}
			}
			if allow == nil {
				allow = &Decision{Allowed: true, Reason: "allowed", PolicyID: p.ID, StatementID: s.Sid}
			}
		}
	}
	if allow != nil {
		return *allow
	}
	return Decision{Allowed: false, Reason: "no matching allow statement (default deny)"}
}

package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// LoadFile reads and validates policies from a JSON file.
func LoadFile(path string) ([]Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load reads and validates policies from any reader.
func Load(r io.Reader) ([]Policy, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields() // catch typos like "efect"
	dec.UseNumber()

	var policies []Policy
	if err := dec.Decode(&policies); err != nil {
		return nil, fmt.Errorf("decode policies: %w", err)
	}
	ids := map[string]bool{}
	for _, p := range policies {
		if ids[p.ID] {
			return nil, fmt.Errorf("duplicate policy id %q", p.ID)
		}
		ids[p.ID] = true
		if err := validate(p); err != nil {
			return nil, err
		}
	}
	return policies, nil
}

// validate rejects malformed policies at load time instead of at request time.
func validate(p Policy) error {
	if p.ID == "" {
		return fmt.Errorf("policy missing id")
	}
	for i, s := range p.Statements {
		where := fmt.Sprintf("policy %q statement %d (%s)", p.ID, i, s.Sid)
		if s.Effect != Allow && s.Effect != Deny {
			return fmt.Errorf("%s: invalid effect %q", where, s.Effect)
		}
		if len(s.Subjects) == 0 || len(s.Actions) == 0 || len(s.Resources) == 0 {
			return fmt.Errorf("%s: subjects, actions and resources are required", where)
		}
		for _, a := range s.Actions {
			if !matchAny([]string{a}, Actions...) {
				return fmt.Errorf("%s: action %q matches no known action", where, a)
			}
		}
		for _, r := range s.Resources {
			if r != "*" && !strings.HasPrefix(r, "/") {
				return fmt.Errorf("%s: resource %q must be \"*\" or an API path starting with /", where, r)
			}
		}
		for _, c := range s.Conditions {
			if !validOperators[c.Operator] {
				return fmt.Errorf("%s: unknown operator %q", where, c.Operator)
			}
			if !validKey(c.Key) {
				return fmt.Errorf("%s: condition key %q must start with subject., resource. or context., or be request.action", where, c.Key)
			}
			if v, ok := c.Value.(string); ok && strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") && !validKey(v[2:len(v)-1]) {
				return fmt.Errorf("%s: condition value %q must reference subject., resource. or context.", where, v)
			}
		}
	}
	return nil
}

func validKey(key string) bool {
	scope, attr, ok := strings.Cut(key, ".")
	if key == "request.action" {
		return true
	}
	return ok && attr != "" && (scope == "subject" || scope == "resource" || scope == "context")
}

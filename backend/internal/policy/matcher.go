package policy

import "strings"

// matchPattern matches value against pattern, where * matches any run of characters,
// including "/".
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(value, parts[0]) {
		return false
	}
	value = value[len(parts[0]):]
	for _, mid := range parts[1 : len(parts)-1] {
		idx := strings.Index(value, mid)
		if idx < 0 {
			return false
		}
		value = value[idx+len(mid):]
	}
	return strings.HasSuffix(value, parts[len(parts)-1])
}

// matchAny reports whether any pattern matches any of values.
func matchAny(patterns []string, values ...string) bool {
	for _, p := range patterns {
		for _, v := range values {
			if matchPattern(p, v) {
				return true
			}
		}
	}
	return false
}

func matchSubject(patterns []string, s Subject) bool {
	ids := make([]string, 0, len(s.Roles)+1)
	ids = append(ids, "user:"+s.ID)
	for _, r := range s.Roles {
		ids = append(ids, "role:"+r)
	}
	return matchAny(patterns, ids...)
}

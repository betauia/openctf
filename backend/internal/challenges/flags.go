package challenges

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"log/slog"
	"regexp"
	"strings"
)

// flagEntry is the object form of a flag. A flag can also be a plain string,
// which must match exactly.
type flagEntry struct {
	Type    string `json:"type"`    // "static" or "regex"
	Content string `json:"content"` // the flag, or a pattern that must match the whole submission
	Data    string `json:"data"`    // "case_insensitive" or empty
}

// CheckFlag reports whether submitted matches any flag in the challenge's flags JSON.
func CheckFlag(submitted string, flags []byte) bool {
	var entries []jsontext.Value
	if err := json.Unmarshal(flags, &entries); err != nil {
		return false
	}
	for _, raw := range entries {
		if raw.Kind() == '"' {
			var s string
			if json.Unmarshal(raw, &s) == nil && s == submitted {
				return true
			}
			continue
		}
		var e flagEntry
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		insensitive := e.Data == "case_insensitive"
		switch e.Type {
		case "static":
			if submitted == e.Content || insensitive && strings.EqualFold(submitted, e.Content) {
				return true
			}
		case "regex":
			if matchFull(e.Content, submitted, insensitive) {
				return true
			}
		}
	}
	return false
}

// matchFull reports whether pattern matches all of s. Go's RE2 has no backreferences or
// lookarounds, so such patterns fail to compile and never match.
func matchFull(pattern, s string, insensitive bool) bool {
	prefix := ""
	if insensitive {
		prefix = "(?i)"
	}
	re, err := regexp.Compile(prefix + `^(?:` + pattern + `)$`)
	if err != nil {
		slog.Warn("invalid flag regex", "pattern", pattern, "err", err)
		return false
	}
	return re.MatchString(s)
}

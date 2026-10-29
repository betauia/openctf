package challenges

import "testing"

func TestCheckFlag(t *testing.T) {
	flags := []byte(`[
		"flag{plain}",
		{"type": "static", "content": "flag{Static}"},
		{"type": "static", "content": "flag{CaseLess}", "data": "case_insensitive"},
		{"type": "regex", "content": "flag\\{re_[0-9]+\\}"},
		{"type": "regex", "content": "ctf\\{abc\\}", "data": "case_insensitive"},
		{"type": "unknown", "content": "flag{unknown}"},
		42
	]`)
	tests := []struct {
		submitted string
		want      bool
	}{
		{"flag{plain}", true},
		{"FLAG{plain}", false},
		{"flag{Static}", true},
		{"flag{static}", false},
		{"FLAG{caseless}", true},
		{"flag{re_123}", true},
		{"flag{re_123}x", false}, // must match the whole submission
		{"xflag{re_1}", false},
		{"flag{re_}", false},
		{"CTF{ABC}", true},
		{"flag{unknown}", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := CheckFlag(tt.submitted, flags); got != tt.want {
			t.Errorf("CheckFlag(%q) = %v, want %v", tt.submitted, got, tt.want)
		}
	}
}

func TestCheckFlagBadInput(t *testing.T) {
	for _, flags := range []string{``, `{}`, `not json`, `[{"type": "regex", "content": "(a"}]`} {
		if CheckFlag("a", []byte(flags)) {
			t.Errorf("CheckFlag matched with flags %s", flags)
		}
	}
}

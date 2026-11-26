package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/betauia/openctf/backend"

// writeProfile writes a coverage profile with the given block lines.
func writeProfile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	content := "mode: set\n"
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runReport runs report and returns the total and what it printed.
func runReport(t *testing.T, path string) (float64, string) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	total, reportErr := report(path, module)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if reportErr != nil {
		t.Fatalf("report: %v", reportErr)
	}
	return total, string(out)
}

func TestReportSumsStatementsPerPackageAndInTotal(t *testing.T) {
	path := writeProfile(t,
		module+"/internal/auth/a.go:1.1,3.2 4 1",
		module+"/internal/auth/a.go:4.1,6.2 6 0",
		module+"/internal/auth/b.go:1.1,2.2 10 5",
		module+"/internal/teams/t.go:1.1,2.2 20 0",
	)

	total, out := runReport(t, path)

	// auth: 14 of 20 covered. teams: 0 of 20. Total: 14 of 40.
	if total != 35 {
		t.Errorf("total = %v, want 35", total)
	}
	for _, want := range []string{"70.0%", "14/20", "internal/auth", "0.0%", "0/20", "internal/teams", "35.0%", "14/40", "total"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

// Every package's test binary writes every block, so the same block shows up several times, and
// it counts once, as covered if any run reached it.
func TestReportCountsADuplicatedBlockOnce(t *testing.T) {
	path := writeProfile(t,
		module+"/internal/auth/a.go:1.1,3.2 5 0",
		module+"/internal/auth/a.go:1.1,3.2 5 0",
		module+"/internal/auth/a.go:1.1,3.2 5 3",
		module+"/internal/auth/a.go:4.1,6.2 5 0",
		module+"/internal/auth/a.go:4.1,6.2 5 0",
	)

	total, out := runReport(t, path)

	if total != 50 {
		t.Errorf("total = %v, want 50: two blocks of 5, one covered", total)
	}
	if !strings.Contains(out, "5/10") {
		t.Errorf("output does not show 5/10:\n%s", out)
	}
}

func TestReportListsTheLeastCoveredPackageFirst(t *testing.T) {
	path := writeProfile(t,
		module+"/internal/best/x.go:1.1,2.2 10 1",
		module+"/internal/worst/x.go:1.1,2.2 10 0",
		module+"/internal/middle/x.go:1.1,2.2 5 1",
		module+"/internal/middle/x.go:3.1,4.2 5 0",
	)

	_, out := runReport(t, path)

	worst, middle, best := strings.Index(out, "internal/worst"), strings.Index(out, "internal/middle"), strings.Index(out, "internal/best")
	if worst < 0 || middle < 0 || best < 0 || !(worst < middle && middle < best) {
		t.Errorf("packages are not sorted worst first:\n%s", out)
	}
}

func TestReportNamesPackagesWithoutTheModulePath(t *testing.T) {
	path := writeProfile(t, module+"/internal/auth/sessions/s.go:1.1,2.2 1 1")

	_, out := runReport(t, path)

	if !strings.Contains(out, "internal/auth/sessions") || strings.Contains(out, module) {
		t.Errorf("output should name the package relative to the module:\n%s", out)
	}
}

func TestReportOfAnEmptyProfileIsFullyCovered(t *testing.T) {
	total, _ := runReport(t, writeProfile(t))

	if total != 100 {
		t.Errorf("total = %v, want 100 when there is nothing to cover", total)
	}
}

func TestReportRejectsAMalformedProfile(t *testing.T) {
	for name, line := range map[string]string{
		"too few fields":       module + "/internal/a/a.go:1.1,2.2 5",
		"no colon in the path": "garbage 1 1",
		"a non-numeric count":  module + "/internal/a/a.go:1.1,2.2 five 1",
		"a non-numeric hit":    module + "/internal/a/a.go:1.1,2.2 5 many",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := report(writeProfile(t, line), module); err == nil {
				t.Error("a malformed profile was accepted")
			}
		})
	}
}

func TestReportFailsWhenTheProfileIsMissing(t *testing.T) {
	if _, err := report(filepath.Join(t.TempDir(), "missing.out"), module); err == nil {
		t.Error("a missing profile was accepted")
	}
}

func TestGeneratedAndSupportCodeIsExcluded(t *testing.T) {
	tests := map[string]bool{
		module + "/internal/database/dbgen":   true,
		module + "/cmd/server":                true,
		module + "/tools/coverage":            true,
		module + "/internal/testutil/testdb":  true,
		module + "/internal/testutil/testenv": true,
		module + "/internal/auth":             false,
		module + "/internal/database":         false,
		module + "/internal/teams":            false,
		module + "/internal/httpserver":       false,
	}
	for pkg, want := range tests {
		if got := isExcluded(pkg); got != want {
			t.Errorf("isExcluded(%q) = %v, want %v", pkg, got, want)
		}
	}
}

func TestMeetsMinimum(t *testing.T) {
	tests := []struct {
		total, min float64
		want       bool
	}{
		{80, 80, true},
		{80.1, 80, true},
		{79.9, 80, false},
		{0, 0, true},
		{12, 0, true},
		{100, 100, true},
		{99.99, 100, false},
	}
	for _, tc := range tests {
		if got := meetsMinimum(tc.total, tc.min); got != tc.want {
			t.Errorf("meetsMinimum(%v, %v) = %v, want %v", tc.total, tc.min, got, tc.want)
		}
	}
}

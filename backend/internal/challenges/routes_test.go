package challenges

import (
	"testing"

	"github.com/betauia/openctf/backend/internal/policy"
)

// The routes authorise with policy actions; these tests check the default policy file
// answers those actions the way the routes expect.
func TestRouteActionsAgainstDefaultPolicy(t *testing.T) {
	policies, err := policy.LoadFile("../policies/default.json")
	if err != nil {
		t.Fatalf("load default policy: %v", err)
	}
	engine := policy.NewEngine(policies)

	admin := policy.Subject{ID: "admin-1", Roles: []string{"admin"}}
	visitor := policy.Subject{}
	running := map[string]any{"ctf_phase": "running"}

	tests := []struct {
		name    string
		subject policy.Subject
		action  string
		path    string
		want    bool
	}{
		{"admin lists challenges", admin, policy.ChallengeRead, "/api/challenges", true},
		{"admin views solves", admin, policy.ChallengeViewSolves, "/api/challenges/1/solves", true},
		{"admin downloads files", admin, policy.ChallengeDownload, "/api/challenges/1/files", true},
		{"admin reads secrets", admin, policy.ChallengeReadSecrets, "/api/admin/challenges/1", true},
		{"admin manages files", admin, policy.ChallengeManageFiles, "/api/admin/challenges/1/files", true},
		{"visitor cannot read secrets", visitor, policy.ChallengeReadSecrets, "/api/admin/challenges/1", false},
		{"visitor cannot manage files", visitor, policy.ChallengeManageFiles, "/api/admin/challenges/1/files", false},
		{"visitor cannot submit", visitor, policy.ChallengeSubmit, "/api/challenges/submit", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := engine.Evaluate(policy.Request{
				Subject:  tc.subject,
				Action:   tc.action,
				Resource: policy.Resource{Path: tc.path, Attributes: map[string]any{"visible": true}},
				Context:  running,
			})
			if d.Allowed != tc.want {
				t.Errorf("allowed = %v, want %v (reason %q)", d.Allowed, tc.want, d.Reason)
			}
		})
	}
}

package teams

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database/dbgen"
)

func TestTeamOutAdd(t *testing.T) {
	out := newTeam(dbgen.Team{ID: uuid.NewV7(), Name: "A"})
	if out.Members == nil || out.Score != 0 {
		t.Fatalf("newTeam = %+v; want empty non-nil members", out)
	}
	out.add(Member{Username: "a", Score: 100})
	out.add(Member{Username: "b", Score: 50})
	if len(out.Members) != 2 || out.Score != 150 {
		t.Errorf("after add = %+v", out)
	}
}

func call(fn func(http.ResponseWriter, *http.Request, dbgen.User), u dbgen.User, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	fn(w, httptest.NewRequest(method, "/", strings.NewReader(body)), u)
	return w
}

func TestMyTeamWithoutTeam(t *testing.T) {
	h := New(nil, nil, nil)
	w := call(h.myTeam, dbgen.User{}, "GET", "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "null" {
		t.Errorf("got %d %s, want 200 null", w.Code, w.Body)
	}
}

// Validation failures that respond before the store is touched.
func TestTeamValidation(t *testing.T) {
	notified := false
	h := New(nil, nil, func() { notified = true })
	team := uuid.NewV7()
	inTeam := dbgen.User{TeamID: &team}
	tests := []struct {
		name       string
		fn         func(http.ResponseWriter, *http.Request, dbgen.User)
		user       dbgen.User
		body       string
		wantStatus int
		wantDetail string
	}{
		{"join bad body", h.joinTeam, dbgen.User{}, `{`, http.StatusUnprocessableEntity, "Invalid request body"},
		{"join while in team", h.joinTeam, inTeam, `{"invite_code":"x"}`, http.StatusBadRequest, "Already in a team"},
		{"create bad body", h.createTeam, dbgen.User{}, `[]`, http.StatusUnprocessableEntity, "Invalid request body"},
		{"create while in team", h.createTeam, inTeam, `{"name":"Team"}`, http.StatusBadRequest, "Already in a team"},
		{"create short name", h.createTeam, dbgen.User{}, `{"name":"A"}`, http.StatusBadRequest, "2–32 characters"},
		{"create long name", h.createTeam, dbgen.User{}, `{"name":"` + strings.Repeat("x", 33) + `"}`, http.StatusBadRequest, "2–32 characters"},
		{"leave without team", h.leaveTeam, dbgen.User{}, ``, http.StatusBadRequest, "Not in a team"},
		{"activity without team", h.myActivity, dbgen.User{}, ``, http.StatusBadRequest, "Not in a team"},
	}
	for _, tt := range tests {
		w := call(tt.fn, tt.user, "POST", tt.body)
		if w.Code != tt.wantStatus || !strings.Contains(w.Body.String(), tt.wantDetail) {
			t.Errorf("%s: got %d %s, want %d containing %q", tt.name, w.Code, w.Body, tt.wantStatus, tt.wantDetail)
		}
	}
	if notified {
		t.Error("a rejected request notified the scoreboard")
	}
}

func TestInvalidPathID(t *testing.T) {
	h := New(nil, nil, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /teams/{id}/activity", h.teamActivity)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/teams/nope/activity", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

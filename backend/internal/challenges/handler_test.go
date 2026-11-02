package challenges

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database/dbgen"
)

func TestToOutOmitsFlags(t *testing.T) {
	desc := "find it"
	c := dbgen.Challenge{
		ID: uuid.NewV7(), Title: "Warmup", Description: &desc, Points: 100,
		Category: "misc", Difficulty: "easy", Solves: 3, Files: []string{"a.txt"},
		Flags: []byte(`["flag{secret}"]`),
	}
	out := toOut(c)
	if out.ID != c.ID || out.Title != "Warmup" || out.Description != &desc || out.Points != 100 ||
		out.Category != "misc" || out.Difficulty != "easy" || out.Solves != 3 || len(out.Files) != 1 {
		t.Errorf("toOut = %+v", out)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "flag{secret}") {
		t.Errorf("flags leaked: %s", b)
	}
}

// Requests rejected before the store is touched.
func TestInvalidRequests(t *testing.T) {
	h := New(nil, nil, t.TempDir(), nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/challenges/{id}", h.get)
	mux.HandleFunc("GET /api/challenges/{id}/files/{filename}", h.file)
	mux.HandleFunc("GET /api/challenges/{id}/solves", h.solves)
	for _, path := range []string{"/api/challenges/nope", "/api/challenges/nope/files/a.txt", "/api/challenges/nope/solves"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", path, w.Code)
		}
	}

	w := httptest.NewRecorder()
	h.submit(w, httptest.NewRequest("POST", "/api/challenges/submit", strings.NewReader(`{"flag":`)), nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("submit bad body: status = %d, want 422", w.Code)
	}
}

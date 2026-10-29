package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"
)

func TestDecode(t *testing.T) {
	var v struct {
		Name string `json:"name"`
	}
	w := httptest.NewRecorder()
	if !Decode(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`)), &v) {
		t.Fatalf("valid body rejected: %s", w.Body)
	}
	if v.Name != "alice" {
		t.Errorf("Name = %q", v.Name)
	}

	for name, body := range map[string]string{
		"empty":     ``,
		"malformed": `{"name":`,
		"wrongtype": `{"name":1}`,
		"toolarge":  `{"name":"` + strings.Repeat("a", 1<<20) + `"}`,
	} {
		w := httptest.NewRecorder()
		if Decode(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), &v) {
			t.Errorf("%s: accepted", name)
		}
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d", name, w.Code)
		}
	}
}

func TestPathID(t *testing.T) {
	id := uuid.NewV7()
	mux := http.NewServeMux()
	var got uuid.UUID
	var ok bool
	mux.HandleFunc("GET /x/{id}", func(w http.ResponseWriter, r *http.Request) {
		got, ok = PathID(w, r, "id")
	})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/x/"+id.String(), nil))
	if !ok || got != id {
		t.Errorf("PathID = %v, %v; want %v, true", got, ok, id)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/x/nope", nil))
	if ok {
		t.Error("invalid id accepted")
	}
	if w.Code != http.StatusUnprocessableEntity || w.Body.String() != `{"detail":"Invalid id"}` {
		t.Errorf("got %d %s", w.Code, w.Body)
	}
}

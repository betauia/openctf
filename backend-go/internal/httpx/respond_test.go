package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusCreated, map[string]int{"n": 1})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if got := w.Body.String(); got != `{"n":1}` {
		t.Errorf("body = %s", got)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, http.StatusTeapot, "short and stout")
	if w.Code != http.StatusTeapot || w.Body.String() != `{"detail":"short and stout"}` {
		t.Errorf("got %d %s", w.Code, w.Body)
	}
}

func TestInternalErrorHidesErr(t *testing.T) {
	w := httptest.NewRecorder()
	InternalError(w, httptest.NewRequest("GET", "/x", nil), errors.New("db password is hunter2"))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "hunter2") {
		t.Errorf("error leaked into response: %s", w.Body)
	}
}

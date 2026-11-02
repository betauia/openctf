package users

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInvalidPathID(t *testing.T) {
	mux := http.NewServeMux()
	New(nil).Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/users/nope", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewServerSetsTheAddressAndAHeaderTimeout(t *testing.T) {
	srv := NewServer(":9999", http.NotFoundHandler())

	if srv.Addr != ":9999" {
		t.Errorf("Addr = %q, want :9999", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 10s so slow clients can't hold connections open", srv.ReadHeaderTimeout)
	}
	if srv.Handler == nil {
		t.Error("the server has no handler")
	}
}

func TestNewServerPassesRequestsToTheHandlerAndLogsThem(t *testing.T) {
	logs := captureLogs(t)
	srv := NewServer(":0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		io.WriteString(w, "short and stout")
	}))
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/tea", nil))

	if rec.Code != http.StatusTeapot || rec.Body.String() != "short and stout" {
		t.Errorf("got %d %q, want the handler's response untouched", rec.Code, rec.Body)
	}
	for _, want := range []string{"msg=request", "method=GET", "path=/tea", "status=418"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q does not contain %q", logs, want)
		}
	}
}

// A handler that panics must not take the connection down, and the request line shows the 500.
func TestNewServerTurnsAPanicIntoALoggedInternalServerError(t *testing.T) {
	logs := captureLogs(t)
	srv := NewServer(":0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, httptest.NewRequest("POST", "/api/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["detail"] != "Internal Server Error" {
		t.Errorf("body = %q (%v), want the generic error", rec.Body, err)
	}
	for _, want := range []string{"msg=panic", "boom", "msg=request", "path=/api/boom", "status=500"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q does not contain %q", logs, want)
		}
	}
}

package httpserver

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	h := Log(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(interface{ Unwrap() http.ResponseWriter }); !ok {
			t.Error("wrapped writer has no Unwrap")
		}
		w.WriteHeader(http.StatusTeapot)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("DELETE", "/thing", nil))
	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d", w.Code)
	}

	var entry struct {
		Msg    string `json:"msg"`
		Method string `json:"method"`
		Path   string `json:"path"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("log line %q: %v", buf.String(), err)
	}
	if entry.Msg != "request" || entry.Method != "DELETE" || entry.Path != "/thing" || entry.Status != http.StatusTeapot {
		t.Errorf("log entry = %+v", entry)
	}
}

func TestLogDefaultStatus(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// A handler that only writes a body never calls WriteHeader.
	Log(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(buf.String(), `"status":200`) {
		t.Errorf("log = %s, want status 200", buf.String())
	}
}

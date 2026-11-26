package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// captureLogs sends slog output to the returned buffer until the test ends.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func TestChainRunsFirstMiddlewareOutermost(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+" in")
				next.ServeHTTP(w, r)
				order = append(order, name+" out")
			})
		}
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	Chain(h, mark("a"), mark("b"), mark("c")).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	want := []string{"a in", "b in", "c in", "handler", "c out", "b out", "a out"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestChainWithoutMiddlewareReturnsHandler(t *testing.T) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	Chain(h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if !called {
		t.Error("handler was not called")
	}
}

func TestRecoverReturns500WithDetailBody(t *testing.T) {
	captureLogs(t)
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest("GET", "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body["detail"] != "Internal Server Error" {
		t.Errorf("detail = %q, want %q", body["detail"], "Internal Server Error")
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body leaks the panic value: %q", rec.Body.String())
	}
}

func TestRecoverLogsThePanic(t *testing.T) {
	logs := captureLogs(t)
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/boom", nil))

	out := logs.String()
	for _, want := range []string{"panic", "boom", "/boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q does not contain %q", out, want)
		}
	}
}

func TestRecoverPassesThroughWhenNoPanic(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		io.WriteString(w, "ok")
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusTeapot || rec.Body.String() != "ok" {
		t.Errorf("got %d %q, want 418 %q", rec.Code, rec.Body.String(), "ok")
	}
}

// net/http uses http.ErrAbortHandler to abort a response on purpose, so Recover must not turn it into a 500.
func TestRecoverRepanicsErrAbortHandler(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	rec := httptest.NewRecorder()

	var got any
	func() {
		defer func() { got = recover() }()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	}()

	if got != http.ErrAbortHandler {
		t.Errorf("recovered %v, want http.ErrAbortHandler", got)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("wrote a body %q, want none", rec.Body.String())
	}
}

// Log wraps Recover in NewServer, so the request line must show the 500 that Recover wrote.
func TestLogRecordsTheStatusRecoverWrites(t *testing.T) {
	logs := captureLogs(t)
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), Log, Recover)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/boom", nil))

	var line string
	for l := range strings.Lines(logs.String()) {
		if strings.Contains(l, "msg=request") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no request line in logs: %q", logs.String())
	}
	for _, want := range []string{"method=POST", "path=/api/boom", "status=500"} {
		if !strings.Contains(line, want) {
			t.Errorf("request line %q does not contain %q", line, want)
		}
	}
}

func TestLogRecordsStatusAndDefaultsTo200(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"explicit status", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }, "status=404"},
		{"no WriteHeader call", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "hi") }, "status=200"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)

			Log(tc.handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))

			if !strings.Contains(logs.String(), tc.want) {
				t.Errorf("log %q does not contain %q", logs.String(), tc.want)
			}
		})
	}
}

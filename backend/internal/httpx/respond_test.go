package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestJSONWritesTheStatusTypeAndBody(t *testing.T) {
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusCreated, map[string]any{"id": 7, "ok": true})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["id"] != float64(7) || body["ok"] != true {
		t.Errorf("body = %s (%v), want the encoded map", rec.Body, err)
	}
}

func TestJSONEncodesNilAsNullAndEmptySlicesAsLists(t *testing.T) {
	nilRec, emptyRec := httptest.NewRecorder(), httptest.NewRecorder()

	JSON(nilRec, http.StatusOK, nil)
	JSON(emptyRec, http.StatusOK, []string{})

	if got := strings.TrimSpace(nilRec.Body.String()); got != "null" {
		t.Errorf("nil encodes as %q, want null", got)
	}
	if got := strings.TrimSpace(emptyRec.Body.String()); got != "[]" {
		t.Errorf("an empty slice encodes as %q, want []", got)
	}
}

// The status line is already sent when encoding fails, so all JSON can do is log it.
func TestJSONLogsAnEncodingFailure(t *testing.T) {
	logs := captureLogs(t)
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusOK, map[string]any{"bad": make(chan int)})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want the status that was already written", rec.Code)
	}
	if !strings.Contains(logs.String(), "encode response") {
		t.Errorf("log %q does not mention the encoding failure", logs)
	}
}

func TestErrorWritesTheDetailShape(t *testing.T) {
	rec := httptest.NewRecorder()

	Error(rec, http.StatusTeapot, `Not "allowed" <here>`)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body, err)
	}
	if len(body) != 1 || body["detail"] != `Not "allowed" <here>` {
		t.Errorf("body = %v, want only the detail, unmodified", body)
	}
}

func TestInternalErrorLogsTheCauseButNeverSendsIt(t *testing.T) {
	logs := captureLogs(t)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/secret-path", nil)

	InternalError(rec, r, errors.New("connection to db-host-17 refused"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "db-host-17") {
		t.Errorf("the response leaks the cause: %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"Internal Server Error"`) {
		t.Errorf("body = %s, want the generic detail", rec.Body)
	}
	for _, want := range []string{"method=PUT", "path=/api/secret-path", "db-host-17"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q does not contain %q", logs, want)
		}
	}
}

package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func get(pool *pgxpool.Pool, method string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	New(pool).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, "/health", nil))
	return rec
}

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %q is not a JSON object of strings: %v", rec.Body, err)
	}
	return out
}

func TestHealthWhenTheDatabaseIsUp(t *testing.T) {
	rec := get(testdb.Pool, "GET")

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	got := body(t, rec)
	if len(got) != 2 || got["status"] != "ok" || got["database"] != "up" {
		t.Errorf("body = %v, want status ok and database up", got)
	}
}

func TestHealthWhenTheDatabaseIsDown(t *testing.T) {
	// A second pool, closed, stands in for a database that can't be reached.
	pool, err := pgxpool.New(context.Background(), testdb.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	start := time.Now()

	rec := get(pool, "GET")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	got := body(t, rec)
	if len(got) != 2 || got["status"] != "unavailable" || got["database"] != "down" {
		t.Errorf("body = %v, want status unavailable and database down", got)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("took %v, want it to fail within the 2 second check timeout", elapsed)
	}
}

func TestHealthDoesNotNeedALogin(t *testing.T) {
	// get sends no cookies at all, and the route is not behind any authorisation.
	if rec := get(testdb.Pool, "GET"); rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Errorf("status = %d, want the health check to be public", rec.Code)
	}
}

func TestHealthOnlyAnswersGET(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		if rec := get(testdb.Pool, method); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, rec.Code)
		}
	}
}

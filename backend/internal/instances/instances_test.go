package instances

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/moby/moby/api/types/container"

	"github.com/betauia/openctf/backend/internal/database/dbgen"
)

func TestShortID(t *testing.T) {
	for in, want := range map[string]string{
		"":                   "",
		"abc":                "abc",
		"0123456789ab":       "0123456789ab",
		"0123456789abcdef01": "0123456789ab",
	} {
		if got := shortID(in); got != want {
			t.Errorf("shortID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromInspect(t *testing.T) {
	chID := uuid.NewV7()
	got := fromInspect(container.InspectResponse{
		ID:      "0123456789abcdef",
		Name:    "/web-1",
		Created: "2026-01-02T03:04:05Z",
		State:   &container.State{Status: "running"},
		Config: &container.Config{
			Image:  "nginx",
			Labels: map[string]string{challengeLabel: "true", challengeIDLabel: chID.String()},
		},
	})
	if got.ID != "0123456789ab" || got.Name != "web-1" || got.Image != "nginx" || got.Status != "running" ||
		got.CreatedAt != "2026-01-02T03:04:05Z" || got.Provider != "docker" {
		t.Errorf("fromInspect = %+v", got)
	}
	if got.ChallengeID == nil || *got.ChallengeID != chID {
		t.Errorf("ChallengeID = %v, want %v", got.ChallengeID, chID)
	}
}

func TestFromInspectMissingFields(t *testing.T) {
	got := fromInspect(container.InspectResponse{ID: "abc"})
	if got.ID != "abc" || got.Image != "" || got.Status != "" || got.ChallengeID != nil {
		t.Errorf("fromInspect = %+v", got)
	}

	got = fromInspect(container.InspectResponse{Config: &container.Config{Labels: map[string]string{challengeIDLabel: "garbage"}}})
	if got.ChallengeID != nil {
		t.Errorf("ChallengeID = %v for unparseable label", got.ChallengeID)
	}
}

func TestNewProvider(t *testing.T) {
	p, err := NewProvider("kubernetes", "localhost")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(Kubernetes); !ok {
		t.Errorf("NewProvider(kubernetes) = %T", p)
	}
	if _, err := NewProvider("podman", "localhost"); err == nil {
		t.Error("unknown provider accepted")
	}
}

func TestKubernetesNotImplemented(t *testing.T) {
	ctx := context.Background()
	var k Kubernetes
	_, createErr := k.Create(ctx, uuid.NewV7(), "nginx", 80)
	_, listErr := k.List(ctx)
	_, getErr := k.Get(ctx, "x")
	deleteErr := k.Delete(ctx, "x")
	for name, err := range map[string]error{"Create": createErr, "List": listErr, "Get": getErr, "Delete": deleteErr} {
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("%s err = %v, want ErrNotImplemented", name, err)
		}
	}
}

// fakeProvider returns err from every call, or canned data when err is nil.
type fakeProvider struct {
	Kubernetes
	err       error
	list      []Instance
	gotID     string
	deletedID string
}

func (f *fakeProvider) List(context.Context) ([]Instance, error) { return f.list, f.err }

func (f *fakeProvider) Get(_ context.Context, id string) (Instance, error) {
	f.gotID = id
	return Instance{ID: id, Provider: "fake"}, f.err
}

func (f *fakeProvider) Delete(_ context.Context, id string) error {
	f.deletedID = id
	return f.err
}

// serve routes req through a mux so path values are set, calling the handler as a logged-in user.
func serve(pattern string, fn func(http.ResponseWriter, *http.Request, dbgen.User), req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) { fn(w, r, dbgen.User{}) })
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestHandlerList(t *testing.T) {
	h := NewHandler(nil, nil, &fakeProvider{})
	w := serve("GET /api/admin/instances", h.list, httptest.NewRequest("GET", "/api/admin/instances", nil))
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("empty list: %d %s", w.Code, w.Body)
	}

	h = NewHandler(nil, nil, &fakeProvider{list: []Instance{{ID: "a"}, {ID: "b"}}})
	w = serve("GET /api/admin/instances", h.list, httptest.NewRequest("GET", "/api/admin/instances", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"id":"b"`) {
		t.Errorf("list: %d %s", w.Code, w.Body)
	}
}

func TestHandlerGetAndDelete(t *testing.T) {
	p := &fakeProvider{}
	h := NewHandler(nil, nil, p)

	w := serve("GET /api/admin/instances/{id}", h.get, httptest.NewRequest("GET", "/api/admin/instances/abc", nil))
	if w.Code != http.StatusOK || p.gotID != "abc" || !strings.Contains(w.Body.String(), `"id":"abc"`) {
		t.Errorf("get: %d %s (id %q)", w.Code, w.Body, p.gotID)
	}

	w = serve("DELETE /api/admin/instances/{id}", h.delete, httptest.NewRequest("DELETE", "/api/admin/instances/abc", nil))
	if w.Code != http.StatusOK || p.deletedID != "abc" {
		t.Errorf("delete: %d %s (id %q)", w.Code, w.Body, p.deletedID)
	}
}

func TestProviderErrors(t *testing.T) {
	tests := []struct {
		err        error
		wantStatus int
		wantDetail string
	}{
		{ErrNotFound, http.StatusNotFound, "Instance not found"},
		{ErrNotImplemented, http.StatusNotImplemented, ErrNotImplemented.Error()},
		{errors.New("docker daemon on fire"), http.StatusInternalServerError, "Error fetching instance"},
	}
	for _, tt := range tests {
		h := NewHandler(nil, nil, &fakeProvider{err: tt.err})
		w := serve("GET /api/admin/instances/{id}", h.get, httptest.NewRequest("GET", "/api/admin/instances/x", nil))
		if w.Code != tt.wantStatus || !strings.Contains(w.Body.String(), tt.wantDetail) {
			t.Errorf("%v: got %d %s, want %d %q", tt.err, w.Code, w.Body, tt.wantStatus, tt.wantDetail)
		}
		if strings.Contains(w.Body.String(), "on fire") {
			t.Errorf("internal error leaked: %s", w.Body)
		}
	}
}

func TestHandlerCreateBadChallengeID(t *testing.T) {
	h := NewHandler(nil, nil, &fakeProvider{})
	w := serve("POST /api/challenges/{cid}/instance", h.create, httptest.NewRequest("POST", "/api/challenges/nope/instance", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d", w.Code)
	}
}

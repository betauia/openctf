package instances

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// Handler serves the challenge instance endpoints.
type Handler struct {
	store    *database.Store
	auth     *auth.Auth
	provider Provider
}

// NewHandler returns a Handler that looks up challenges in store and runs them with p.
func NewHandler(store *database.Store, a *auth.Auth, p Provider) *Handler {
	return &Handler{store: store, auth: a, provider: p}
}

// Register adds the instance routes to mux: starting a challenge's instance, and
// the admin routes for listing, inspecting and killing any instance. All of them
// require a logged-in user.
func (h *Handler) Register(mux *http.ServeMux) {
	req := h.auth.Require
	mux.HandleFunc("POST /api/challenges/{cid}/instance", req(h.create))
	mux.HandleFunc("GET /api/admin/instances", req(h.list))
	mux.HandleFunc("GET /api/admin/instances/{id}", req(h.get))
	mux.HandleFunc("DELETE /api/admin/instances/{id}", req(h.delete))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, _ dbgen.User) {
	cid, ok := httpx.PathID(w, r, "cid")
	if !ok {
		return
	}
	c, err := h.store.GetVisibleChallenge(r.Context(), cid)
	if err != nil && !database.IsNotFound(err) {
		httpx.InternalError(w, r, err)
		return
	}
	if err != nil || c.DockerImage == nil || *c.DockerImage == "" {
		httpx.Error(w, http.StatusNotFound, "Challenge has no docker image")
		return
	}
	port := 80
	if c.DockerPort != nil {
		port = int(*c.DockerPort)
	}
	inst, err := h.provider.Create(r.Context(), c.ID, *c.DockerImage, port)
	if err != nil {
		h.providerError(w, r, err, "Failed to spawn instance")
		return
	}
	httpx.JSON(w, http.StatusOK, inst)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, _ dbgen.User) {
	insts, err := h.provider.List(r.Context())
	if err != nil {
		h.providerError(w, r, err, "Error fetching instances")
		return
	}
	if insts == nil {
		insts = []Instance{}
	}
	httpx.JSON(w, http.StatusOK, insts)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, _ dbgen.User) {
	inst, err := h.provider.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.providerError(w, r, err, "Error fetching instance")
		return
	}
	httpx.JSON(w, http.StatusOK, inst)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, _ dbgen.User) {
	if err := h.provider.Delete(r.Context(), r.PathValue("id")); err != nil {
		h.providerError(w, r, err, "Failed to delete instance")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"detail": "Instance deleted successfully"})
}

// providerError maps provider errors to responses; anything unexpected is logged and reported with msg.
func (h *Handler) providerError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "Instance not found")
	case errors.Is(err, ErrNotImplemented):
		httpx.Error(w, http.StatusNotImplemented, err.Error())
	default:
		slog.Error("instance provider", "method", r.Method, "path", r.URL.Path, "err", err)
		httpx.Error(w, http.StatusInternalServerError, msg)
	}
}

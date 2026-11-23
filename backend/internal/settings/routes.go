package settings

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", h.publicSettings)
	mux.HandleFunc("GET /api/admin/settings", h.a.Authorise(policy.SettingsRead, h.getSettings))
	mux.HandleFunc("PUT /api/admin/settings", h.a.Authorise(policy.SettingsUpdate, h.updateSettings))
}

func (h *Handler) publicSettings(w http.ResponseWriter, r *http.Request) {
	s, err := load(r.Context(), h.pool)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.Theme = h.theme.Load().(string)
	s.Rules = h.rules.Load().(string)
	httpx.JSON(w, http.StatusOK, s)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request, u *auth.User) {
	s, err := load(r.Context(), h.pool)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.Theme = h.theme.Load().(string)
	s.Rules = h.rules.Load().(string)
	httpx.JSON(w, http.StatusOK, s)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var body struct {
		CTFName      *string    `json:"ctf_name"`
		CTFStart     *time.Time `json:"ctf_start"`
		CTFEnd       *time.Time `json:"ctf_end"`
		Registration *bool      `json:"registration"`
		MaxTeamSize  *int32     `json:"max_team_size"`
		Rules        *string    `json:"rules"`
		Theme        *string    `json:"theme"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if body.CTFName != nil {
		name := strings.TrimSpace(*body.CTFName)
		if name == "" || len(name) > 64 {
			httpx.Error(w, http.StatusBadRequest, "CTF name must be 1-64 characters")
			return
		}
		body.CTFName = &name
	}
	if body.Theme != nil && !slices.Contains(Themes, *body.Theme) {
		httpx.Error(w, http.StatusBadRequest, "Unknown theme")
		return
	}
	if body.MaxTeamSize != nil && *body.MaxTeamSize < 1 {
		httpx.Error(w, http.StatusBadRequest, "Max team size must be at least 1")
		return
	}

	ctx := r.Context()
	var s Settings
	var badWindow bool
	err := pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		// Lock the row so the start/end check sees the values it is checked against.
		q := dbgen.New(tx)
		window, err := q.LockSettingsWindow(ctx)
		if err != nil {
			return err
		}
		start, end := window.CtfStart, window.CtfEnd
		if body.CTFStart != nil {
			start = *body.CTFStart
		}
		if body.CTFEnd != nil {
			end = *body.CTFEnd
		}
		if !start.Before(end) {
			badWindow = true
			return nil
		}
		err = q.UpdateSettings(ctx, dbgen.UpdateSettingsParams{
			CtfName:      body.CTFName,
			CtfStart:     start,
			CtfEnd:       end,
			Registration: body.Registration,
			MaxTeamSize:  body.MaxTeamSize,
		})
		if err != nil {
			return err
		}
		s, err = load(ctx, tx)
		return err
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if badWindow {
		httpx.Error(w, http.StatusBadRequest, "CTF start must be before CTF end")
		return
	}
	if body.Theme != nil {
		h.theme.Store(*body.Theme)
	}
	if body.Rules != nil {
		h.rules.Store(*body.Rules)
	}
	s.Theme = h.theme.Load().(string)
	s.Rules = h.rules.Load().(string)
	httpx.JSON(w, http.StatusOK, s)
}

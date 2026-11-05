package settings

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/httpx"
)

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/settings", h.a.Authorise(authz.SettingsRead, h.getSettings))
	mux.HandleFunc("PUT /api/admin/settings", h.a.Authorise(authz.SettingsUpdate, h.updateSettings))
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request, u *auth.User) {
	s, err := load(r.Context(), h.pool)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var body struct {
		CTFName      *string    `json:"ctf_name"`
		CTFStart     *time.Time `json:"ctf_start"`
		CTFEnd       *time.Time `json:"ctf_end"`
		Registration *bool      `json:"registration"`
		MaxTeamSize  *int       `json:"max_team_size"`
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
	if body.MaxTeamSize != nil && *body.MaxTeamSize < 1 {
		httpx.Error(w, http.StatusBadRequest, "Max team size must be at least 1")
		return
	}

	ctx := r.Context()
	var s Settings
	var badWindow bool
	err := pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		// Lock the row so the start/end check sees the values it is checked against.
		var start, end time.Time
		if err := tx.QueryRow(ctx, `SELECT ctf_start, ctf_end FROM settings FOR UPDATE`).Scan(&start, &end); err != nil {
			return err
		}
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
		_, err := tx.Exec(ctx, `
			UPDATE settings SET
				ctf_name      = COALESCE($1, ctf_name),
				ctf_start     = $2,
				ctf_end       = $3,
				registration  = COALESCE($4, registration),
				max_team_size = COALESCE($5, max_team_size)`,
			body.CTFName, start, end, body.Registration, body.MaxTeamSize)
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
	httpx.JSON(w, http.StatusOK, s)
}

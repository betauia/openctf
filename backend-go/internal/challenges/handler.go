// Package challenges serves the /api/challenges endpoints: listing, files, solves and flag submission.
package challenges

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// Handler serves the /api/challenges endpoints.
type Handler struct {
	store     *database.Store
	auth      *auth.Auth
	uploadDir string
	onSolve   func()
}

// New returns the challenges handler. onSolve is called after each new solve is recorded.
func New(store *database.Store, a *auth.Auth, uploadDir string, onSolve func()) *Handler {
	return &Handler{store: store, auth: a, uploadDir: uploadDir, onSolve: onSolve}
}

// Register adds the /api/challenges routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/challenges", h.list)
	mux.HandleFunc("GET /api/challenges/{id}", h.get)
	mux.HandleFunc("GET /api/challenges/{id}/files/{filename}", h.file)
	mux.HandleFunc("GET /api/challenges/{id}/solves", h.solves)
	mux.HandleFunc("POST /api/challenges/submit", h.auth.Optional(h.submit))
}

type challengeOut struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Description    *string   `json:"description"`
	Author         *string   `json:"author"`
	Points         int32     `json:"points"`
	Category       string    `json:"category"`
	Difficulty     string    `json:"difficulty"`
	Solves         int32     `json:"solves"`
	ConnectionInfo *string   `json:"connection_info"`
	Files          []string  `json:"files"`
	DockerImage    *string   `json:"docker_image"`
	DockerPort     *int32    `json:"docker_port"`
}

func toOut(c dbgen.Challenge) challengeOut {
	return challengeOut{
		ID: c.ID, Title: c.Title, Description: c.Description, Author: c.Author,
		Points: c.Points, Category: c.Category, Difficulty: c.Difficulty, Solves: c.Solves,
		ConnectionInfo: c.ConnectionInfo, Files: c.Files,
		DockerImage: c.DockerImage, DockerPort: c.DockerPort,
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	cs, err := h.store.ListVisibleChallenges(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	out := make([]challengeOut, len(cs))
	for i, c := range cs {
		out[i] = toOut(c)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// visible loads the visible challenge named by the {id} path value, writing an error response if there is none.
func (h *Handler) visible(w http.ResponseWriter, r *http.Request) (dbgen.Challenge, bool) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return dbgen.Challenge{}, false
	}
	c, err := h.store.GetVisibleChallenge(r.Context(), id)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "Challenge not found")
		return c, false
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return c, false
	}
	return c, true
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	if c, ok := h.visible(w, r); ok {
		httpx.JSON(w, http.StatusOK, toOut(c))
	}
}

func (h *Handler) file(w http.ResponseWriter, r *http.Request) {
	c, ok := h.visible(w, r)
	if !ok {
		return
	}
	name := r.PathValue("filename")
	// Only bare filenames listed on the challenge may be served from the upload dir.
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || !slices.Contains(c.Files, name) {
		httpx.Error(w, http.StatusNotFound, "File not in challenge")
		return
	}
	f, err := os.Open(filepath.Join(h.uploadDir, name))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "File not found on server")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		httpx.Error(w, http.StatusNotFound, "File not found on server")
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func (h *Handler) solves(w http.ResponseWriter, r *http.Request) {
	c, ok := h.visible(w, r)
	if !ok {
		return
	}
	rows, err := h.store.ListChallengeSolves(r.Context(), c.ID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	type solveOut struct {
		UID      uuid.UUID `json:"uid"`
		Username string    `json:"username"`
		Team     *string   `json:"team"`
		SolvedAt time.Time `json:"solved_at"`
	}
	out := make([]solveOut, len(rows))
	for i, s := range rows {
		out[i] = solveOut{s.Uid, s.Username, s.Team, s.SolvedAt}
	}
	httpx.JSON(w, http.StatusOK, out)
}

// submit checks a flag against every visible challenge, as the frontend doesn't send a challenge id.
func (h *Handler) submit(w http.ResponseWriter, r *http.Request, u *dbgen.User) {
	var body struct {
		Flag string `json:"flag"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	cs, err := h.store.ListVisibleChallenges(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	i := slices.IndexFunc(cs, func(c dbgen.Challenge) bool { return CheckFlag(body.Flag, c.Flags) })
	if i < 0 {
		httpx.JSON(w, http.StatusOK, map[string]bool{"correct": false})
		return
	}
	c := cs[i]

	if u == nil {
		// Same as the Python backend: anonymous correct submissions still bump the solve count.
		err = h.store.IncrementChallengeSolves(ctx, c.ID)
	} else {
		var solved bool
		err = h.store.InTx(ctx, func(q *dbgen.Queries) error {
			n, err := q.CreateSolve(ctx, dbgen.CreateSolveParams{UserID: u.ID, ChallengeID: c.ID})
			if err != nil || n == 0 {
				return err
			}
			solved = true
			if err := q.AddUserScore(ctx, dbgen.AddUserScoreParams{ID: u.ID, Points: c.Points}); err != nil {
				return err
			}
			return q.IncrementChallengeSolves(ctx, c.ID)
		})
		if err == nil && solved && h.onSolve != nil {
			h.onSolve()
		}
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"correct": true, "challenge": c.Title, "points": c.Points})
}

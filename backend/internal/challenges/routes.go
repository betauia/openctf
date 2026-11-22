package challenges

import (
	"context"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

func (h *Handler) Register(mux *http.ServeMux) {
	// Player
	mux.HandleFunc("GET /api/challenges", h.a.Optional(h.getChallenges))
	mux.HandleFunc("GET /api/challenges/{id}", h.a.Optional(h.getChallenge))
	mux.HandleFunc("GET /api/challenges/{id}/solves", h.a.Optional(h.viewSolves))
	mux.HandleFunc("GET /api/challenges/{id}/files", h.a.Optional(h.getFiles))
	mux.HandleFunc("GET /api/challenges/{id}/files/{fid}", h.a.Optional(h.getFile))
	mux.HandleFunc("POST /api/challenges/submit", h.a.Optional(h.submitFlag))

	// Admin
	mux.HandleFunc("GET /api/admin/challenges", h.a.Optional(h.adminGetChallenges))
	mux.HandleFunc("GET /api/admin/challenges/{id}", h.a.Optional(h.adminGetChallenge))
	mux.HandleFunc("GET /api/admin/challenges/{id}/files", h.a.Optional(h.adminGetFiles))
	mux.HandleFunc("GET /api/admin/challenges/{id}/files/{fid}", h.a.Optional(h.adminGetFile))
}

// Player Functions
// Can potentially add filtering by category, depending on what Frontend developer wants
func (h *Handler) getChallenges(w http.ResponseWriter, r *http.Request, u *auth.User) {
	challenges, ok := h.allowedChallenges(w, r, u, policy.ChallengeRead, false)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, challenges)
}

func (h *Handler) getChallenge(w http.ResponseWriter, r *http.Request, u *auth.User) {
	challenge, ok := h.allowedChallenge(w, r, u, policy.ChallengeRead)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, challenge)
}

func (h *Handler) viewSolves(w http.ResponseWriter, r *http.Request, u *auth.User) {
	c, ok := h.allowedChallenge(w, r, u, policy.ChallengeViewSolves)
	if !ok {
		return
	}
	solves, err := listSolves(r.Context(), h.pool, c.ID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, solves)
}
func (h *Handler) getFile(w http.ResponseWriter, r *http.Request, u *auth.User) {
	c, ok := h.allowedChallenge(w, r, u, policy.ChallengeDownload)
	if !ok {
		return
	}
	cid := c.ID
	fid, ok := httpx.PathID(w, r, "fid")
	if !ok {
		return
	}
	f, err := getChallengeFile(r.Context(), h.pool, cid, fid, false)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}

	root, err := os.OpenRoot(h.filesDir)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	defer root.Close()

	file, err := root.Open(f.Filepath)
	if errors.Is(err, fs.ErrNotExist) {
		httpx.Error(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if info.IsDir() {
		httpx.Error(w, http.StatusNotFound, "File not found")
		return
	}

	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Filename}))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, f.Filename, info.ModTime(), file)
}

func (h *Handler) getFiles(w http.ResponseWriter, r *http.Request, u *auth.User) {
	c, ok := h.allowedChallenge(w, r, u, policy.ChallengeDownload)
	if !ok {
		return
	}
	files, err := getChallengeFiles(r.Context(), h.pool, c.ID, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, files)
}

func (h *Handler) submitFlag(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if u == nil {
		auth.Deny(w, u)
		return
	}
	var body struct {
		Flag        string     `json:"flag"`
		ChallengeID *uuid.UUID `json:"challenge_id"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	flag := strings.TrimSpace(body.Flag)
	if flag == "" {
		httpx.Error(w, http.StatusBadRequest, "Flag is required")
		return
	}

	ctx := r.Context()
	c, err := findChallengeByFlag(ctx, h.pool, flag, body.ChallengeID)
	if database.IsNotFound(err) {
		httpx.JSON(w, http.StatusOK, submitResult{Correct: false})
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	allowed, err := h.a.IsAllowed(r, u, policy.ChallengeSubmit, c.resource())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		auth.Deny(w, u)
		return
	}

	var points int
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		solved, err := recordSolve(ctx, tx, u.ID, c.ID, u.TeamID)
		if err != nil || !solved {
			return err
		}
		points = c.Points
		if _, err := tx.Exec(ctx, `UPDATE challenges SET solves = solves + 1 WHERE id = $1`, c.ID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE users SET score = score + $1 WHERE id = $2`, c.Points, u.ID)
		return err
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if points > 0 {
		h.publishSolve(ctx, c, u)
	}
	httpx.JSON(w, http.StatusOK, submitResult{Correct: true, ChallengeID: &c.ID, Points: points})
}

func (h *Handler) publishSolve(ctx context.Context, c challenge, u *auth.User) {
	var team *string
	if u.TeamID != nil {
		_ = h.pool.QueryRow(ctx, `SELECT name FROM teams WHERE id = $1 AND NOT hidden`, *u.TeamID).Scan(&team)
	}
	h.publish("solve", solveEvent{
		ChallengeID: c.ID,
		Challenge:   c.Title,
		Category:    c.Category,
		Points:      c.Points,
		UserID:      u.ID,
		Username:    u.Username,
		TeamID:      u.TeamID,
		Team:        team,
		SolvedAt:    time.Now(),
	})
}

// Admin Functions
func (h *Handler) adminGetChallenges(w http.ResponseWriter, r *http.Request, u *auth.User) {
	challenges, ok := h.allowedChallenges(w, r, u, policy.ChallengeReadSecrets, true)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, challenges)
}
func (h *Handler) adminGetChallenge(w http.ResponseWriter, r *http.Request, u *auth.User) {
	challenge, ok := h.allowedChallenge(w, r, u, policy.ChallengeReadSecrets)
	if !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, challenge)
}
func (h *Handler) adminGetFile(w http.ResponseWriter, r *http.Request, u *auth.User) {
	c, ok := h.allowedChallenge(w, r, u, policy.ChallengeManageFiles)
	if !ok {
		return
	}
	cid := c.ID
	fid, ok := httpx.PathID(w, r, "fid")
	if !ok {
		return
	}
	f, err := getChallengeFile(r.Context(), h.pool, cid, fid, true)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}

	root, err := os.OpenRoot(h.filesDir)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	defer root.Close()

	file, err := root.Open(f.Filepath)
	if errors.Is(err, fs.ErrNotExist) {
		httpx.Error(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if info.IsDir() {
		httpx.Error(w, http.StatusNotFound, "File not found")
		return
	}

	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Filename}))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, f.Filename, info.ModTime(), file)
}

func (h *Handler) adminGetFiles(w http.ResponseWriter, r *http.Request, u *auth.User) {
	c, ok := h.allowedChallenge(w, r, u, policy.ChallengeManageFiles)
	if !ok {
		return
	}
	files, err := getChallengeFiles(r.Context(), h.pool, c.ID, true)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, files)
}

func (h *Handler) allowedChallenges(w http.ResponseWriter, r *http.Request, u *auth.User, action string, includeHidden bool) ([]challenge, bool) {
	if u == nil {
		auth.Deny(w, u)
		return nil, false
	}
	challenges, err := listChallenges(r.Context(), h.pool, includeHidden)
	if err != nil {
		httpx.InternalError(w, r, err)
		return nil, false
	}
	check, err := h.a.Checker(r, u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return nil, false
	}
	allowed := []challenge{}
	for _, c := range challenges {
		if check(action, c.resource()) {
			allowed = append(allowed, c)
		}
	}
	return allowed, true
}

func (h *Handler) allowedChallenge(w http.ResponseWriter, r *http.Request, u *auth.User, action string) (challenge, bool) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return challenge{}, false
	}
	c, err := listChallengeByID(r.Context(), h.pool, id, true)
	if err != nil {
		auth.Fail(w, r, u, err, "Challenge not found")
		return challenge{}, false
	}
	allowed, err := h.a.IsAllowed(r, u, action, c.resource())
	if err != nil {
		httpx.InternalError(w, r, err)
		return challenge{}, false
	}
	if !allowed {
		if !c.IsVisible && u != nil {
			httpx.Error(w, http.StatusNotFound, "Challenge not found")
		} else {
			auth.Deny(w, u)
		}
		return challenge{}, false
	}
	return c, true
}

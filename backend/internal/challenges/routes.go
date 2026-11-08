package challenges

import (
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
)

func (h *Handler) Register(mux *http.ServeMux) {
	// Player
	mux.HandleFunc("GET /api/challenges", h.a.Authorise(authz.ChallengeRead, h.getChallenges))
	mux.HandleFunc("GET /api/challenges/{id}", h.a.Authorise(authz.ChallengeRead, h.getChallenge))
	mux.HandleFunc("GET /api/challenges/{id}/solves", h.a.Authorise(authz.ChallengeViewSolves, h.viewSolves))
	mux.HandleFunc("GET /api/challenges/{id}/files", h.a.Authorise(authz.ChallengeDownload, h.getFiles))
	mux.HandleFunc("GET /api/challenges/{id}/files/{fid}", h.a.Authorise(authz.ChallengeDownload, h.getFile))
	mux.HandleFunc("POST /api/challenges/submit", h.a.Authorise(authz.ChallengeSubmit, h.submitFlag))

	// Admin
	mux.HandleFunc("GET /api/admin/challenges", h.a.Authorise(authz.ChallengeReadSecrets, h.adminGetChallenges))
	mux.HandleFunc("GET /api/admin/challenges/{id}", h.a.Authorise(authz.ChallengeReadSecrets, h.adminGetChallenge))
	mux.HandleFunc("GET /api/admin/challenges/{id}/files", h.a.Authorise(authz.ChallengeManageFiles, h.adminGetFiles))
	mux.HandleFunc("GET /api/admin/challenges/{id}/files/{fid}", h.a.Authorise(authz.ChallengeManageFiles, h.adminGetFile))
}

// Player Functions
// Can potentially add filtering by category, depending on what Frontend developer wants
func (h *Handler) getChallenges(w http.ResponseWriter, r *http.Request, u *auth.User) {
	challenges, err := listChallenges(r.Context(), h.pool, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, challenges)
}

func (h *Handler) getChallenge(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	challenge, err := listChallengeByID(r.Context(), h.pool, id, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, challenge)
}

func (h *Handler) viewSolves(w http.ResponseWriter, r *http.Request, u *auth.User) {

}
func (h *Handler) getFile(w http.ResponseWriter, r *http.Request, u *auth.User) {
	cid, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
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
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	files, err := getChallengeFiles(r.Context(), h.pool, id, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, files)
}

func (h *Handler) submitFlag(w http.ResponseWriter, r *http.Request, u *auth.User) {

}

// Admin Functions
func (h *Handler) adminGetChallenges(w http.ResponseWriter, r *http.Request, u *auth.User) {
	challenges, err := listChallenges(r.Context(), h.pool, true)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, challenges)
}
func (h *Handler) adminGetChallenge(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	challenge, err := listChallengeByID(r.Context(), h.pool, id, true)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, challenge)
}
func (h *Handler) adminGetFile(w http.ResponseWriter, r *http.Request, u *auth.User) {
	cid, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
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
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	files, err := getChallengeFiles(r.Context(), h.pool, id, true)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, files)
}

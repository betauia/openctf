// Package auth handles session and refresh cookies and the /api/auth endpoints.
// Password hashing lives in auth/passwords and token generation in auth/sessions.
package auth

import (
	"context"
	"net/http"
	"net/mail"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/auth/passwords"
	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

const (
	cookieName        = "session"
	refreshCookieName = "refresh"
	refreshPath       = "/api/auth"
)

// Auth authenticates requests with signed session cookies and serves the /api/auth endpoints.
type Auth struct {
	pool     *pgxpool.Pool
	secret   []byte
	policies *policy.Engine
}

// New returns an Auth that loads users from pool, signs session tokens with secretKey and
// authorises requests with policies.
func New(pool *pgxpool.Pool, secretKey string, policies *policy.Engine) *Auth {
	return &Auth{pool: pool, secret: []byte(secretKey), policies: policies}
}

// Register adds the /api/auth routes to mux.
func (a *Auth) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/refresh", a.refresh)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/auth/me", a.Require(a.me))
	mux.HandleFunc("PUT /api/auth/password", a.Require(a.changePassword))
	mux.HandleFunc("GET /api/auth/solves", a.Optional(a.solves))
}

type userRef struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

func (a *Auth) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if n := utf8.RuneCountInString(body.Username); n < 5 || n > 32 {
		httpx.Error(w, http.StatusBadRequest, "Username must be 5-32 characters")
		return
	}
	if utf8.RuneCountInString(body.Password) < 12 {
		httpx.Error(w, http.StatusBadRequest, "Password must be at least 12 characters")
		return
	}
	if n := utf8.RuneCountInString(body.Email); n < 5 || n > 255 {
		httpx.Error(w, http.StatusBadRequest, "Email must be 5-255 characters")
		return
	}
	if _, err := mail.ParseAddress(body.Email); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Please provide a valid email address")
		return
	}
	ctx := r.Context()
	taken, err := userExists(ctx, a.pool, body.Username, body.Email)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if taken {
		httpx.Error(w, http.StatusBadRequest, "Username or email already taken")
		return
	}
	hash, err := passwords.Hash(body.Password)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	u, err := createUser(ctx, a.pool, uuid.NewV7(), body.Username, body.Email, hash)
	if database.IsUniqueViolation(err) { // lost a race with another registration
		httpx.Error(w, http.StatusBadRequest, "Username or email already taken")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if err := a.startSession(w, r, u.ID); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, userRef{ID: u.ID, Username: u.Username})
}

func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	u, err := getUserByUsername(r.Context(), a.pool, body.Username)
	if err != nil && !database.IsNotFound(err) {
		httpx.InternalError(w, r, err)
		return
	}
	if err != nil || !passwords.Verify(body.Password, u.PasswordHash) {
		httpx.Error(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}
	if err := a.startSession(w, r, u.ID); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, userRef{ID: u.ID, Username: u.Username})
}

// refresh exchanges the refresh cookie for a new session and a rotated refresh token.
func (a *Auth) refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Not logged in")
		return
	}
	ctx := r.Context()
	hash := sessions.HashRefreshToken(c.Value)
	var userID uuid.UUID
	var token string
	err = pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		uid, family, err := consumeRefreshToken(ctx, tx, hash, time.Now().Add(-sessions.RefreshReuseWindow))
		if err != nil {
			return err
		}
		userID = uid
		token, err = newRefreshToken(ctx, tx, uid, family)
		return err
	})
	if database.IsNotFound(err) {
		// Unknown, expired, revoked, or replayed after the reuse window. A replay means the
		// token may have been stolen, so revoke every token rotated from the same login.
		if family, err := getRefreshTokenFamily(ctx, a.pool, hash); err == nil {
			if err := revokeRefreshFamily(ctx, a.pool, family); err != nil {
				httpx.InternalError(w, r, err)
				return
			}
		}
		a.clearSession(w)
		httpx.Error(w, http.StatusUnauthorized, "Session expired")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	a.setSession(w, userID)
	a.setRefreshCookie(w, token)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(refreshCookieName); err == nil {
		ctx := r.Context()
		family, err := getRefreshTokenFamily(ctx, a.pool, sessions.HashRefreshToken(c.Value))
		if err == nil {
			err = revokeRefreshFamily(ctx, a.pool, family)
		}
		if err != nil && !database.IsNotFound(err) {
			httpx.InternalError(w, r, err)
			return
		}
	}
	a.clearSession(w)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *Auth) me(w http.ResponseWriter, r *http.Request, u User) {
	rank, err := userRank(r.Context(), a.pool, u.Score)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, struct {
		ID       uuid.UUID `json:"id"`
		Username string    `json:"username"`
		Score    int32     `json:"score"`
		Role     string    `json:"role"`
		Rank     int32     `json:"rank"`
	}{u.ID, u.Username, u.Score, u.Role, rank})
}

// changePassword replaces the user's password after checking the current one. Every other
// device is logged out: their refresh tokens are revoked, and their session cookies expire
// within sessions.MaxAge. The caller gets a fresh session.
func (a *Auth) changePassword(w http.ResponseWriter, r *http.Request, u User) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if !passwords.Verify(body.CurrentPassword, u.PasswordHash) {
		httpx.Error(w, http.StatusBadRequest, "Current password is incorrect")
		return
	}
	if utf8.RuneCountInString(body.NewPassword) < 12 {
		httpx.Error(w, http.StatusBadRequest, "Password must be at least 12 characters")
		return
	}
	hash, err := passwords.Hash(body.NewPassword)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	ctx := r.Context()
	err = pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		if err := updatePasswordHash(ctx, tx, u.ID, hash); err != nil {
			return err
		}
		return revokeUserRefreshTokens(ctx, tx, u.ID)
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if err := a.startSession(w, r, u.ID); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// solves lists the ids of challenges the user solved; empty when not logged in.
func (a *Auth) solves(w http.ResponseWriter, r *http.Request, u *User) {
	ids := []uuid.UUID{}
	if u != nil {
		var err error
		if ids, err = listSolvedChallengeIDs(r.Context(), a.pool, u.ID); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, ids)
}

// startSession sets the session cookie and a refresh token for a new login.
func (a *Auth) startSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	token, err := newRefreshToken(r.Context(), a.pool, userID, uuid.NewV7())
	if err != nil {
		return err
	}
	a.setSession(w, userID)
	a.setRefreshCookie(w, token)
	return nil
}

// newRefreshToken stores a refresh token for userID in familyID and returns the cookie value.
func newRefreshToken(ctx context.Context, q database.Querier, userID, familyID uuid.UUID) (string, error) {
	token, hash, err := sessions.NewRefreshToken()
	if err != nil {
		return "", err
	}
	return token, createRefreshToken(ctx, q, userID, familyID, hash, time.Now().Add(sessions.RefreshMaxAge))
}

func (a *Auth) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshPath,
		MaxAge:   int(sessions.RefreshMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *Auth) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Path: refreshPath, MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func (a *Auth) setSession(w http.ResponseWriter, userID uuid.UUID) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessions.NewToken(a.secret, userID, time.Now()),
		Path:     "/",
		MaxAge:   int(sessions.MaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

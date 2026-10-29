// Package auth handles session and refresh cookies and the /api/auth endpoints.
// Password hashing lives in auth/passwords and token generation in auth/sessions.
package auth

import (
	"context"
	"net/http"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth/passwords"
	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
)

const (
	cookieName        = "session"
	refreshCookieName = "refresh"
	refreshPath       = "/api/auth/refresh"
)

// Auth authenticates requests with signed session cookies and serves the /api/auth endpoints.
type Auth struct {
	store  *database.Store
	secret []byte
}

// New returns an Auth that loads users from store and signs session tokens with secretKey.
func New(store *database.Store, secretKey string) *Auth {
	return &Auth{store: store, secret: []byte(secretKey)}
}

// Register adds the /api/auth routes to mux.
func (a *Auth) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/refresh", a.refresh)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/auth/me", a.Require(a.me))
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
	if n := utf8.RuneCountInString(body.Username); n < 2 || n > 32 {
		httpx.Error(w, http.StatusBadRequest, "Username must be 2-32 characters")
		return
	}
	if utf8.RuneCountInString(body.Password) < 8 {
		httpx.Error(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	ctx := r.Context()
	taken, err := a.store.UserExists(ctx, dbgen.UserExistsParams{Username: body.Username, Email: body.Email})
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
	u, err := a.store.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           uuid.NewV7(),
		Username:     body.Username,
		Email:        body.Email,
		PasswordHash: hash,
	})
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
	u, err := a.store.GetUserByUsername(r.Context(), body.Username)
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
	err = a.store.InTx(ctx, func(q *dbgen.Queries) error {
		t, err := q.ConsumeRefreshToken(ctx, dbgen.ConsumeRefreshTokenParams{
			TokenHash:  hash,
			ReuseAfter: time.Now().Add(-sessions.RefreshReuseWindow),
		})
		if err != nil {
			return err
		}
		userID = t.UserID
		token, err = newRefreshToken(ctx, q, t.UserID, t.FamilyID)
		return err
	})
	if database.IsNotFound(err) {
		// Unknown, expired, revoked, or replayed after the reuse window. A replay means the
		// token may have been stolen, so revoke every token rotated from the same login.
		if family, err := a.store.GetRefreshTokenFamily(ctx, hash); err == nil {
			if err := a.store.RevokeRefreshFamily(ctx, family); err != nil {
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
		family, err := a.store.GetRefreshTokenFamily(ctx, sessions.HashRefreshToken(c.Value))
		if err == nil {
			err = a.store.RevokeRefreshFamily(ctx, family)
		}
		if err != nil && !database.IsNotFound(err) {
			httpx.InternalError(w, r, err)
			return
		}
	}
	a.clearSession(w)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *Auth) me(w http.ResponseWriter, r *http.Request, u dbgen.User) {
	rank, err := a.store.UserRank(r.Context(), u.Score)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, struct {
		ID       uuid.UUID `json:"id"`
		Username string    `json:"username"`
		Score    int32     `json:"score"`
		IsAdmin  bool      `json:"is_admin"`
		Rank     int32     `json:"rank"`
	}{u.ID, u.Username, u.Score, u.IsAdmin, rank})
}

// solves lists the ids of challenges the user solved; empty when not logged in.
func (a *Auth) solves(w http.ResponseWriter, r *http.Request, u *dbgen.User) {
	ids := []uuid.UUID{}
	if u != nil {
		var err error
		if ids, err = a.store.ListSolvedChallengeIDs(r.Context(), u.ID); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, ids)
}

// startSession sets the session cookie and a refresh token for a new login.
func (a *Auth) startSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	token, err := newRefreshToken(r.Context(), a.store.Queries, userID, uuid.NewV7())
	if err != nil {
		return err
	}
	a.setSession(w, userID)
	a.setRefreshCookie(w, token)
	return nil
}

// newRefreshToken stores a refresh token for userID in familyID and returns the cookie value.
func newRefreshToken(ctx context.Context, q *dbgen.Queries, userID, familyID uuid.UUID) (string, error) {
	token, hash, err := sessions.NewRefreshToken()
	if err != nil {
		return "", err
	}
	return token, q.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		ID:        uuid.NewV7(),
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(sessions.RefreshMaxAge),
	})
}

func (a *Auth) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshPath, // only sent to the refresh endpoint
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

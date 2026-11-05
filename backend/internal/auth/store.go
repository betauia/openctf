package auth

import (
	"context"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/jackc/pgx/v5"
)

// User is a row of the users table.
type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	Score        int32
	Role         string
	TeamID       *uuid.UUID
	Banned       bool
}

const userColumns = `id, username, email, password_hash, score, role, team_id, banned`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Score, &u.Role, &u.TeamID, &u.Banned)
	return u, err
}

func getUserByID(ctx context.Context, q database.Querier, id uuid.UUID) (User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func getUserByUsername(ctx context.Context, q database.Querier, username string) (User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE username = $1`, username))
}

func userExists(ctx context.Context, q database.Querier, username, email string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE username = $1 OR email = $2)`, username, email,
	).Scan(&exists)
	return exists, err
}

func createUser(ctx context.Context, q database.Querier, id uuid.UUID, username, email, passwordHash string) (User, error) {
	return scanUser(q.QueryRow(ctx,
		`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4) RETURNING `+userColumns,
		id, username, email, passwordHash,
	))
}

// userRank returns 1 + the number of users with a higher score.
func userRank(ctx context.Context, q database.Querier, score int32) (int32, error) {
	var rank int32
	err := q.QueryRow(ctx, `SELECT COUNT(*) + 1 FROM users WHERE score > $1`, score).Scan(&rank)
	return rank, err
}

func listSolvedChallengeIDs(ctx context.Context, q database.Querier, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT challenge_id FROM solves WHERE user_id = $1 ORDER BY challenge_id`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func createRefreshToken(ctx context.Context, q database.Querier, userID, familyID uuid.UUID, hash []byte, expiresAt time.Time) error {
	_, err := q.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		uuid.NewV7(), userID, familyID, hash, expiresAt,
	)
	return err
}

// consumeRefreshToken marks a live token as used and returns its owner and family.
// A token first used after reuseAfter can be consumed again. pgx.ErrNoRows means
// the token is unknown, expired, revoked or replayed.
func consumeRefreshToken(ctx context.Context, q database.Querier, hash []byte, reuseAfter time.Time) (userID, familyID uuid.UUID, err error) {
	err = q.QueryRow(ctx, `
		UPDATE refresh_tokens SET used_at = COALESCE(used_at, now())
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		  AND (used_at IS NULL OR used_at > $2)
		RETURNING user_id, family_id`,
		hash, reuseAfter,
	).Scan(&userID, &familyID)
	return userID, familyID, err
}

func getRefreshTokenFamily(ctx context.Context, q database.Querier, hash []byte) (uuid.UUID, error) {
	var family uuid.UUID
	err := q.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE token_hash = $1`, hash).Scan(&family)
	return family, err
}

func revokeRefreshFamily(ctx context.Context, q database.Querier, familyID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`, familyID)
	return err
}

func updatePasswordHash(ctx context.Context, q database.Querier, userID uuid.UUID, passwordHash string) error {
	_, err := q.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash)
	return err
}

// revokeUserRefreshTokens revokes every live refresh token of userID, logging out all devices.
func revokeUserRefreshTokens(ctx context.Context, q database.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

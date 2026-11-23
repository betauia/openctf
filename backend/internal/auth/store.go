package auth

import (
	"context"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
)

// User is a row of the users table.
type User dbgen.User

func getSessionUser(ctx context.Context, q database.Querier, id, familyID uuid.UUID) (User, error) {
	u, err := dbgen.New(q).GetSessionUser(ctx, dbgen.GetSessionUserParams{ID: id, FamilyID: familyID})
	return User(u), err
}

func getUserByUsername(ctx context.Context, q database.Querier, username string) (User, error) {
	u, err := dbgen.New(q).GetUserByUsername(ctx, username)
	return User(u), err
}

func userExists(ctx context.Context, q database.Querier, username, email string) (bool, error) {
	return dbgen.New(q).UserExists(ctx, dbgen.UserExistsParams{Username: username, Email: email})
}

func createUser(ctx context.Context, q database.Querier, id uuid.UUID, username, email, passwordHash string) (User, error) {
	u, err := dbgen.New(q).CreateUser(ctx, dbgen.CreateUserParams{
		ID: id, Username: username, Email: email, PasswordHash: passwordHash,
	})
	return User(u), err
}

// userRank returns 1 + the number of users with a higher score.
func userRank(ctx context.Context, q database.Querier, score int32) (int32, error) {
	return dbgen.New(q).UserRank(ctx, score)
}

// listSolvedChallengeIDs returns the challenges solved by the user's team, or by
// the user alone when they are not on a team.
func listSolvedChallengeIDs(ctx context.Context, q database.Querier, userID uuid.UUID) ([]uuid.UUID, error) {
	return dbgen.New(q).ListSolvedChallengeIDs(ctx, userID)
}

func createRefreshToken(ctx context.Context, q database.Querier, userID, familyID uuid.UUID, hash []byte, expiresAt time.Time) error {
	return dbgen.New(q).CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		ID: uuid.NewV7(), UserID: userID, FamilyID: familyID, TokenHash: hash, ExpiresAt: expiresAt,
	})
}

// consumeRefreshToken marks a live token as used and returns its owner and family.
// A token first used after reuseAfter can be consumed again. pgx.ErrNoRows means
// the token is unknown, expired, revoked or replayed.
func consumeRefreshToken(ctx context.Context, q database.Querier, hash []byte, reuseAfter time.Time) (userID, familyID uuid.UUID, err error) {
	row, err := dbgen.New(q).ConsumeRefreshToken(ctx, dbgen.ConsumeRefreshTokenParams{TokenHash: hash, ReuseAfter: reuseAfter})
	return row.UserID, row.FamilyID, err
}

func getRefreshTokenFamily(ctx context.Context, q database.Querier, hash []byte) (uuid.UUID, error) {
	return dbgen.New(q).GetRefreshTokenFamily(ctx, hash)
}

func revokeRefreshFamily(ctx context.Context, q database.Querier, familyID uuid.UUID) error {
	return dbgen.New(q).RevokeRefreshFamily(ctx, familyID)
}

func updatePasswordHash(ctx context.Context, q database.Querier, userID uuid.UUID, passwordHash string) error {
	return dbgen.New(q).UpdatePasswordHash(ctx, dbgen.UpdatePasswordHashParams{ID: userID, PasswordHash: passwordHash})
}

// revokeUserRefreshTokens revokes every live refresh token of userID, logging out all devices.
func revokeUserRefreshTokens(ctx context.Context, q database.Querier, userID uuid.UUID) error {
	return dbgen.New(q).RevokeUserRefreshTokens(ctx, userID)
}

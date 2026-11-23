package auth

import (
	"context"
	"errors"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth/passwords"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureDefaultAdmin(ctx context.Context, pool *pgxpool.Pool, password string, email string) error {
	if password == "" {
		return nil
	}
	hash, err := passwords.Hash(password)
	if err != nil {
		return err
	}
	if email == "" {
		return errors.New("admin email must be set")
	}

	return dbgen.New(pool).EnsureAdmin(ctx, dbgen.EnsureAdminParams{
		ID: uuid.NewV7(), Email: email, PasswordHash: hash,
	})
}

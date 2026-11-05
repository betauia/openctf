package auth

import (
	"context"
	"errors"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth/passwords"
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

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, role)
		VALUES ($1, 'admin', $2, $3, 'admin')
		ON CONFLICT (username) DO NOTHING`,
		uuid.NewV7(), email, hash)
	return err
}

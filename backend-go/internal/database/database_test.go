package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	unique := &pgconn.PgError{Code: "23505"}
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{unique, true},
		{fmt.Errorf("create user: %w", unique), true},
		{&pgconn.PgError{Code: "23503"}, false}, // foreign key violation
		{errors.New("23505"), false},
	}
	for _, tt := range tests {
		if got := IsUniqueViolation(tt.err); got != tt.want {
			t.Errorf("IsUniqueViolation(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{pgx.ErrNoRows, true},
		{fmt.Errorf("get user: %w", pgx.ErrNoRows), true},
		{errors.New("no rows in result set"), false},
	}
	for _, tt := range tests {
		if got := IsNotFound(tt.err); got != tt.want {
			t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

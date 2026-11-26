// Package testdb starts the Postgres container the database tests share and has small helpers for
// setting up rows. It is for tests only.
//
// A package's tests start the container once from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }
//
// If the container can't start, usually because Docker isn't reachable, the whole run fails.
// The tests are never skipped.
package testdb

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/betauia/openctf/backend/internal/auth/passwords"
	"github.com/betauia/openctf/backend/internal/database"
)

// Password is the password of every user made by NewUser. PasswordHash is its hash, computed once
// because PBKDF2 is slow on purpose.
const Password = "correct horse battery"

var (
	// Pool is connected to the migrated test database. It is set by Run.
	Pool         *pgxpool.Pool
	PasswordHash string
)

// Run starts a Postgres container, migrates it, runs the package's tests and stops the
// container. It returns the exit code for os.Exit.
func Run(m *testing.M) int {
	// testcontainers prints a banner and a line per container step; a failure is reported below.
	tclog.SetDefault(tclog.NewNoopLogger())
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("openctf_test"),
		postgres.WithUsername("openctf"),
		postgres.WithPassword("openctf"),
		postgres.BasicWaitStrategies(),
	)
	if err == nil {
		var url string
		if url, err = ctr.ConnectionString(ctx, "sslmode=disable"); err == nil {
			if Pool, err = database.Open(ctx, url); err == nil {
				if err = database.Migrate(ctx, Pool); err == nil {
					// The migration starts the CTF at now(), so the first requests could still see it as not started.
					_, err = Pool.Exec(ctx, `UPDATE settings SET ctf_start = now() - interval '1 hour'`)
				}
				if err == nil {
					PasswordHash, err = passwords.Hash(Password)
				}
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "testcontainers cannot run: could not start Postgres, is Docker running and reachable? %v\n", err)
		stop(ctr)
		return 1
	}

	code := m.Run()

	stop(ctr)
	return code
}

func stop(ctr testcontainers.Container) {
	if Pool != nil {
		Pool.Close()
	}
	if ctr != nil {
		_ = testcontainers.TerminateContainer(ctr)
	}
}

// Unique returns a short random string, so names never collide in the shared database.
func Unique() string { return strings.ReplaceAll(uuid.NewV7().String(), "-", "")[12:] }

// Exec runs a statement and fails the test if it errors.
func Exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// QueryRow returns the single value a query selects.
func QueryRow[T any](t *testing.T, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := Pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

// User is a row of the users table. Cookie is the session cookie, set when the user logs in.
type User struct {
	ID     uuid.UUID
	Name   string
	Cookie *http.Cookie
}

// NewUser inserts a user with the role "user" and the password Password.
func NewUser(t *testing.T) *User { return newUser(t, "user") }

// NewMaintainer inserts a user with the role "maintainer".
func NewMaintainer(t *testing.T) *User { return newUser(t, "maintainer") }

// NewAdmin inserts a user with the role "admin".
func NewAdmin(t *testing.T) *User { return newUser(t, "admin") }

func newUser(t *testing.T, role string) *User {
	t.Helper()
	u := &User{ID: uuid.NewV7(), Name: "u" + Unique()}
	Exec(t, `INSERT INTO users (id, username, email, password_hash, role) VALUES ($1, $2, $3, $4, $5)`,
		u.ID, u.Name, u.Name+"@example.com", PasswordHash, role)
	return u
}

// Ban marks the user as banned. A banned user can't log in, so ban a user who is already logged
// in to have one that sends requests.
func (u *User) Ban(t *testing.T) {
	t.Helper()
	Exec(t, `UPDATE users SET banned = true WHERE id = $1`, u.ID)
}

// NewTeam inserts a team with the given captain and members, bypassing the API.
func NewTeam(t *testing.T, captain *User, members ...*User) (id uuid.UUID, inviteCode string) {
	t.Helper()
	id, inviteCode = uuid.NewV7(), Unique()
	Exec(t, `INSERT INTO teams (id, name, captain_id, invite_code) VALUES ($1, $2, $3, $4)`, id, "team"+Unique(), captain.ID, inviteCode)
	for _, u := range append([]*User{captain}, members...) {
		Exec(t, `UPDATE users SET team_id = $1 WHERE id = $2`, id, u.ID)
	}
	return id, inviteCode
}

// Challenge describes a challenge for AddChallenge. Empty fields get defaults: a unique title,
// the category "test" and 100 points.
type Challenge struct {
	Title    string
	Category string
	Points   int
	Flags    []string
	Hidden   bool
	Owner    *User
}

// AddChallenge inserts a challenge and deletes it, with its solves and files, when the test ends.
func AddChallenge(t *testing.T, c Challenge) uuid.UUID {
	t.Helper()
	if c.Title == "" {
		c.Title = "challenge" + Unique()
	}
	if c.Category == "" {
		c.Category = "test"
	}
	if c.Points == 0 {
		c.Points = 100
	}
	if c.Flags == nil {
		c.Flags = []string{}
	}
	var owner *uuid.UUID
	if c.Owner != nil {
		owner = &c.Owner.ID
	}
	id := uuid.NewV7()
	Exec(t, `INSERT INTO challenges (id, title, points, category, flags, is_visible, created_by_id) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		id, c.Title, c.Points, c.Category, jsonList(c.Flags), !c.Hidden, owner)
	t.Cleanup(func() { _, _ = Pool.Exec(context.Background(), `DELETE FROM challenges WHERE id = $1`, id) })
	return id
}

func jsonList(items []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q", s)
	}
	b.WriteByte(']')
	return b.String()
}

// snapshotSettings restores the settings row to what it is now when the test ends. Call it
// before anything that changes the row.
func snapshotSettings(t *testing.T) {
	t.Helper()
	var name string
	var start, end time.Time
	var registration bool
	var size int
	err := Pool.QueryRow(context.Background(),
		`SELECT ctf_name, ctf_start, ctf_end, registration, max_team_size FROM settings`).
		Scan(&name, &start, &end, &registration, &size)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = Pool.Exec(context.Background(),
			`UPDATE settings SET ctf_name = $1, ctf_start = $2, ctf_end = $3, registration = $4, max_team_size = $5`,
			name, start, end, registration, size)
	})
}

// ChangeSettings applies a SET clause to the settings row and restores the row when the test ends.
func ChangeSettings(t *testing.T, set string, args ...any) {
	t.Helper()
	snapshotSettings(t)
	Exec(t, `UPDATE settings SET `+set, args...)
}

// SetMaxTeamSize sets the largest allowed team; 0 means unlimited.
func SetMaxTeamSize(t *testing.T, n int) { ChangeSettings(t, `max_team_size = $1`, n) }

// EndCTF makes the CTF phase "ended".
func EndCTF(t *testing.T) {
	t.Helper()
	ChangeSettings(t, `ctf_start = now() - interval '2 hours', ctf_end = now() - interval '1 hour'`)
}

// DelayCTF makes the CTF phase "not_started".
func DelayCTF(t *testing.T) {
	t.Helper()
	ChangeSettings(t, `ctf_start = now() + interval '1 hour', ctf_end = now() + interval '2 hours'`)
}

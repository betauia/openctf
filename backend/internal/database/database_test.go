package database_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// connString is the URL of the test server, naming another database.
func connString(db string) string {
	return strings.Replace(testdb.Pool.Config().ConnString(), "/openctf_test", "/"+db, 1)
}

// freshDatabase creates an empty database on the test server and drops it when the test ends.
func freshDatabase(t *testing.T) string {
	t.Helper()
	name := "fresh_" + testdb.Unique()
	testdb.Exec(t, `CREATE DATABASE `+name)
	t.Cleanup(func() {
		_, _ = testdb.Pool.Exec(context.Background(), `DROP DATABASE `+name+` WITH (FORCE)`)
	})
	return connString(name)
}

// try runs a statement and returns its error instead of failing the test.
func try(sql string, args ...any) error {
	_, err := testdb.Pool.Exec(context.Background(), sql, args...)
	return err
}

// --- Open ---

func TestOpenReturnsAWorkingPool(t *testing.T) {
	pool, err := database.Open(context.Background(), testdb.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var one int
	if err := pool.QueryRow(context.Background(), `SELECT 1`).Scan(&one); err != nil || one != 1 {
		t.Errorf("SELECT 1 = %d, %v", one, err)
	}
}

func TestOpenFailsWithoutAPool(t *testing.T) {
	good := testdb.Pool.Config().ConnString()
	tests := map[string]string{
		"an unparsable URL":     "postgres://%zz",
		"nothing listening":     "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=2",
		"the wrong password":    strings.Replace(good, "openctf:openctf@", "openctf:wrong@", 1),
		"a database that isn't": connString("no_such_database"),
	}
	for name, url := range tests {
		t.Run(name, func(t *testing.T) {
			pool, err := database.Open(context.Background(), url)

			if err == nil {
				pool.Close()
				t.Fatal("Open succeeded")
			}
			if pool != nil {
				t.Error("Open returned a pool together with an error")
			}
		})
	}
}

// --- Migrate ---

func tables(t *testing.T, url string) []string {
	t.Helper()
	pool, err := database.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(context.Background(),
		`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestMigrateBuildsTheSchemaOnAnEmptyDatabase(t *testing.T) {
	url := freshDatabase(t)
	pool, err := database.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}

	got := tables(t, url)
	for _, want := range []string{"challenge_files", "challenges", "goose_db_version", "refresh_tokens", "settings", "solves", "teams", "users"} {
		if !slices.Contains(got, want) {
			t.Errorf("table %q missing; have %v", want, got)
		}
	}
	var rows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM settings`).Scan(&rows); err != nil || rows != 1 {
		t.Errorf("settings has %d rows (%v), want the one default row", rows, err)
	}
}

func TestMigrateCanRunAgain(t *testing.T) {
	url := freshDatabase(t)
	pool, err := database.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	before := tables(t, url)

	if err := database.Migrate(ctx, pool); err != nil {
		t.Errorf("the second Migrate failed: %v", err)
	}
	if after := tables(t, url); !slices.Equal(before, after) {
		t.Errorf("tables changed on the second run: %v then %v", before, after)
	}
	var settings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM settings`).Scan(&settings); err != nil || settings != 1 {
		t.Errorf("settings has %d rows (%v) after migrating twice, want 1", settings, err)
	}
}

func TestMigrateOnTheAlreadyMigratedTestDatabase(t *testing.T) {
	if err := database.Migrate(context.Background(), testdb.Pool); err != nil {
		t.Errorf("Migrate on a migrated database failed: %v", err)
	}
}

// --- error helpers ---

func TestIsNotFound(t *testing.T) {
	var id uuid.UUID
	missing := testdb.Pool.QueryRow(context.Background(), `SELECT id FROM users WHERE id = $1`, uuid.NewV7()).Scan(&id)
	syntax := testdb.Pool.QueryRow(context.Background(), `SELEC nope`).Scan(&id)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"no row", missing, true},
		{"no row, wrapped", fmt.Errorf("load user: %w", missing), true},
		{"no row, wrapped twice", fmt.Errorf("a: %w", fmt.Errorf("b: %w", missing)), true},
		{"a SQL error", syntax, false},
		{"another error", errors.New("no rows in result set"), false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := database.IsNotFound(tc.err); got != tc.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	u := testdb.NewUser(t)
	unique := try(`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, 'x')`,
		uuid.NewV7(), strings.ToUpper(u.Name), "other"+u.Name+"@example.com")
	check := try(`INSERT INTO users (id, username, email, password_hash, role) VALUES ($1, $2, $3, 'x', 'owner')`,
		uuid.NewV7(), "n"+testdb.Unique(), "n"+testdb.Unique()+"@example.com")
	foreign := try(`INSERT INTO solves (user_id, challenge_id) VALUES ($1, $2)`, uuid.NewV7(), uuid.NewV7())
	syntax := try(`INSERT NOTHING`)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"a duplicate username", unique, true},
		{"a duplicate, wrapped", fmt.Errorf("create user: %w", unique), true},
		{"a check violation", check, false},
		{"a foreign key violation", foreign, false},
		{"a syntax error", syntax, false},
		{"a plain error", errors.New("duplicate key value violates unique constraint"), false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil && tc.name != "nil" {
				t.Fatal("the statement unexpectedly succeeded")
			}
			if got := database.IsUniqueViolation(tc.err); got != tc.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// --- schema constraints ---

func TestSchemaRejectsInvalidRows(t *testing.T) {
	existing := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, existing)
	teamName := testdb.QueryRow[string](t, `SELECT name FROM teams WHERE id = $1`, team)
	newEmail := func() string { return "e" + testdb.Unique() + "@example.com" }

	tests := []struct {
		name       string
		err        error
		wantUnique bool // a unique violation, not just any error
	}{
		{"a role outside user, maintainer and admin",
			try(`INSERT INTO users (id, username, email, password_hash, role) VALUES ($1, $2, $3, 'x', 'owner')`, uuid.NewV7(), "r"+testdb.Unique(), newEmail()), false},
		{"a username that differs only in case",
			try(`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, 'x')`, uuid.NewV7(), strings.ToUpper(existing.Name), newEmail()), true},
		{"a repeated email",
			try(`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, 'x')`, uuid.NewV7(), "d"+testdb.Unique(), existing.Name+"@example.com"), true},
		{"a team name that differs only in case",
			try(`INSERT INTO teams (id, name, invite_code) VALUES ($1, $2, $3)`, uuid.NewV7(), strings.ToUpper(teamName), testdb.Unique()), true},
		{"a repeated invite code",
			try(`INSERT INTO teams (id, name, invite_code) VALUES ($1, $2, (SELECT invite_code FROM teams WHERE id = $3))`, uuid.NewV7(), "x"+testdb.Unique(), team), true},
		{"a second settings row",
			try(`INSERT INTO settings DEFAULT VALUES`), true},
		{"a settings row that isn't the only one",
			try(`INSERT INTO settings (id) VALUES (false)`), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("the row was accepted")
			}
			if got := database.IsUniqueViolation(tc.err); got != tc.wantUnique {
				t.Errorf("unique violation = %v, want %v (%v)", got, tc.wantUnique, tc.err)
			}
		})
	}
}

func TestSchemaAllowsOneSolvePerUserAndPerTeamAndChallenge(t *testing.T) {
	first, second := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, first, second)
	challenge := testdb.AddChallenge(t, testdb.Challenge{})
	insert := func(user *testdb.User) error {
		return try(`INSERT INTO solves (user_id, team_id, challenge_id) VALUES ($1, $2, $3)`, user.ID, team, challenge)
	}

	if err := insert(first); err != nil {
		t.Fatal(err)
	}

	if err := insert(first); !database.IsUniqueViolation(err) {
		t.Errorf("the same user solving twice: %v, want a unique violation", err)
	}
	if err := insert(second); !database.IsUniqueViolation(err) {
		t.Errorf("a teammate solving again: %v, want a unique violation", err)
	}
}

func TestDeletingAChallengeRemovesItsSolvesAndFiles(t *testing.T) {
	user := testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, user)
	challenge := testdb.AddChallenge(t, testdb.Challenge{})
	testdb.Exec(t, `INSERT INTO solves (user_id, team_id, challenge_id) VALUES ($1, $2, $3)`, user.ID, team, challenge)
	testdb.Exec(t, `INSERT INTO challenge_files (id, challenge_id, filename, filepath) VALUES ($1, $2, 'f', 'p')`, uuid.NewV7(), challenge)

	testdb.Exec(t, `DELETE FROM challenges WHERE id = $1`, challenge)

	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM solves WHERE challenge_id = $1`, challenge); n != 0 {
		t.Errorf("%d solves remain", n)
	}
	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM challenge_files WHERE challenge_id = $1`, challenge); n != 0 {
		t.Errorf("%d files remain", n)
	}
}

func TestDeletingATeamFreesItsMembersAndKeepsTheirSolves(t *testing.T) {
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)
	challenge := testdb.AddChallenge(t, testdb.Challenge{})
	testdb.Exec(t, `INSERT INTO solves (user_id, team_id, challenge_id) VALUES ($1, $2, $3)`, member.ID, team, challenge)

	testdb.Exec(t, `DELETE FROM teams WHERE id = $1`, team)

	for _, u := range []*testdb.User{captain, member} {
		if got := testdb.QueryRow[*uuid.UUID](t, `SELECT team_id FROM users WHERE id = $1`, u.ID); got != nil {
			t.Errorf("user %s is still on team %v", u.Name, got)
		}
	}
	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM solves WHERE challenge_id = $1 AND team_id IS NULL`, challenge); n != 1 {
		t.Errorf("%d solves kept without a team, want the 1 solve kept", n)
	}
}

func TestDeletingAUserRemovesTheirSolvesAndLeavesTheTeamCaptainless(t *testing.T) {
	captain, member := testdb.NewUser(t), testdb.NewUser(t)
	team, _ := testdb.NewTeam(t, captain, member)
	challenge := testdb.AddChallenge(t, testdb.Challenge{})
	testdb.Exec(t, `INSERT INTO solves (user_id, team_id, challenge_id) VALUES ($1, $2, $3)`, captain.ID, team, challenge)

	testdb.Exec(t, `DELETE FROM users WHERE id = $1`, captain.ID)

	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM solves WHERE challenge_id = $1`, challenge); n != 0 {
		t.Errorf("%d solves remain for a deleted user", n)
	}
	if got := testdb.QueryRow[*uuid.UUID](t, `SELECT captain_id FROM teams WHERE id = $1`, team); got != nil {
		t.Errorf("the team's captain is %v, want none", got)
	}
	if n := testdb.QueryRow[int](t, `SELECT count(*) FROM teams WHERE id = $1`, team); n != 1 {
		t.Error("deleting the captain deleted the team")
	}
}

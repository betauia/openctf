package dbgen_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// newTx returns Queries bound to a transaction that is rolled back when the test ends.
func newTx(t *testing.T) (*dbgen.Queries, pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	tx, err := testdb.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return dbgen.New(tx), tx
}

func newUser(t *testing.T, q *dbgen.Queries, name string) dbgen.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), dbgen.CreateUserParams{
		ID: uuid.NewV7(), Username: name, Email: name + "@example.com", PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func newChallenge(t *testing.T, tx pgx.Tx, title string, visible bool) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	_, err := tx.Exec(context.Background(),
		`INSERT INTO challenges (id, title, points, category, is_visible) VALUES ($1, $2, 100, 'zzz', $3)`,
		id, title, visible)
	if err != nil {
		t.Fatalf("insert challenge: %v", err)
	}
	return id
}

func TestChallengeVisibility(t *testing.T) {
	q, tx := newTx(t)
	ctx := context.Background()
	shown := newChallenge(t, tx, "shown", true)
	hidden := newChallenge(t, tx, "hidden", false)

	// GetChallenge used an invalid "$i" placeholder before sqlc; it must work in both modes.
	if c, err := q.GetChallenge(ctx, dbgen.GetChallengeParams{ID: shown}); err != nil || c.Title != "shown" {
		t.Fatalf("visible challenge: %v, %+v", err, c)
	}
	if _, err := q.GetChallenge(ctx, dbgen.GetChallengeParams{ID: hidden}); !database.IsNotFound(err) {
		t.Errorf("hidden challenge without include_hidden: err = %v, want not found", err)
	}
	if c, err := q.GetChallenge(ctx, dbgen.GetChallengeParams{ID: hidden, IncludeHidden: true}); err != nil || c.Title != "hidden" {
		t.Errorf("hidden challenge with include_hidden: %v, %+v", err, c)
	}

	has := func(list []dbgen.Challenge, id uuid.UUID) bool {
		for _, c := range list {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	pub, err := q.ListChallenges(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !has(pub, shown) || has(pub, hidden) {
		t.Errorf("public list: shown=%v hidden=%v, want true/false", has(pub, shown), has(pub, hidden))
	}
	all, err := q.ListChallenges(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !has(all, shown) || !has(all, hidden) {
		t.Error("admin list should contain both challenges")
	}
}

func TestChallengeFilesFollowChallengeVisibility(t *testing.T) {
	q, tx := newTx(t)
	ctx := context.Background()
	hidden := newChallenge(t, tx, "hidden", false)
	fid := uuid.NewV7()
	if _, err := tx.Exec(ctx,
		`INSERT INTO challenge_files (id, challenge_id, filename, filepath) VALUES ($1, $2, 'a.zip', 'a.zip')`,
		fid, hidden); err != nil {
		t.Fatal(err)
	}

	files, err := q.ListChallengeFiles(ctx, dbgen.ListChallengeFilesParams{ChallengeID: hidden})
	if err != nil || len(files) != 0 {
		t.Errorf("hidden files for player: %v, %d files", err, len(files))
	}
	files, err = q.ListChallengeFiles(ctx, dbgen.ListChallengeFilesParams{ChallengeID: hidden, IncludeHidden: true})
	if err != nil || len(files) != 1 {
		t.Errorf("hidden files for admin: %v, %d files", err, len(files))
	}
	if _, err := q.GetChallengeFile(ctx, dbgen.GetChallengeFileParams{ID: fid, ChallengeID: hidden}); !database.IsNotFound(err) {
		t.Errorf("hidden file for player: err = %v, want not found", err)
	}
	if f, err := q.GetChallengeFile(ctx, dbgen.GetChallengeFileParams{ID: fid, ChallengeID: hidden, IncludeHidden: true}); err != nil || f.Filename != "a.zip" {
		t.Errorf("hidden file for admin: %v, %+v", err, f)
	}
	// A file id paired with the wrong challenge must not match.
	other := newChallenge(t, tx, "other", true)
	if _, err := q.GetChallengeFile(ctx, dbgen.GetChallengeFileParams{ID: fid, ChallengeID: other, IncludeHidden: true}); !database.IsNotFound(err) {
		t.Errorf("file under wrong challenge: err = %v, want not found", err)
	}
}

func consume(h []byte, reuseAfter time.Duration) dbgen.ConsumeRefreshTokenParams {
	return dbgen.ConsumeRefreshTokenParams{TokenHash: h, ReuseAfter: time.Now().Add(reuseAfter)}
}

func TestRefreshTokenLifecycle(t *testing.T) {
	q, _ := newTx(t)
	ctx := context.Background()
	u := newUser(t, q, "refresh")
	family := uuid.NewV7()
	hash := []byte("hash-1")
	if err := q.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		ID: uuid.NewV7(), UserID: u.ID, FamilyID: family, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// First use succeeds and returns the owner and family.
	row, err := q.ConsumeRefreshToken(ctx, consume(hash, -time.Minute))
	if err != nil || row.UserID != u.ID || row.FamilyID != family {
		t.Fatalf("first consume: %v, %+v", err, row)
	}
	// Within the reuse window it may be consumed again.
	if _, err := q.ConsumeRefreshToken(ctx, consume(hash, -time.Minute)); err != nil {
		t.Errorf("consume inside reuse window: %v", err)
	}
	// Outside the window it counts as a replay.
	if _, err := q.ConsumeRefreshToken(ctx, consume(hash, time.Minute)); !database.IsNotFound(err) {
		t.Errorf("replay: err = %v, want not found", err)
	}
	if got, err := q.GetRefreshTokenFamily(ctx, hash); err != nil || got != family {
		t.Errorf("family: %v, %v", err, got)
	}

	// Revoking the family kills it.
	if err := q.RevokeRefreshFamily(ctx, family); err != nil {
		t.Fatal(err)
	}
	if _, err := q.ConsumeRefreshToken(ctx, consume(hash, -time.Minute)); !database.IsNotFound(err) {
		t.Errorf("revoked token: err = %v, want not found", err)
	}
}

func TestRefreshTokenExpiredAndUserRevoke(t *testing.T) {
	q, _ := newTx(t)
	ctx := context.Background()
	u := newUser(t, q, "refresh2")
	expired, live := []byte("expired"), []byte("live")
	for _, tk := range []struct {
		h   []byte
		exp time.Time
	}{{expired, time.Now().Add(-time.Hour)}, {live, time.Now().Add(time.Hour)}} {
		if err := q.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
			ID: uuid.NewV7(), UserID: u.ID, FamilyID: uuid.NewV7(), TokenHash: tk.h, ExpiresAt: tk.exp,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.ConsumeRefreshToken(ctx, consume(expired, -time.Minute)); !database.IsNotFound(err) {
		t.Errorf("expired: err = %v, want not found", err)
	}
	if err := q.RevokeUserRefreshTokens(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.ConsumeRefreshToken(ctx, consume(live, -time.Minute)); !database.IsNotFound(err) {
		t.Errorf("after user revoke: err = %v, want not found", err)
	}
}

func TestUpdateSettingsKeepsUnsetFields(t *testing.T) {
	q, _ := newTx(t)
	ctx := context.Background()
	before, err := q.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	name := "Renamed CTF"
	start := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	end := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := q.UpdateSettings(ctx, dbgen.UpdateSettingsParams{CtfName: &name, CtfStart: start, CtfEnd: end}); err != nil {
		t.Fatal(err)
	}
	after, err := q.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.CtfName != name || !after.CtfStart.Equal(start) || !after.CtfEnd.Equal(end) {
		t.Errorf("set fields not updated: %+v", after)
	}
	if after.Registration != before.Registration || after.MaxTeamSize != before.MaxTeamSize {
		t.Errorf("nil fields changed: registration %v->%v, max_team_size %v->%v",
			before.Registration, after.Registration, before.MaxTeamSize, after.MaxTeamSize)
	}

	win, err := q.LockSettingsWindow(ctx)
	if err != nil || !win.CtfStart.Equal(start) || !win.CtfEnd.Equal(end) {
		t.Errorf("lock window: %v, %+v", err, win)
	}
}

func TestPolicyEnvCountsAdmins(t *testing.T) {
	q, tx := newTx(t)
	ctx := context.Background()
	before, err := q.GetPolicyEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u := newUser(t, q, "newadmin")
	if _, err := tx.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, u.ID); err != nil {
		t.Fatal(err)
	}
	after, err := q.GetPolicyEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.AdminCount != before.AdminCount+1 {
		t.Errorf("admin count %d -> %d, want +1", before.AdminCount, after.AdminCount)
	}
}

func TestTeamMembersAndVisibility(t *testing.T) {
	q, tx := newTx(t)
	ctx := context.Background()
	captain, mate := newUser(t, q, "captain"), newUser(t, q, "mate")
	teamID := uuid.NewV7()
	code := "invite-" + teamID.String()
	if err := q.CreateTeam(ctx, dbgen.CreateTeamParams{ID: teamID, Name: "Zzz Team", CaptainID: &captain.ID, InviteCode: &code}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []uuid.UUID{captain.ID, mate.ID} {
		if n, err := q.JoinTeam(ctx, dbgen.JoinTeamParams{TeamID: &teamID, UserID: u}); err != nil || n != 1 {
			t.Fatalf("join: %v, rows %d", err, n)
		}
	}
	// A user already on a team cannot be moved by JoinTeam.
	other := uuid.NewV7()
	if err := q.CreateTeam(ctx, dbgen.CreateTeamParams{ID: other, Name: "Other Team"}); err != nil {
		t.Fatal(err)
	}
	if n, err := q.JoinTeam(ctx, dbgen.JoinTeamParams{TeamID: &other, UserID: mate.ID}); err != nil || n != 0 {
		t.Errorf("join while on a team: %v, rows %d, want 0", err, n)
	}

	got, err := q.GetTeam(ctx, dbgen.GetTeamParams{ID: teamID})
	if err != nil || got.MemberCount != 2 {
		t.Fatalf("get team: %v, %+v", err, got)
	}
	if id, err := q.GetTeamIDByInviteCode(ctx, &code); err != nil || id != teamID {
		t.Errorf("invite lookup: %v, %v", err, id)
	}
	if st, err := q.GetSubjectTeam(ctx, dbgen.GetSubjectTeamParams{UserID: captain.ID, TeamID: teamID}); err != nil || !st.IsCaptain {
		t.Errorf("captain subject: %v, %+v", err, st)
	}
	if st, err := q.GetSubjectTeam(ctx, dbgen.GetSubjectTeamParams{UserID: mate.ID, TeamID: teamID}); err != nil || st.IsCaptain {
		t.Errorf("member subject: %v, %+v", err, st)
	}

	// Removing a member only works for the right team.
	if n, err := q.RemoveTeamMember(ctx, dbgen.RemoveTeamMemberParams{UserID: mate.ID, TeamID: &other}); err != nil || n != 0 {
		t.Errorf("remove from wrong team: %v, rows %d", err, n)
	}
	if n, err := q.RemoveTeamMember(ctx, dbgen.RemoveTeamMemberParams{UserID: mate.ID, TeamID: &teamID}); err != nil || n != 1 {
		t.Errorf("remove member: %v, rows %d", err, n)
	}
	if got, _ := q.GetTeam(ctx, dbgen.GetTeamParams{ID: teamID}); got.MemberCount != 1 {
		t.Errorf("member count after removal = %d, want 1", got.MemberCount)
	}

	// Hidden teams are only visible with include_hidden.
	if _, err := tx.Exec(ctx, `UPDATE teams SET hidden = TRUE WHERE id = $1`, teamID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetTeam(ctx, dbgen.GetTeamParams{ID: teamID}); !database.IsNotFound(err) {
		t.Errorf("hidden team for player: err = %v, want not found", err)
	}
	if _, err := q.GetTeam(ctx, dbgen.GetTeamParams{ID: teamID, IncludeHidden: true}); err != nil {
		t.Errorf("hidden team for admin: %v", err)
	}
	listed := func(include bool) bool {
		rows, err := q.ListTeams(ctx, dbgen.ListTeamsParams{IncludeHidden: include, Lim: 1000})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == teamID {
				return true
			}
		}
		return false
	}
	if listed(false) || !listed(true) {
		t.Errorf("hidden team listing: player=%v admin=%v, want false/true", listed(false), listed(true))
	}
}

func TestListSolvedChallengeIDsAndRank(t *testing.T) {
	q, tx := newTx(t)
	ctx := context.Background()
	u := newUser(t, q, "solver")
	a := newChallenge(t, tx, "a", true)
	b := newChallenge(t, tx, "b", true)
	for _, c := range []uuid.UUID{a, b} {
		if _, err := tx.Exec(ctx, `INSERT INTO solves (user_id, challenge_id) VALUES ($1, $2)`, u.ID, c); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := q.ListSolvedChallengeIDs(ctx, u.ID)
	if err != nil || len(ids) != 2 {
		t.Fatalf("solved ids: %v, %v", err, ids)
	}
	if ids[0].String() > ids[1].String() {
		t.Errorf("solved ids not sorted: %v", ids)
	}

	// Rank is 1 + users with a strictly higher score.
	if _, err := tx.Exec(ctx, `UPDATE users SET score = 1000000 WHERE id = $1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if r, err := q.UserRank(ctx, 1000000); err != nil || r != 1 {
		t.Errorf("rank for top score: %v, %d, want 1", err, r)
	}
	if r, err := q.UserRank(ctx, 999999); err != nil || r < 2 {
		t.Errorf("rank below top score: %v, %d, want >= 2", err, r)
	}
}

package teams

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/database"
)

type team struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	CaptainID   *uuid.UUID `json:"captain_id"`            // nil when the captain's account was deleted
	InviteCode  *string    `json:"invite_code,omitempty"` // only for members and admins
	Banned      bool       `json:"banned"`
	MemberCount int        `json:"member_count"`
}

type member struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int32     `json:"score"`
}

const teamColumns = `t.id, t.name, t.captain_id, t.invite_code, t.banned,
	(SELECT COUNT(*) FROM users u WHERE u.team_id = t.id)`

func scanTeam(row pgx.Row) (team, error) {
	var t team
	err := row.Scan(&t.ID, &t.Name, &t.CaptainID, &t.InviteCode, &t.Banned, &t.MemberCount)
	return t, err
}

// resource is the only place a team becomes an authz.Resource. MemberCount and Banned are
// required: TeamFull, CaptainMustTransfer and NoJoinBannedTeam read them.
func (t team) resource() authz.Resource {
	return authz.Resource{ID: t.ID, MemberCount: t.MemberCount, Banned: t.Banned}
}

func getTeam(ctx context.Context, q database.Querier, id uuid.UUID) (team, error) {
	return scanTeam(q.QueryRow(ctx, `SELECT `+teamColumns+` FROM teams t WHERE t.id = $1`, id))
}

func lockTeam(ctx context.Context, tx pgx.Tx, id uuid.UUID) (team, error) {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM teams WHERE id = $1 FOR UPDATE`, id); err != nil {
		return team{}, err
	}
	return getTeam(ctx, tx, id)
}

func listMembers(ctx context.Context, q database.Querier, teamID uuid.UUID) ([]member, error) {
	rows, err := q.Query(ctx, `SELECT id, username, score FROM users WHERE team_id = $1 ORDER BY username`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (member, error) {
		var m member
		err := row.Scan(&m.ID, &m.Username, &m.Score)
		return m, err
	})
}

func listTeams(ctx context.Context, q database.Querier, limit, offset int) ([]team, error) {
	rows, err := q.Query(ctx, "SELECT"+teamColumns+" FROM teams t ORDER BY t.name ASC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (team, error) {
		return scanTeam(row)
	})
}

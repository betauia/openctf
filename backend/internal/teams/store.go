package teams

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/policy"
)

type team struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	CaptainID   *uuid.UUID `json:"captain_id"`            // nil when the captain's account was deleted
	InviteCode  *string    `json:"invite_code,omitempty"` // only for members and admins
	Banned      bool       `json:"banned"`
	Hidden      bool       `json:"hidden"`
	MemberCount int        `json:"member_count"`
}

type member struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int32     `json:"score"`
}

const teamColumns = `t.id, t.name, t.captain_id, t.invite_code, t.banned, t.hidden,
	(SELECT COUNT(*) FROM users u WHERE u.team_id = t.id)`

func scanTeam(row pgx.Row) (team, error) {
	var t team
	err := row.Scan(&t.ID, &t.Name, &t.CaptainID, &t.InviteCode, &t.Banned, &t.Hidden, &t.MemberCount)
	return t, err
}

// resource is the only place a team becomes a policy.Resource. The path is left to the
// checker (the request path). member_count and banned are required: TeamJoinNotFull,
// CaptainMustTransfer and NoJoinBannedTeam read them.
func (t team) resource() policy.Resource {
	return policy.Resource{ID: t.ID.String(), Attributes: map[string]any{
		"member_count": t.MemberCount,
		"banned":       t.Banned,
		"hidden":       t.Hidden,
	}}
}

// getTeam loads one team. Unless includeHidden is set, a hidden team is reported as not found.
func getTeam(ctx context.Context, q database.Querier, id uuid.UUID, includeHidden bool) (team, error) {
	if includeHidden {
		return scanTeam(q.QueryRow(ctx, `SELECT `+teamColumns+` FROM teams t WHERE t.id = $1`, id))
	}
	return scanTeam(q.QueryRow(ctx, `SELECT `+teamColumns+` FROM teams t WHERE t.id = $1 AND NOT t.hidden`, id))
}

// lockTeam locks a team row for the rest of tx. Hidden teams are included: hiding a team
// must not stop its members or admins from changing it.
func lockTeam(ctx context.Context, tx pgx.Tx, id uuid.UUID) (team, error) {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM teams WHERE id = $1 FOR UPDATE`, id); err != nil {
		return team{}, err
	}
	return getTeam(ctx, tx, id, true)
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

// listTeams returns a page of teams sorted by name. Hidden teams are left out unless includeHidden is set.
func listTeams(ctx context.Context, q database.Querier, limit, offset int, includeHidden bool) ([]team, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if includeHidden {
		rows, err = q.Query(ctx, "SELECT "+teamColumns+" FROM teams t ORDER BY t.name ASC LIMIT $1 OFFSET $2", limit, offset)
	} else {
		rows, err = q.Query(ctx, "SELECT "+teamColumns+" FROM teams t WHERE NOT t.hidden ORDER BY t.name ASC LIMIT $1 OFFSET $2", limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (team, error) {
		return scanTeam(row)
	})
}

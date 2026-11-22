package teams

import (
	"context"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/policy"
)

type team struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	CaptainID   *uuid.UUID `json:"captain_id"`
	InviteCode  *string    `json:"invite_code,omitempty"`
	Banned      bool       `json:"banned"`
	Hidden      bool       `json:"hidden"`
	MemberCount int        `json:"member_count"`
	Score       int        `json:"score"`
	Rank        int        `json:"rank"`
}

type member struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int32     `json:"score"`
	Solves   int       `json:"solves"`
}

const teamScore = `(SELECT COALESCE(SUM(c.points), 0) FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE s.team_id = t.id)`

const teamColumns = `t.id, t.name, t.captain_id, t.invite_code, t.banned, t.hidden,
	(SELECT COUNT(*) FROM users u WHERE u.team_id = t.id), ` + teamScore

func scanTeam(row pgx.Row) (team, error) {
	var t team
	err := row.Scan(&t.ID, &t.Name, &t.CaptainID, &t.InviteCode, &t.Banned, &t.Hidden, &t.MemberCount, &t.Score)
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
	rows, err := q.Query(ctx, `SELECT u.id, u.username, u.score, (SELECT COUNT(*) FROM solves s WHERE s.user_id = u.id)
		FROM users u WHERE u.team_id = $1 ORDER BY u.score DESC, u.username`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (member, error) {
		var m member
		err := row.Scan(&m.ID, &m.Username, &m.Score, &m.Solves)
		return m, err
	})
}

type activityEntry struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	Username    string    `json:"username"`
	Challenge   string    `json:"challenge"`
	Category    string    `json:"category"`
	Points      int       `json:"points"`
	SolvedAt    time.Time `json:"solved_at"`
}

func listActivity(ctx context.Context, q database.Querier, teamID uuid.UUID) ([]activityEntry, error) {
	rows, err := q.Query(ctx, `
		SELECT c.id, u.username, c.title, c.category, c.points, s.solved_at
		FROM solves s
		JOIN users u ON u.id = s.user_id
		JOIN challenges c ON c.id = s.challenge_id AND c.is_visible
		WHERE s.team_id = $1
		ORDER BY s.solved_at DESC`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (activityEntry, error) {
		var e activityEntry
		err := row.Scan(&e.ChallengeID, &e.Username, &e.Challenge, &e.Category, &e.Points, &e.SolvedAt)
		return e, err
	})
}

type profileSolve struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	Challenge   string    `json:"challenge"`
	Category    string    `json:"category"`
	Points      int       `json:"points"`
	SolvedAt    time.Time `json:"solved_at"`
}

type profile struct {
	ID       uuid.UUID      `json:"id"`
	Username string         `json:"username"`
	Score    int32          `json:"score"`
	TeamID   *uuid.UUID     `json:"team_id"`
	Team     *string        `json:"team"`
	Solves   []profileSolve `json:"solves"`
}

func getProfile(ctx context.Context, q database.Querier, userID uuid.UUID) (profile, error) {
	var p profile
	if err := q.QueryRow(ctx, `SELECT u.id, u.username, u.score, t.id, t.name FROM users u LEFT JOIN teams t ON t.id = u.team_id AND NOT t.hidden WHERE u.id = $1 AND NOT u.banned`, userID).
		Scan(&p.ID, &p.Username, &p.Score, &p.TeamID, &p.Team); err != nil {
		return p, err
	}
	rows, err := q.Query(ctx, `
		SELECT c.id, c.title, c.category, c.points, s.solved_at
		FROM solves s
		JOIN challenges c ON c.id = s.challenge_id AND c.is_visible
		WHERE s.user_id = $1
		ORDER BY s.solved_at DESC`, userID)
	if err != nil {
		return p, err
	}
	p.Solves, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (profileSolve, error) {
		var s profileSolve
		err := row.Scan(&s.ChallengeID, &s.Challenge, &s.Category, &s.Points, &s.SolvedAt)
		return s, err
	})
	return p, err
}

// listTeams returns a page of teams sorted by name. Hidden teams are left out unless includeHidden is set.
func listTeams(ctx context.Context, q database.Querier, limit, offset int, includeHidden bool) ([]team, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if includeHidden {
		rows, err = q.Query(ctx, "SELECT "+teamColumns+" FROM teams t ORDER BY "+teamScore+" DESC, t.name ASC LIMIT $1 OFFSET $2", limit, offset)
	} else {
		rows, err = q.Query(ctx, "SELECT "+teamColumns+" FROM teams t WHERE NOT t.hidden ORDER BY "+teamScore+" DESC, t.name ASC LIMIT $1 OFFSET $2", limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (team, error) {
		return scanTeam(row)
	})
}

func teamRank(ctx context.Context, q database.Querier, score int) (int, error) {
	var rank int
	err := q.QueryRow(ctx, `SELECT COUNT(*) + 1 FROM teams t WHERE NOT t.hidden AND `+teamScore+` > $1`, score).Scan(&rank)
	return rank, err
}

type categoryTotal struct {
	Category string
	Total    int
}

func listCategoryTotals(ctx context.Context, q database.Querier) ([]categoryTotal, error) {
	rows, err := q.Query(ctx, `SELECT category, COUNT(*) FROM challenges WHERE is_visible GROUP BY category ORDER BY category`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (categoryTotal, error) {
		var c categoryTotal
		err := row.Scan(&c.Category, &c.Total)
		return c, err
	})
}

type solvedChallenge struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	Category string    `json:"category"`
	Points   int       `json:"points"`
}

func listTeamSolves(ctx context.Context, q database.Querier, teamID uuid.UUID) ([]solvedChallenge, error) {
	rows, err := q.Query(ctx, `SELECT c.id, c.title, c.category, c.points FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE c.is_visible AND s.team_id = $1`, teamID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (solvedChallenge, error) {
		var c solvedChallenge
		err := row.Scan(&c.ID, &c.Title, &c.Category, &c.Points)
		return c, err
	})
}

type categoryProgress struct {
	Category string `json:"category"`
	Total    int    `json:"total"`
	Solved   int    `json:"solved"`
}

type progress struct {
	Total      int                `json:"total"`
	Solved     int                `json:"solved"`
	Categories []categoryProgress `json:"categories"`
}

func teamProgress(ctx context.Context, q database.Querier, teamID uuid.UUID) (progress, error) {
	totals, err := listCategoryTotals(ctx, q)
	if err != nil {
		return progress{}, err
	}
	solves, err := listTeamSolves(ctx, q, teamID)
	if err != nil {
		return progress{}, err
	}

	solvedIn := map[string]int{}
	for _, c := range solves {
		solvedIn[c.Category]++
	}

	p := progress{Categories: []categoryProgress{}}
	for _, t := range totals {
		p.Categories = append(p.Categories, categoryProgress{Category: t.Category, Total: t.Total, Solved: solvedIn[t.Category]})
		p.Total += t.Total
		p.Solved += solvedIn[t.Category]
	}
	return p, nil
}

type categoryComparison struct {
	Category string `json:"category"`
	Total    int    `json:"total"`
	Them     int    `json:"them"`
	Us       int    `json:"us"`
}

type comparison struct {
	Categories []categoryComparison `json:"categories"`
	Missing    []solvedChallenge    `json:"missing"`
}

func compareTeams(ctx context.Context, q database.Querier, theirs uuid.UUID, ours *uuid.UUID) (comparison, error) {
	totals, err := listCategoryTotals(ctx, q)
	if err != nil {
		return comparison{}, err
	}
	theirSolves, err := listTeamSolves(ctx, q, theirs)
	if err != nil {
		return comparison{}, err
	}
	var ourSolves []solvedChallenge
	if ours != nil {
		ourSolves, err = listTeamSolves(ctx, q, *ours)
		if err != nil {
			return comparison{}, err
		}
	}

	them := map[string]int{}
	for _, c := range theirSolves {
		them[c.Category]++
	}
	us := map[string]int{}
	ourIDs := map[uuid.UUID]bool{}
	for _, c := range ourSolves {
		us[c.Category]++
		ourIDs[c.ID] = true
	}

	out := comparison{Categories: []categoryComparison{}, Missing: []solvedChallenge{}}
	for _, t := range totals {
		out.Categories = append(out.Categories, categoryComparison{Category: t.Category, Total: t.Total, Them: them[t.Category], Us: us[t.Category]})
	}
	slices.SortStableFunc(out.Categories, func(a, b categoryComparison) int { return b.Them - a.Them })

	for _, c := range theirSolves {
		if !ourIDs[c.ID] {
			out.Missing = append(out.Missing, c)
		}
	}
	slices.SortFunc(out.Missing, func(a, b solvedChallenge) int {
		if a.Points != b.Points {
			return b.Points - a.Points
		}
		return strings.Compare(a.Title, b.Title)
	})
	return out, nil
}

type scorePoint struct {
	At    time.Time `json:"at"`
	Score int       `json:"score"`
}

type teamSeries struct {
	TeamID   uuid.UUID    `json:"team_id"`
	TeamName string       `json:"team_name"`
	Points   []scorePoint `json:"points"`
}

func scoreSeries(ctx context.Context, q database.Querier, teams []team) ([]teamSeries, error) {
	ids := make([]uuid.UUID, len(teams))
	series := make([]teamSeries, len(teams))
	index := make(map[uuid.UUID]int, len(teams))
	for i, t := range teams {
		ids[i] = t.ID
		series[i] = teamSeries{TeamID: t.ID, TeamName: t.Name}
		index[t.ID] = i
	}

	rows, err := q.Query(ctx, `SELECT s.team_id, s.solved_at, c.points FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE c.is_visible AND s.team_id = ANY($1) ORDER BY s.solved_at`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var teamID uuid.UUID
		var at time.Time
		var points int
		if err := rows.Scan(&teamID, &at, &points); err != nil {
			return nil, err
		}
		s := &series[index[teamID]]
		if n := len(s.Points); n > 0 {
			points += s.Points[n-1].Score
		}
		s.Points = append(s.Points, scorePoint{At: at, Score: points})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	solved := []teamSeries{}
	for _, s := range series {
		if len(s.Points) > 0 {
			solved = append(solved, s)
		}
	}
	return solved, nil
}

type player struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int       `json:"score"`
	Rank     int       `json:"rank"`
}

func listPlayers(ctx context.Context, q database.Querier, limit int) ([]player, error) {
	rows, err := q.Query(ctx, `SELECT id, username, score FROM users ORDER BY score DESC, username LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (player, error) {
		var p player
		err := row.Scan(&p.ID, &p.Username, &p.Score)
		return p, err
	})
}

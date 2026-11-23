package teams

import (
	"context"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
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
	Score       int        `json:"score"`
	Rank        int        `json:"rank"`
}

type member struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int32     `json:"score"`
	Solves   int       `json:"solves"`
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
	t, err := dbgen.New(q).GetTeam(ctx, dbgen.GetTeamParams{ID: id, IncludeHidden: includeHidden})
	if err != nil {
		return team{}, err
	}
	return team{ID: t.ID, Name: t.Name, CaptainID: t.CaptainID, InviteCode: t.InviteCode, Banned: t.Banned,
		Hidden: t.Hidden, MemberCount: int(t.MemberCount), Score: int(t.Score)}, nil
}

// lockTeam locks a team row for the rest of tx. Hidden teams are included: hiding a team
// must not stop its members or admins from changing it.
func lockTeam(ctx context.Context, tx pgx.Tx, id uuid.UUID) (team, error) {
	if err := dbgen.New(tx).LockTeam(ctx, id); err != nil {
		return team{}, err
	}
	return getTeam(ctx, tx, id, true)
}

func listMembers(ctx context.Context, q database.Querier, teamID uuid.UUID) ([]member, error) {
	rows, err := dbgen.New(q).ListTeamMembers(ctx, &teamID)
	if err != nil {
		return nil, err
	}
	out := make([]member, len(rows))
	for i, m := range rows {
		out[i] = member{ID: m.ID, Username: m.Username, Score: m.Score, Solves: int(m.Solves)}
	}
	return out, nil
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
	rows, err := dbgen.New(q).ListTeamActivity(ctx, &teamID)
	if err != nil {
		return nil, err
	}
	out := make([]activityEntry, len(rows))
	for i, r := range rows {
		out[i] = activityEntry{ChallengeID: r.ChallengeID, Username: r.Username, Challenge: r.Challenge,
			Category: r.Category, Points: int(r.Points), SolvedAt: r.SolvedAt}
	}
	return out, nil
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
	queries := dbgen.New(q)
	u, err := queries.GetProfile(ctx, userID)
	if err != nil {
		return profile{}, err
	}
	p := profile{ID: u.ID, Username: u.Username, Score: u.Score, TeamID: u.TeamID, Team: u.Team}
	rows, err := queries.ListProfileSolves(ctx, userID)
	if err != nil {
		return p, err
	}
	p.Solves = make([]profileSolve, len(rows))
	for i, r := range rows {
		p.Solves[i] = profileSolve{ChallengeID: r.ChallengeID, Challenge: r.Challenge, Category: r.Category,
			Points: int(r.Points), SolvedAt: r.SolvedAt}
	}
	return p, nil
}

// listTeams returns a page of teams sorted by score, then name. Hidden teams are left out unless includeHidden is set.
func listTeams(ctx context.Context, q database.Querier, limit, offset int, includeHidden bool) ([]team, error) {
	rows, err := dbgen.New(q).ListTeams(ctx, dbgen.ListTeamsParams{
		IncludeHidden: includeHidden, Lim: int32(limit), Off: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]team, len(rows))
	for i, t := range rows {
		out[i] = team{ID: t.ID, Name: t.Name, CaptainID: t.CaptainID, InviteCode: t.InviteCode, Banned: t.Banned,
			Hidden: t.Hidden, MemberCount: int(t.MemberCount), Score: int(t.Score)}
	}
	return out, nil
}

func teamRank(ctx context.Context, q database.Querier, score int) (int, error) {
	rank, err := dbgen.New(q).TeamRank(ctx, int64(score))
	return int(rank), err
}

type categoryTotal struct {
	Category string
	Total    int
}

func listCategoryTotals(ctx context.Context, q database.Querier) ([]categoryTotal, error) {
	rows, err := dbgen.New(q).ListCategoryTotals(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]categoryTotal, len(rows))
	for i, r := range rows {
		out[i] = categoryTotal{Category: r.Category, Total: int(r.Total)}
	}
	return out, nil
}

type solvedChallenge struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	Category string    `json:"category"`
	Points   int       `json:"points"`
}

func listTeamSolves(ctx context.Context, q database.Querier, teamID uuid.UUID) ([]solvedChallenge, error) {
	rows, err := dbgen.New(q).ListTeamSolves(ctx, &teamID)
	if err != nil {
		return nil, err
	}
	out := make([]solvedChallenge, len(rows))
	for i, r := range rows {
		out[i] = solvedChallenge{ID: r.ID, Title: r.Title, Category: r.Category, Points: int(r.Points)}
	}
	return out, nil
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

	rows, err := dbgen.New(q).ListTeamScoreEvents(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		s := &series[index[*r.TeamID]]
		points := int(r.Points)
		if n := len(s.Points); n > 0 {
			points += s.Points[n-1].Score
		}
		s.Points = append(s.Points, scorePoint{At: r.SolvedAt, Score: points})
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
	rows, err := dbgen.New(q).ListPlayers(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]player, len(rows))
	for i, r := range rows {
		out[i] = player{ID: r.ID, Username: r.Username, Score: int(r.Score)}
	}
	return out, nil
}

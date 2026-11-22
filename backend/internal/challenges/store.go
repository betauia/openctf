package challenges

import (
	"context"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/policy"
	"github.com/jackc/pgx/v5"
)

type flag struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Data    string `json:"data,omitempty"`
}

type challenge struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Description    *string    `json:"description,omitempty"`
	Author         *string    `json:"author,omitempty"`
	Points         int        `json:"points"`
	Category       string     `json:"category"`
	Difficulty     string     `json:"difficulty"`
	Solves         int        `json:"solves"`
	ConnectionInfo *string    `json:"connection_info,omitempty"`
	Files          []string   `json:"files"`
	DockerImage    *string    `json:"docker_image,omitempty"`
	DockerPort     int        `json:"docker_port"`
	IsVisible      bool       `json:"is_visible"`
	CreatedByID    *uuid.UUID `json:"created_by_id,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type challengeFile struct {
	ID          uuid.UUID  `json:"id"`
	ChallengeID *uuid.UUID `json:"challenge_id"`
	Filename    string     `json:"filename"`
	Filepath    string     `json:"filepath"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

const challengeColumns = `c.id,c.title,c.description,c.author,c.points,c.category,c.difficulty,c.solves,c.connection_info,c.docker_image,c.docker_port,c.is_visible,c.created_by_id,c.uploaded_at`
const challengeFileColumns = `cf.id,cf.challenge_id,cf.filename,cf.filepath,cf.uploaded_at`

func (c challenge) resource() policy.Resource {
	owner := ""
	if c.CreatedByID != nil {
		owner = c.CreatedByID.String()
	}
	return policy.Resource{ID: c.ID.String(), Attributes: map[string]any{"visible": c.IsVisible, "owner_id": owner}}
}

func scanChallenge(row pgx.Row) (challenge, error) {
	var c challenge
	err := row.Scan(&c.ID, &c.Title, &c.Description, &c.Author, &c.Points, &c.Category, &c.Difficulty, &c.Solves, &c.ConnectionInfo, &c.DockerImage, &c.DockerPort, &c.IsVisible, &c.CreatedByID, &c.UpdatedAt)
	return c, err
}
func scanChallengeFile(row pgx.Row) (challengeFile, error) {
	var f challengeFile
	err := row.Scan(&f.ID, &f.ChallengeID, &f.Filename, &f.Filepath, &f.UpdatedAt)
	return f, err
}

func listChallenges(ctx context.Context, q database.Querier, isAdmin bool) ([]challenge, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if !isAdmin {
		rows, err = q.Query(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.is_visible = TRUE ORDER BY c.category")
	} else {
		rows, err = q.Query(ctx, "SELECT "+challengeColumns+" FROM challenges c ORDER BY c.category")
	}
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (challenge, error) {
		return scanChallenge(row)
	})
}

func listChallengeByID(ctx context.Context, q database.Querier, challengeID uuid.UUID, isAdmin bool) (challenge, error) {
	var row pgx.Row

	if !isAdmin {
		row = q.QueryRow(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.is_visible = TRUE AND c.id = $1", challengeID)
	} else {
		row = q.QueryRow(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.id = $1", challengeID)
	}
	return scanChallenge(row)
}

func getChallengeFiles(ctx context.Context, q database.Querier, challengeID uuid.UUID, isAdmin bool) ([]challengeFile, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if !isAdmin {
		rows, err = q.Query(ctx, "SELECT "+challengeFileColumns+" FROM challenge_files cf JOIN challenges c ON c.id = cf.challenge_id WHERE cf.challenge_id = $1 AND c.is_visible = TRUE ORDER BY cf.filename", challengeID)
	} else {
		rows, err = q.Query(ctx, "SELECT "+challengeFileColumns+" FROM challenge_files cf JOIN challenges c ON c.id = cf.challenge_id WHERE cf.challenge_id = $1 ORDER BY cf.filename", challengeID)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (challengeFile, error) {
		return scanChallengeFile(row)
	})
}

// getChallengeFile returns the file fileID belonging to challengeID. Players only see
// files of visible challenges.
func getChallengeFile(ctx context.Context, q database.Querier, challengeID, fileID uuid.UUID, isAdmin bool) (challengeFile, error) {
	var row pgx.Row
	if !isAdmin {
		row = q.QueryRow(ctx, "SELECT "+challengeFileColumns+" FROM challenge_files cf JOIN challenges c ON c.id = cf.challenge_id WHERE cf.id = $1 AND cf.challenge_id = $2 AND c.is_visible = TRUE", fileID, challengeID)
	} else {
		row = q.QueryRow(ctx, "SELECT "+challengeFileColumns+" FROM challenge_files cf JOIN challenges c ON c.id = cf.challenge_id WHERE cf.id = $1 AND cf.challenge_id = $2", fileID, challengeID)
	}
	return scanChallengeFile(row)
}

type solve struct {
	UserID   uuid.UUID  `json:"uid"`
	Username string     `json:"username"`
	TeamID   *uuid.UUID `json:"team_id"`
	Team     *string    `json:"team"`
	SolvedAt time.Time  `json:"solved_at"`
}

func listSolves(ctx context.Context, q database.Querier, challengeID uuid.UUID) ([]solve, error) {
	rows, err := q.Query(ctx, `SELECT u.id, u.username, t.id, t.name, s.solved_at FROM solves s JOIN users u ON u.id = s.user_id LEFT JOIN teams t ON t.id = u.team_id AND NOT t.hidden WHERE s.challenge_id = $1 ORDER BY s.solved_at`, challengeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (solve, error) {
		var s solve
		err := row.Scan(&s.UserID, &s.Username, &s.TeamID, &s.Team, &s.SolvedAt)
		return s, err
	})
}

type submitResult struct {
	Correct     bool       `json:"correct"`
	ChallengeID *uuid.UUID `json:"challenge_id"`
	Points      int        `json:"points"`
}

func findChallengeByFlag(ctx context.Context, q database.Querier, flag string, challengeID *uuid.UUID) (challenge, error) {
	if challengeID != nil {
		return scanChallenge(q.QueryRow(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.is_visible = TRUE AND c.id = $1 AND c.flags @> jsonb_build_array($2::text)", *challengeID, flag))
	}
	return scanChallenge(q.QueryRow(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.is_visible = TRUE AND c.flags @> jsonb_build_array($1::text)", flag))
}

func recordSolve(ctx context.Context, q database.Querier, userID, challengeID uuid.UUID, teamID *uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `INSERT INTO solves (user_id, challenge_id) SELECT $1, $2 WHERE NOT EXISTS (SELECT 1 FROM solves s JOIN users m ON m.id = s.user_id WHERE s.challenge_id = $2 AND m.team_id = $3) ON CONFLICT DO NOTHING`, userID, challengeID, teamID)
	return tag.RowsAffected() == 1, err
}

type solveEvent struct {
	ChallengeID uuid.UUID  `json:"challenge_id"`
	Challenge   string     `json:"challenge"`
	Category    string     `json:"category"`
	Points      int        `json:"points"`
	UserID      uuid.UUID  `json:"user_id"`
	Username    string     `json:"username"`
	TeamID      *uuid.UUID `json:"team_id"`
	Team        *string    `json:"team"`
	SolvedAt    time.Time  `json:"solved_at"`
}

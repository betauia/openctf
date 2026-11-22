package challenges

import (
	"context"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
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
		row = q.QueryRow(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.is_visible = TRUE AND c.id = $i", challengeID)
	} else {
		row = q.QueryRow(ctx, "SELECT "+challengeColumns+" FROM challenges c WHERE c.id = $i", challengeID)
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

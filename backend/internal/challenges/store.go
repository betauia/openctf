package challenges

import (
	"context"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/policy"
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

func (c challenge) resource() policy.Resource {
	owner := ""
	if c.CreatedByID != nil {
		owner = c.CreatedByID.String()
	}
	return policy.Resource{ID: c.ID.String(), Attributes: map[string]any{"visible": c.IsVisible, "owner_id": owner}}
}

func fromChallenge(c dbgen.Challenge) challenge {
	var port int
	if c.DockerPort != nil {
		port = int(*c.DockerPort)
	}
	return challenge{
		ID:             c.ID,
		Title:          c.Title,
		Description:    c.Description,
		Author:         c.Author,
		Points:         int(c.Points),
		Category:       c.Category,
		Difficulty:     c.Difficulty,
		Solves:         int(c.Solves),
		ConnectionInfo: c.ConnectionInfo,
		DockerImage:    c.DockerImage,
		DockerPort:     port,
		IsVisible:      c.IsVisible,
		CreatedByID:    c.CreatedByID,
		UpdatedAt:      c.UploadedAt,
	}
}

func fromChallengeFile(f dbgen.ChallengeFile) challengeFile {
	return challengeFile{
		ID:          f.ID,
		ChallengeID: &f.ChallengeID,
		Filename:    f.Filename,
		Filepath:    f.Filepath,
		UpdatedAt:   f.UploadedAt,
	}
}

// listChallenges returns every challenge, sorted by category. Players only see visible ones.
func listChallenges(ctx context.Context, q database.Querier, isAdmin bool) ([]challenge, error) {
	rows, err := dbgen.New(q).ListChallenges(ctx, isAdmin)
	if err != nil {
		return nil, err
	}
	out := make([]challenge, len(rows))
	for i, c := range rows {
		out[i] = fromChallenge(c)
	}
	return out, nil
}

func listChallengeByID(ctx context.Context, q database.Querier, challengeID uuid.UUID, isAdmin bool) (challenge, error) {
	c, err := dbgen.New(q).GetChallenge(ctx, dbgen.GetChallengeParams{ID: challengeID, IncludeHidden: isAdmin})
	if err != nil {
		return challenge{}, err
	}
	return fromChallenge(c), nil
}

func getChallengeFiles(ctx context.Context, q database.Querier, challengeID uuid.UUID, isAdmin bool) ([]challengeFile, error) {
	rows, err := dbgen.New(q).ListChallengeFiles(ctx, dbgen.ListChallengeFilesParams{ChallengeID: challengeID, IncludeHidden: isAdmin})
	if err != nil {
		return nil, err
	}
	out := make([]challengeFile, len(rows))
	for i, f := range rows {
		out[i] = fromChallengeFile(f)
	}
	return out, nil
}

// getChallengeFile returns the file fileID belonging to challengeID. Players only see
// files of visible challenges.
func getChallengeFile(ctx context.Context, q database.Querier, challengeID, fileID uuid.UUID, isAdmin bool) (challengeFile, error) {
	f, err := dbgen.New(q).GetChallengeFile(ctx, dbgen.GetChallengeFileParams{ID: fileID, ChallengeID: challengeID, IncludeHidden: isAdmin})
	if err != nil {
		return challengeFile{}, err
	}
	return fromChallengeFile(f), nil
}

type solve struct {
	UserID   uuid.UUID  `json:"uid"`
	Username string     `json:"username"`
	TeamID   *uuid.UUID `json:"team_id"`
	Team     *string    `json:"team"`
	SolvedAt time.Time  `json:"solved_at"`
}

func listSolves(ctx context.Context, q database.Querier, challengeID uuid.UUID) ([]solve, error) {
	rows, err := dbgen.New(q).ListChallengeSolves(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	out := make([]solve, len(rows))
	for i, r := range rows {
		out[i] = solve{UserID: r.UserID, Username: r.Username, TeamID: r.TeamID, Team: r.Team, SolvedAt: r.SolvedAt}
	}
	return out, nil
}

type submitResult struct {
	Correct     bool       `json:"correct"`
	ChallengeID *uuid.UUID `json:"challenge_id"`
	Points      int        `json:"points"`
}

func findChallengeByFlag(ctx context.Context, q database.Querier, flag string, challengeID *uuid.UUID) (challenge, error) {
	c, err := dbgen.New(q).FindChallengeByFlag(ctx, dbgen.FindChallengeByFlagParams{ChallengeID: challengeID, Flag: flag})
	if err != nil {
		return challenge{}, err
	}
	return fromChallenge(c), nil
}

func recordSolve(ctx context.Context, q database.Querier, userID, challengeID uuid.UUID) (bool, error) {
	n, err := dbgen.New(q).RecordSolve(ctx, dbgen.RecordSolveParams{UserID: userID, ChallengeID: challengeID})
	return n == 1, err
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

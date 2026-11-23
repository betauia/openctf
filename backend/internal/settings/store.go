package settings

import (
	"context"
	"time"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
)

type Settings struct {
	CTFName      string     `json:"ctf_name"`
	CTFStart     *time.Time `json:"ctf_start"`
	CTFEnd       *time.Time `json:"ctf_end"`
	Registration bool       `json:"registration"`
	MaxTeamSize  int        `json:"max_team_size"`
	Rules        string     `json:"rules"`
	Theme        string     `json:"theme"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func load(ctx context.Context, q database.Querier) (Settings, error) {
	row, err := dbgen.New(q).GetSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		CTFName:      row.CtfName,
		CTFStart:     &row.CtfStart,
		CTFEnd:       &row.CtfEnd,
		Registration: row.Registration,
		MaxTeamSize:  int(row.MaxTeamSize),
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

var Themes = []string{"uiactf", "betauia"}

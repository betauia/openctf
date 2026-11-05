package settings

import (
	"context"
	"time"

	"github.com/betauia/openctf/backend/internal/database"
)

type Settings struct {
	CTFName      string     `json:"ctf_name"`
	CTFStart     *time.Time `json:"ctf_start"`
	CTFEnd       *time.Time `json:"ctf_end"`
	Registration bool       `json:"registration"`
	MaxTeamSize  int        `json:"max_team_size"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

const settingsColumns = `ctf_name, ctf_start, ctf_end, registration, max_team_size, updated_at`

func load(ctx context.Context, q database.Querier) (Settings, error) {
	var s Settings
	err := q.QueryRow(ctx, `SELECT `+settingsColumns+` FROM settings`).Scan(
		&s.CTFName, &s.CTFStart, &s.CTFEnd, &s.Registration, &s.MaxTeamSize, &s.UpdatedAt)
	return s, err
}

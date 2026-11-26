-- name: GetSettings :one
SELECT * FROM settings;

-- name: LockSettingsWindow :one
-- Lock the row so the start/end check sees the values it is checked against.
SELECT ctf_start, ctf_end FROM settings FOR UPDATE;

-- name: UpdateSettings :exec
UPDATE settings SET
    ctf_name      = COALESCE(sqlc.narg('ctf_name'), ctf_name),
    ctf_start     = @ctf_start,
    ctf_end       = @ctf_end,
    registration  = COALESCE(sqlc.narg('registration'), registration),
    max_team_size = COALESCE(sqlc.narg('max_team_size'), max_team_size);

-- name: GetPolicyEnv :one
SELECT ctf_start, ctf_end, max_team_size, registration,
       (SELECT COUNT(*) FROM users WHERE role = 'admin')::int AS admin_count
FROM settings;

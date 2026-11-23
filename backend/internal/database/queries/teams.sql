-- name: GetTeam :one
-- Unless include_hidden is set, a hidden team is not returned.
SELECT t.id, t.name, t.captain_id, t.invite_code, t.banned, t.hidden,
       (SELECT COUNT(*) FROM users u WHERE u.team_id = t.id)::int AS member_count,
       (SELECT COALESCE(SUM(c.points), 0) FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE s.team_id = t.id)::int AS score
FROM teams t
WHERE t.id = @id AND (@include_hidden::bool OR NOT t.hidden);

-- name: ListTeams :many
-- A page of teams sorted by score, then name. Hidden teams are left out unless include_hidden is set.
SELECT t.id, t.name, t.captain_id, t.invite_code, t.banned, t.hidden,
       (SELECT COUNT(*) FROM users u WHERE u.team_id = t.id)::int AS member_count,
       (SELECT COALESCE(SUM(c.points), 0) FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE s.team_id = t.id)::int AS score
FROM teams t
WHERE @include_hidden::bool OR NOT t.hidden
ORDER BY score DESC, t.name ASC
LIMIT @lim OFFSET @off;

-- name: LockTeam :exec
SELECT 1 FROM teams WHERE id = $1 FOR UPDATE;

-- name: ListTeamMembers :many
SELECT u.id, u.username, u.score, (SELECT COUNT(*) FROM solves s WHERE s.user_id = u.id)::int AS solves
FROM users u WHERE u.team_id = $1 ORDER BY u.score DESC, u.username;

-- name: GetTeamIDByInviteCode :one
SELECT id FROM teams WHERE invite_code = $1;

-- name: CreateTeam :exec
INSERT INTO teams (id, name, captain_id, invite_code) VALUES ($1, $2, $3, $4);

-- name: UpdateTeamByCaptain :exec
UPDATE teams SET
    name        = COALESCE(sqlc.narg('name'), name),
    invite_code = COALESCE(sqlc.narg('invite_code'), invite_code)
WHERE id = @id;

-- name: AdminUpdateTeam :execrows
-- The EXISTS keeps the update from running at all when captain_id isn't a member.
UPDATE teams SET
    name       = COALESCE(sqlc.narg('name'), name),
    banned     = COALESCE(sqlc.narg('banned'), banned),
    hidden     = COALESCE(sqlc.narg('hidden'), hidden),
    captain_id = COALESCE(sqlc.narg('captain_id'), captain_id)
WHERE teams.id = @id
  AND (sqlc.narg('captain_id')::uuid IS NULL
       OR EXISTS (SELECT 1 FROM users WHERE users.id = sqlc.narg('captain_id') AND users.team_id = @id));

-- name: TransferCaptain :execrows
UPDATE teams SET captain_id = @captain_id
WHERE teams.id = @id
  AND EXISTS (SELECT 1 FROM users WHERE users.id = @captain_id AND users.team_id = @id);

-- name: ReassignCaptain :exec
-- UUIDv7 ids sort by creation time, so this picks the oldest remaining account.
UPDATE teams SET captain_id = (SELECT u.id FROM users u WHERE u.team_id = @id ORDER BY u.id LIMIT 1)
WHERE teams.id = @id;

-- name: DeleteTeam :exec
DELETE FROM teams WHERE id = $1;

-- name: JoinTeam :execrows
-- team_id IS NULL: the user may have joined a team since they were loaded.
UPDATE users SET team_id = @team_id WHERE id = @user_id AND team_id IS NULL;

-- name: LeaveTeam :exec
UPDATE users SET team_id = NULL WHERE id = $1;

-- name: RemoveTeamMember :execrows
UPDATE users SET team_id = NULL WHERE id = @user_id AND team_id = @team_id;

-- name: ListTeamActivity :many
SELECT c.id AS challenge_id, u.username, c.title AS challenge, c.category, c.points, s.solved_at
FROM solves s
JOIN users u ON u.id = s.user_id
JOIN challenges c ON c.id = s.challenge_id AND c.is_visible
WHERE s.team_id = $1
ORDER BY s.solved_at DESC;

-- name: GetProfile :one
-- A user's public profile. The team is left out when it is hidden.
SELECT u.id, u.username, u.score, t.id AS team_id, t.name AS team
FROM users u LEFT JOIN teams t ON t.id = u.team_id AND NOT t.hidden
WHERE u.id = $1 AND NOT u.banned;

-- name: ListProfileSolves :many
SELECT c.id AS challenge_id, c.title AS challenge, c.category, c.points, s.solved_at
FROM solves s
JOIN challenges c ON c.id = s.challenge_id AND c.is_visible
WHERE s.user_id = $1
ORDER BY s.solved_at DESC;

-- name: TeamRank :one
-- 1 + the number of visible teams with a higher score.
SELECT (COUNT(*) + 1)::int AS rank FROM teams t
WHERE NOT t.hidden
  AND (SELECT COALESCE(SUM(c.points), 0) FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE s.team_id = t.id) > @score::bigint;

-- name: ListCategoryTotals :many
SELECT category, COUNT(*)::int AS total FROM challenges WHERE is_visible GROUP BY category ORDER BY category;

-- name: ListTeamSolves :many
SELECT c.id, c.title, c.category, c.points
FROM solves s JOIN challenges c ON c.id = s.challenge_id
WHERE c.is_visible AND s.team_id = $1;

-- name: ListTeamScoreEvents :many
SELECT s.team_id, s.solved_at, c.points
FROM solves s JOIN challenges c ON c.id = s.challenge_id
WHERE c.is_visible AND s.team_id = ANY(@team_ids::uuid[])
ORDER BY s.solved_at;

-- name: ListPlayers :many
SELECT u.id, u.username, u.score
FROM users u LEFT JOIN teams t ON t.id = u.team_id
WHERE NOT u.banned AND NOT COALESCE(t.hidden, false)
ORDER BY u.score DESC, u.username
LIMIT $1;

-- name: ListTeams :many
SELECT * FROM teams ORDER BY id;

-- name: GetTeamByID :one
SELECT * FROM teams WHERE id = $1;

-- name: GetTeamByInviteCode :one
SELECT * FROM teams WHERE invite_code = $1;

-- name: TeamNameTaken :one
SELECT EXISTS (SELECT 1 FROM teams WHERE name = $1);

-- name: CreateTeam :one
INSERT INTO teams (id, name, invite_code)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListMembers :many
-- Every user that is in a team, with their solve count.
SELECT u.id, u.username, u.score, u.team_id, COUNT(s.id) AS solves
FROM users u
LEFT JOIN solves s ON s.user_id = u.id
WHERE u.team_id IS NOT NULL
GROUP BY u.id
ORDER BY u.id;

-- name: ListTeamMembers :many
SELECT u.id, u.username, u.score, u.team_id, COUNT(s.id) AS solves
FROM users u
LEFT JOIN solves s ON s.user_id = u.id
WHERE u.team_id = $1
GROUP BY u.id
ORDER BY u.id;

-- name: ListTeamActivity :many
SELECT u.id AS user_id, u.username, c.title AS challenge, c.category, c.points, s.solved_at
FROM solves s
JOIN users u ON u.id = s.user_id
JOIN challenges c ON c.id = s.challenge_id
WHERE u.team_id = $1
ORDER BY s.solved_at DESC, s.id DESC
LIMIT 50;

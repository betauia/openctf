-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: UserExists :one
SELECT EXISTS (
    SELECT 1 FROM users WHERE username = sqlc.arg(username) OR email = sqlc.arg(email)
);

-- name: CreateUser :one
INSERT INTO users (id, username, email, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UserRank :one
SELECT COUNT(*) + 1 FROM users WHERE score > $1;

-- name: AddUserScore :exec
UPDATE users SET score = score + sqlc.arg(points) WHERE id = sqlc.arg(id);

-- name: SetUserTeam :exec
UPDATE users SET team_id = $2 WHERE id = $1;

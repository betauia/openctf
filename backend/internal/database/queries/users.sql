-- name: GetSessionUser :one
-- The user, provided the refresh token family the session belongs to is still live.
SELECT * FROM users
WHERE users.id = @id
  AND EXISTS (SELECT 1 FROM refresh_tokens WHERE family_id = @family_id AND user_id = @id AND revoked_at IS NULL);

-- name: GetUserByUsername :one
SELECT * FROM users WHERE lower(username) = lower($1);

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE lower(username) = lower(@username) OR email = @email);

-- name: CreateUser :one
INSERT INTO users (id, username, email, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: EnsureAdmin :exec
INSERT INTO users (id, username, email, password_hash, role)
VALUES ($1, 'admin', $2, $3, 'admin')
ON CONFLICT (username) DO NOTHING;

-- name: UserRank :one
-- 1 + the number of users with a higher score.
SELECT (COUNT(*) + 1)::int AS rank FROM users WHERE score > $1;

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: GetSubjectTeam :one
-- COALESCE: captain_id is NULL when the team has no captain.
SELECT COALESCE(captain_id = @user_id::uuid, false)::bool AS is_captain, banned
FROM teams WHERE id = @team_id;


-- name: AddUserScore :exec
UPDATE users SET score = score + @points WHERE id = @id;

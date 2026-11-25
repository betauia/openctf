-- name: ListChallenges :many
-- Unless include_hidden is set, only visible challenges are returned.
SELECT * FROM challenges
WHERE @include_hidden::bool OR is_visible
ORDER BY category;

-- name: GetChallenge :one
SELECT * FROM challenges
WHERE id = @id AND (@include_hidden::bool OR is_visible);

-- name: FindChallengeByFlag :one
-- A visible challenge with this flag. A NULL challenge_id searches all of them.
SELECT * FROM challenges
WHERE is_visible AND (sqlc.narg('challenge_id')::uuid IS NULL OR id = sqlc.narg('challenge_id'))
  AND flags @> jsonb_build_array(@flag::text);

-- name: ListChallengeFiles :many
SELECT cf.* FROM challenge_files cf
JOIN challenges c ON c.id = cf.challenge_id
WHERE cf.challenge_id = @challenge_id AND (@include_hidden::bool OR c.is_visible)
ORDER BY cf.filename;

-- name: GetChallengeFile :one
SELECT cf.* FROM challenge_files cf
JOIN challenges c ON c.id = cf.challenge_id
WHERE cf.id = @id AND cf.challenge_id = @challenge_id AND (@include_hidden::bool OR c.is_visible);

-- name: IncrementChallengeSolves :exec
UPDATE challenges SET solves = solves + 1 WHERE id = $1;

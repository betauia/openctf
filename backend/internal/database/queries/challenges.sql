-- name: ListVisibleChallenges :many
SELECT * FROM challenges WHERE is_visible ORDER BY id;

-- name: GetVisibleChallenge :one
SELECT * FROM challenges WHERE id = $1 AND is_visible;

-- name: IncrementChallengeSolves :exec
UPDATE challenges SET solves = solves + 1 WHERE id = $1;

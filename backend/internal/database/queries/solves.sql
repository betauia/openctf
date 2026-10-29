-- name: ListSolvedChallengeIDs :many
SELECT challenge_id FROM solves WHERE user_id = $1 ORDER BY challenge_id;

-- name: CreateSolve :execrows
-- Returns 0 rows affected when the user already solved the challenge.
INSERT INTO solves (user_id, challenge_id)
VALUES ($1, $2)
ON CONFLICT (user_id, challenge_id) DO NOTHING;

-- name: ListChallengeSolves :many
SELECT u.id AS uid, u.username, t.name AS team, s.solved_at
FROM solves s
JOIN users u ON u.id = s.user_id
LEFT JOIN teams t ON t.id = u.team_id
WHERE s.challenge_id = $1
ORDER BY s.solved_at, s.id;

-- name: ListUserSolves :many
SELECT c.id AS challenge_id, c.title AS challenge, c.category, c.points, s.solved_at
FROM solves s
JOIN challenges c ON c.id = s.challenge_id
WHERE s.user_id = $1
ORDER BY s.solved_at DESC, s.id DESC;

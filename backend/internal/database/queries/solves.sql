-- name: ListSolvedChallengeIDs :many
-- The challenges solved by the user's team, or by the user alone when they are not on a team.
SELECT DISTINCT s.challenge_id FROM solves s
JOIN users u ON u.id = $1
WHERE (u.team_id IS NOT NULL AND s.team_id = u.team_id)
   OR (u.team_id IS NULL AND s.user_id = u.id)
ORDER BY s.challenge_id;

-- name: ListChallengeSolves :many
SELECT u.id AS user_id, u.username, t.id AS team_id, t.name AS team, s.solved_at
FROM solves s
JOIN users u ON u.id = s.user_id
LEFT JOIN teams t ON t.id = s.team_id AND NOT t.hidden
WHERE s.challenge_id = $1
ORDER BY s.solved_at;

-- name: RecordSolve :execrows
INSERT INTO solves (user_id, team_id, challenge_id)
SELECT u.id, u.team_id, @challenge_id FROM users u WHERE u.id = @user_id AND u.team_id IS NOT NULL
ON CONFLICT DO NOTHING;

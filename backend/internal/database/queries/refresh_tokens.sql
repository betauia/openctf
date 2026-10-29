-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ConsumeRefreshToken :one
-- Marks a live token as used and returns its owner. A token first used after
-- reuse_after can be consumed again. No row means unknown, expired, revoked or replayed.
UPDATE refresh_tokens SET used_at = COALESCE(used_at, now())
WHERE token_hash = sqlc.arg(token_hash) AND revoked_at IS NULL AND expires_at > now()
  AND (used_at IS NULL OR used_at > sqlc.arg(reuse_after)::timestamptz)
RETURNING user_id, family_id;

-- name: GetRefreshTokenFamily :one
SELECT family_id FROM refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshFamily :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL;

-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < now();

-- name: CreateSession :one
INSERT INTO session (id, user_id, refresh_token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, refresh_token_hash, expires_at, created_at, revoked_at;

-- name: GetSessionByTokenHash :one
SELECT id, user_id, refresh_token_hash, expires_at, created_at, revoked_at
FROM session
WHERE refresh_token_hash = $1;

-- name: RevokeSession :exec
UPDATE session
SET revoked_at = now()
WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE session
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

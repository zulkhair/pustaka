-- name: CreateEmailVerification :one
INSERT INTO email_verification (id, user_id, code_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, code_hash, expires_at, attempts, consumed_at, created_at;

-- name: GetActiveEmailVerification :one
SELECT id, user_id, code_hash, expires_at, attempts, consumed_at, created_at
FROM email_verification
WHERE user_id = $1 AND consumed_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementVerificationAttempts :one
UPDATE email_verification
SET attempts = attempts + 1
WHERE id = $1
RETURNING attempts;

-- name: ConsumeEmailVerification :exec
UPDATE email_verification
SET consumed_at = now()
WHERE id = $1;

-- name: DeleteEmailVerificationsByUser :exec
DELETE FROM email_verification
WHERE user_id = $1;

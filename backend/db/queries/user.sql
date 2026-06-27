-- name: CreateUser :one
INSERT INTO web_user (id, username, email, password_hash, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, username, email, password_hash, role, email_verified, created_at;

-- name: GetUserByEmail :one
SELECT id, username, email, password_hash, role, email_verified, created_at
FROM web_user
WHERE email = $1;

-- name: GetUserByUsername :one
SELECT id, username, email, password_hash, role, email_verified, created_at
FROM web_user
WHERE username = $1;

-- name: GetUserByID :one
SELECT id, username, email, password_hash, role, email_verified, created_at
FROM web_user
WHERE id = $1;

-- name: SetUserEmailVerified :exec
UPDATE web_user
SET email_verified = true
WHERE id = $1;

-- name: CreateDocument :one
INSERT INTO document (id, user_id, title, mode)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, title, mode, page_count, status, created_at;

-- name: GetDocument :one
SELECT id, user_id, title, mode, page_count, status, created_at
FROM document
WHERE id = $1;

-- name: ListDocumentsByUser :many
SELECT id, user_id, title, mode, page_count, status, created_at
FROM document
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: SetDocumentStatus :exec
UPDATE document SET status = $2 WHERE id = $1;

-- name: IncrementDocumentPageCount :one
UPDATE document SET page_count = page_count + 1 WHERE id = $1 RETURNING page_count;

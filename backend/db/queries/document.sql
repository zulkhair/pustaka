-- name: CreateDocument :one
INSERT INTO document (id, user_id, title, mode)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, title, mode, page_count, status, created_at, deleted_at;

-- name: GetDocument :one
SELECT id, user_id, title, mode, page_count, status, created_at, deleted_at
FROM document
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListDocumentsByUser :many
SELECT id, user_id, title, mode, page_count, status, created_at, deleted_at
FROM document
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: SetDocumentStatus :exec
UPDATE document SET status = $2 WHERE id = $1;

-- name: IncrementDocumentPageCount :one
UPDATE document SET page_count = page_count + 1 WHERE id = $1 RETURNING page_count;

-- name: UpdateDocumentTitle :one
UPDATE document SET title = $2 WHERE id = $1
RETURNING id, user_id, title, mode, page_count, status, created_at, deleted_at;

-- name: SoftDeleteDocument :exec
UPDATE document SET deleted_at = now() WHERE id = $1;

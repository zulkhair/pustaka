-- name: CreateOutput :one
INSERT INTO output (id, user_id, document_id, template_id, content, file_path, model, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, document_id, template_id, content, file_path, model, status, created_at;

-- name: GetOutput :one
SELECT id, user_id, document_id, template_id, content, file_path, model, status, created_at
FROM output
WHERE id = $1;

-- name: ListOutputsByDocument :many
SELECT id, user_id, document_id, template_id, content, file_path, model, status, created_at
FROM output
WHERE document_id = $1
ORDER BY created_at DESC;

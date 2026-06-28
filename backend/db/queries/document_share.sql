-- name: CreateShare :one
INSERT INTO document_share (id, document_id, shared_with_user_id, permission)
VALUES ($1, $2, $3, $4)
ON CONFLICT (document_id, shared_with_user_id)
DO UPDATE SET permission = EXCLUDED.permission
RETURNING id, document_id, shared_with_user_id, permission, created_at;

-- name: ListSharesForDocument :many
SELECT id, document_id, shared_with_user_id, permission, created_at
FROM document_share
WHERE document_id = $1
ORDER BY created_at ASC;

-- name: GetShare :one
SELECT id, document_id, shared_with_user_id, permission, created_at
FROM document_share
WHERE document_id = $1 AND shared_with_user_id = $2;

-- name: DeleteShare :exec
DELETE FROM document_share
WHERE document_id = $1 AND shared_with_user_id = $2;

-- name: ListDocumentsSharedWith :many
SELECT d.id, d.user_id, d.title, d.mode, d.page_count, d.status, d.created_at, d.deleted_at
FROM document d
JOIN document_share s ON s.document_id = d.id
WHERE s.shared_with_user_id = $1 AND d.deleted_at IS NULL
ORDER BY d.created_at DESC;

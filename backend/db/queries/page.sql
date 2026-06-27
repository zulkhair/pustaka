-- name: CreatePage :one
INSERT INTO page (id, document_id, page_number, image_path, thumb_path, width, height, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, document_id, page_number, image_path, thumb_path, width, height, status;

-- name: GetPageByNumber :one
SELECT id, document_id, page_number, image_path, thumb_path, width, height, status
FROM page
WHERE document_id = $1 AND page_number = $2;

-- name: ListPagesByDocument :many
SELECT id, document_id, page_number, image_path, thumb_path, width, height, status
FROM page
WHERE document_id = $1
ORDER BY page_number ASC;

-- name: SetPageStatus :exec
UPDATE page SET status = $2 WHERE id = $1;

-- name: ClearPageImage :exec
UPDATE page SET image_path = NULL, thumb_path = NULL WHERE id = $1;

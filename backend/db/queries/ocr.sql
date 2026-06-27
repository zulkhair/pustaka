-- name: CreateOCRResult :one
INSERT INTO ocr_result (id, page_id, model, text, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, page_id, model, text, status, created_at;

-- name: GetLatestOCRResult :one
SELECT id, page_id, model, text, status, created_at
FROM ocr_result
WHERE page_id = $1
ORDER BY created_at DESC
LIMIT 1;

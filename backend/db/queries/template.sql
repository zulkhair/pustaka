-- name: ListTemplates :many
SELECT id, owner_user_id, name, doc_type_hint, scope, prompt, output_format, json_schema, is_builtin
FROM template
WHERE is_builtin = true OR owner_user_id IS NOT NULL
ORDER BY is_builtin DESC, name ASC;

-- name: GetTemplate :one
SELECT id, owner_user_id, name, doc_type_hint, scope, prompt, output_format, json_schema, is_builtin
FROM template
WHERE id = $1;

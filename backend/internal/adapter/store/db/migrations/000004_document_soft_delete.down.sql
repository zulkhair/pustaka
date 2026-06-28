DROP INDEX IF EXISTS idx_document_deleted_at;
ALTER TABLE document DROP COLUMN IF EXISTS deleted_at;

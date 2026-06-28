ALTER TABLE document ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_document_deleted_at ON document (deleted_at);

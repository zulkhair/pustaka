CREATE TABLE document_share (
    id                  VARCHAR(36) PRIMARY KEY,
    document_id         VARCHAR(36) NOT NULL REFERENCES document(id) ON DELETE CASCADE,
    shared_with_user_id VARCHAR(36) NOT NULL REFERENCES web_user(id) ON DELETE CASCADE,
    permission          VARCHAR(10) NOT NULL DEFAULT 'viewer' CHECK (permission IN ('viewer', 'editor')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, shared_with_user_id)
);

CREATE INDEX idx_document_share_doc ON document_share (document_id);
CREATE INDEX idx_document_share_user ON document_share (shared_with_user_id);

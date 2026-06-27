CREATE TABLE document (
    id         VARCHAR(36)  PRIMARY KEY,
    user_id    VARCHAR(36)  NOT NULL REFERENCES web_user(id) ON DELETE CASCADE,
    title      VARCHAR(255) NOT NULL,
    mode       VARCHAR(10)  NOT NULL CHECK (mode IN ('photo', 'text')),
    page_count INT          NOT NULL DEFAULT 0,
    status     VARCHAR(12)  NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE page (
    id          VARCHAR(36) PRIMARY KEY,
    document_id VARCHAR(36) NOT NULL REFERENCES document(id) ON DELETE CASCADE,
    page_number INT         NOT NULL,
    image_path  TEXT,
    thumb_path  TEXT,
    width       INT         NOT NULL DEFAULT 0,
    height      INT         NOT NULL DEFAULT 0,
    status      VARCHAR(12) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    UNIQUE (document_id, page_number)
);

CREATE TABLE ocr_result (
    id         VARCHAR(36)  PRIMARY KEY,
    page_id    VARCHAR(36)  NOT NULL REFERENCES page(id) ON DELETE CASCADE,
    model      VARCHAR(100) NOT NULL,
    text       TEXT         NOT NULL,
    status     VARCHAR(12)  NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE template (
    id            VARCHAR(36)  PRIMARY KEY,
    owner_user_id VARCHAR(36)  REFERENCES web_user(id) ON DELETE CASCADE,
    name          VARCHAR(255) NOT NULL,
    doc_type_hint VARCHAR(255) NOT NULL DEFAULT '',
    scope         VARCHAR(10)  NOT NULL CHECK (scope IN ('page', 'document')),
    prompt        TEXT         NOT NULL,
    output_format VARCHAR(10)  NOT NULL
                  CHECK (output_format IN ('markdown', 'json', 'csv', 'text')),
    json_schema   TEXT,
    is_builtin    BOOLEAN      NOT NULL DEFAULT false
);

CREATE TABLE output (
    id          VARCHAR(36)  PRIMARY KEY,
    user_id     VARCHAR(36)  NOT NULL REFERENCES web_user(id) ON DELETE CASCADE,
    document_id VARCHAR(36)  NOT NULL REFERENCES document(id) ON DELETE CASCADE,
    template_id VARCHAR(36)  NOT NULL REFERENCES template(id),
    content     TEXT         NOT NULL,
    file_path   TEXT,
    model       VARCHAR(100) NOT NULL,
    status      VARCHAR(12)  NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_document_user ON document (user_id);
CREATE INDEX idx_page_document ON page (document_id);
CREATE INDEX idx_ocr_result_page ON ocr_result (page_id);
CREATE INDEX idx_output_document ON output (document_id);
CREATE INDEX idx_output_user ON output (user_id);

-- Built-in templates (fixed IDs match domain.TemplateIDCleanMarkdown / StructuredJSON).
INSERT INTO template (id, owner_user_id, name, doc_type_hint, scope, prompt, output_format, json_schema, is_builtin)
VALUES
  ('00000000-0000-0000-0000-000000000001', NULL, 'Clean Markdown document', 'general', 'document',
   'You are given the OCR transcription of every page of a scanned document, page-marked. Assemble it into a single clean, readable Markdown document. Fix obvious OCR artifacts, preserve headings and lists, and drop repeated running headers, footers, and standalone page numbers. Output only the Markdown.',
   'markdown', NULL, true),
  ('00000000-0000-0000-0000-000000000002', NULL, 'Structured fields to JSON', 'general', 'page',
   'Extract the key fields present on this page into a JSON object. Use lowercase snake_case keys. Omit fields that are not present. Output only JSON.',
   'json', '{"type":"object","additionalProperties":true}', true);

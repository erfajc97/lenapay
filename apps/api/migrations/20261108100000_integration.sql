-- Integration pass: verified identity on organizations (KYC approval copies it, the ERP link
-- looks it up), push tokens of any platform, org-requested reconciliation runs and payout
-- attachments.

-- +goose Up
ALTER TABLE organizations
    ADD COLUMN doc_type   text CHECK (doc_type IN ('cuit', 'cuil', 'dni')),
    ADD COLUMN doc_number text CHECK (doc_number ~ '^[0-9]{7,11}$'),
    ADD CONSTRAINT organizations_doc_pair_check CHECK ((doc_type IS NULL) = (doc_number IS NULL));
CREATE INDEX organizations_doc_idx ON organizations (doc_type, doc_number);

ALTER TABLE push_tokens DROP CONSTRAINT push_tokens_platform_check;
ALTER TABLE push_tokens
    ADD CONSTRAINT push_tokens_platform_check CHECK (platform IN ('ios', 'android', 'web', 'unknown'));

ALTER TABLE reconciliation_runs DROP CONSTRAINT reconciliation_runs_requested_by_check;
ALTER TABLE reconciliation_runs
    ADD CONSTRAINT reconciliation_runs_requested_by_check CHECK (requested_by IN ('schedule', 'internal', 'user'));
CREATE INDEX reconciliation_items_run_created_idx ON reconciliation_items (run_id, created_at, id);

CREATE TABLE payout_attachments (
    id           uuid        PRIMARY KEY,
    org_id       uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    payout_id    uuid        NOT NULL,
    file_name    text        NOT NULL CHECK (char_length(file_name) BETWEEN 1 AND 200),
    content_type text        NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png')),
    size_bytes   bigint      NOT NULL CHECK (size_bytes BETWEEN 1 AND 10485760),
    sha256       bytea       NOT NULL CHECK (octet_length(sha256) = 32),
    storage_key  text        NOT NULL UNIQUE CHECK (char_length(storage_key) BETWEEN 1 AND 300),
    uploaded_by  uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (payout_id, org_id) REFERENCES payouts (id, org_id) ON DELETE RESTRICT
);
CREATE INDEX payout_attachments_payout_idx ON payout_attachments (payout_id, org_id, created_at, id);
CREATE INDEX payout_attachments_org_id_idx ON payout_attachments (org_id);
CREATE INDEX payout_attachments_uploaded_by_idx ON payout_attachments (uploaded_by);

-- +goose Down
DROP TABLE payout_attachments;

DROP INDEX reconciliation_items_run_created_idx;
DELETE FROM reconciliation_runs WHERE requested_by = 'user';
ALTER TABLE reconciliation_runs DROP CONSTRAINT reconciliation_runs_requested_by_check;
ALTER TABLE reconciliation_runs
    ADD CONSTRAINT reconciliation_runs_requested_by_check CHECK (requested_by IN ('schedule', 'internal'));

DELETE FROM push_tokens WHERE platform NOT IN ('ios', 'android');
ALTER TABLE push_tokens DROP CONSTRAINT push_tokens_platform_check;
ALTER TABLE push_tokens ADD CONSTRAINT push_tokens_platform_check CHECK (platform IN ('ios', 'android'));

DROP INDEX organizations_doc_idx;
ALTER TABLE organizations
    DROP CONSTRAINT organizations_doc_pair_check,
    DROP COLUMN doc_number,
    DROP COLUMN doc_type;

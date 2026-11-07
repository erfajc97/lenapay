-- KYC onboarding cases (one per organization) and their documents.

-- +goose Up
CREATE TABLE kyc_cases (
    id              uuid        PRIMARY KEY,
    org_id          uuid        NOT NULL UNIQUE REFERENCES organizations (id) ON DELETE CASCADE,
    kind            text        NOT NULL CHECK (kind IN ('company', 'person')),
    status          text        NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft', 'submitted', 'approved', 'rejected')),
    data            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    provider_ref    text        CHECK (char_length(provider_ref) <= 120),
    decision_reason text        CHECK (char_length(decision_reason) <= 500),
    submitted_at    timestamptz,
    decided_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, org_id)
);
CREATE INDEX kyc_cases_status_idx ON kyc_cases (status);

CREATE TABLE kyc_documents (
    id                 uuid        PRIMARY KEY,
    org_id             uuid        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    case_id            uuid        NOT NULL,
    kind               text        NOT NULL
                                   CHECK (kind IN ('bylaws', 'afip_constancy', 'dni_front', 'dni_back', 'selfie', 'other')),
    representative_dni text        CHECK (representative_dni ~ '^[0-9]{7,8}$'),
    file_name          text        NOT NULL CHECK (char_length(file_name) BETWEEN 1 AND 200),
    content_type       text        NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png')),
    size_bytes         bigint      NOT NULL CHECK (size_bytes > 0),
    sha256             bytea       NOT NULL CHECK (octet_length(sha256) = 32),
    storage_key        text        NOT NULL UNIQUE,
    created_at         timestamptz NOT NULL DEFAULT now(),
    -- Same-tenant guard (T6): a document's case belongs to the same org.
    FOREIGN KEY (case_id, org_id) REFERENCES kyc_cases (id, org_id) ON DELETE CASCADE
);
CREATE INDEX kyc_documents_case_id_idx ON kyc_documents (case_id, org_id);
CREATE INDEX kyc_documents_org_id_idx ON kyc_documents (org_id);

-- +goose Down
DROP TABLE kyc_documents;
DROP TABLE kyc_cases;

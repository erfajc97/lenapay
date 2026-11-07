-- Reconciliation of the PSP master account, ERP identity index and ERP links.

-- +goose Up
CREATE TABLE reconciliation_runs (
    id                      uuid        PRIMARY KEY,
    currency                char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    day                     date        NOT NULL,
    status                  text        NOT NULL DEFAULT 'queued'
                                        CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    matched_count           integer     NOT NULL DEFAULT 0 CHECK (matched_count >= 0),
    missing_in_ledger_count integer     NOT NULL DEFAULT 0 CHECK (missing_in_ledger_count >= 0),
    missing_at_psp_count    integer     NOT NULL DEFAULT 0 CHECK (missing_at_psp_count >= 0),
    psp_total_minor         bigint      NOT NULL DEFAULT 0,
    ledger_total_minor      bigint      NOT NULL DEFAULT 0,
    difference_minor        bigint      NOT NULL DEFAULT 0,
    requested_by            text        NOT NULL CHECK (requested_by IN ('schedule', 'internal')),
    created_at              timestamptz NOT NULL DEFAULT now(),
    started_at              timestamptz,
    finished_at             timestamptz,
    CHECK (difference_minor = psp_total_minor - ledger_total_minor)
);
CREATE INDEX reconciliation_runs_created_idx ON reconciliation_runs (created_at DESC, id DESC);
CREATE INDEX reconciliation_runs_currency_day_idx ON reconciliation_runs (currency, day);

CREATE TABLE reconciliation_items (
    id                   uuid        PRIMARY KEY,
    run_id               uuid        NOT NULL REFERENCES reconciliation_runs (id) ON DELETE CASCADE,
    -- The org the movement belongs to, when the ledger knows it (null for missing_in_ledger).
    org_id               uuid        REFERENCES organizations (id) ON DELETE RESTRICT,
    provider_transfer_id text        NOT NULL CHECK (char_length(provider_transfer_id) BETWEEN 1 AND 100),
    direction            text        NOT NULL CHECK (direction IN ('in', 'out')),
    currency             char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    psp_amount_minor     bigint,
    ledger_amount_minor  bigint,
    status               text        NOT NULL CHECK (status IN ('matched', 'missing_in_ledger', 'missing_at_psp')),
    created_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, provider_transfer_id)
);
CREATE INDEX reconciliation_items_run_status_idx ON reconciliation_items (run_id, status);
CREATE INDEX reconciliation_items_org_id_idx ON reconciliation_items (org_id);

CREATE TABLE erp_identity_index (
    doc_type      text        NOT NULL CHECK (doc_type IN ('cuit', 'cuil', 'dni')),
    doc_number    text        NOT NULL CHECK (doc_number ~ '^[0-9]{7,11}$'),
    erp_org_id    text        NOT NULL CHECK (char_length(erp_org_id) BETWEEN 1 AND 120),
    erp_client_id text        CHECK (char_length(erp_client_id) <= 120),
    display_name  text        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 200),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (doc_type, doc_number)
);
CREATE INDEX erp_identity_index_erp_org_id_idx ON erp_identity_index (erp_org_id);

CREATE TABLE erp_links (
    id            uuid        PRIMARY KEY,
    org_id        uuid        NOT NULL UNIQUE REFERENCES organizations (id) ON DELETE RESTRICT,
    erp_org_id    text        NOT NULL UNIQUE CHECK (char_length(erp_org_id) BETWEEN 1 AND 120),
    display_name  text        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 200),
    linked_by     uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    linked_at     timestamptz NOT NULL DEFAULT now(),
    import_status text        NOT NULL DEFAULT 'pending'
                              CHECK (import_status IN ('pending', 'running', 'completed', 'failed')),
    imported_at   timestamptz,
    snapshot      jsonb       NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX erp_links_linked_by_idx ON erp_links (linked_by);

-- +goose Down
DROP TABLE erp_links;
DROP TABLE erp_identity_index;
DROP TABLE reconciliation_items;
DROP TABLE reconciliation_runs;

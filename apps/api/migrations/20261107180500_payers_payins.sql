-- Payers, CVUs, PSP webhook events and payins.

-- +goose Up
CREATE TABLE payers (
    id         uuid        PRIMARY KEY,
    org_id     uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    account_id uuid        NOT NULL,
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 2 AND 120),
    doc_type   text        NOT NULL CHECK (doc_type IN ('cuit', 'cuil', 'dni')),
    doc_number text        NOT NULL CHECK (doc_number ~ '^[0-9]{7,11}$'),
    email      text        CHECK (char_length(email) BETWEEN 3 AND 254),
    erp_ref    text        CHECK (char_length(erp_ref) <= 120),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- Same-tenant guard (T6).
    FOREIGN KEY (account_id, org_id) REFERENCES accounts (id, org_id) ON DELETE RESTRICT,
    UNIQUE (id, org_id)
);
CREATE INDEX payers_org_id_idx ON payers (org_id, created_at DESC, id DESC);
CREATE INDEX payers_account_id_idx ON payers (account_id, org_id);
CREATE INDEX payers_org_doc_idx ON payers (org_id, doc_number);
CREATE UNIQUE INDEX payers_erp_ref_key ON payers (org_id, erp_ref) WHERE erp_ref IS NOT NULL;

CREATE TABLE cvus (
    id          uuid        PRIMARY KEY,
    org_id      uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    account_id  uuid        NOT NULL,
    payer_id    uuid,
    currency    char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    cvu         char(22)    NOT NULL UNIQUE CHECK (cvu ~ '^[0-9]{22}$'),
    alias       text        CHECK (alias ~ '^[a-zA-Z0-9.-]{6,20}$'),
    psp_ref     text        NOT NULL UNIQUE CHECK (char_length(psp_ref) BETWEEN 1 AND 120),
    status      text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    disabled_at timestamptz,
    FOREIGN KEY (account_id, org_id) REFERENCES accounts (id, org_id) ON DELETE RESTRICT,
    FOREIGN KEY (payer_id, org_id) REFERENCES payers (id, org_id) ON DELETE RESTRICT,
    CHECK ((status = 'disabled') = (disabled_at IS NOT NULL))
);
CREATE UNIQUE INDEX cvus_alias_key ON cvus (lower(alias)) WHERE alias IS NOT NULL;
-- One active CVU per payer per currency (R-CVU-1) and per personal account per currency (R-CVU-3).
CREATE UNIQUE INDEX cvus_payer_currency_key ON cvus (payer_id, currency)
    WHERE payer_id IS NOT NULL AND status = 'active';
CREATE UNIQUE INDEX cvus_account_currency_key ON cvus (account_id, currency)
    WHERE payer_id IS NULL AND status = 'active';
CREATE INDEX cvus_org_id_idx ON cvus (org_id);
CREATE INDEX cvus_account_id_idx ON cvus (account_id, org_id);
CREATE INDEX cvus_payer_id_idx ON cvus (payer_id, org_id);

CREATE TABLE psp_webhook_events (
    id                uuid        PRIMARY KEY,
    provider_event_id text        NOT NULL UNIQUE CHECK (char_length(provider_event_id) BETWEEN 1 AND 100),
    event_type        text        NOT NULL CHECK (char_length(event_type) BETWEEN 1 AND 60),
    raw_body          bytea       NOT NULL,
    signature_ok      boolean     NOT NULL,
    received_at       timestamptz NOT NULL DEFAULT now(),
    processed_at      timestamptz,
    error_code        text        CHECK (char_length(error_code) <= 80)
);
CREATE INDEX psp_webhook_events_unprocessed_idx ON psp_webhook_events (received_at) WHERE processed_at IS NULL;

CREATE TABLE payins (
    id                    uuid        PRIMARY KEY,
    org_id                uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    account_id            uuid        NOT NULL,
    cvu_id                uuid        NOT NULL REFERENCES cvus (id) ON DELETE RESTRICT,
    payer_id              uuid,
    currency              char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    gross_minor           bigint      NOT NULL CHECK (gross_minor > 0),
    fee_minor             bigint      NOT NULL CHECK (fee_minor >= 0),
    net_minor             bigint      NOT NULL CHECK (net_minor >= 0),
    origin_name           text        CHECK (char_length(origin_name) <= 200),
    origin_cuit           text        CHECK (origin_cuit ~ '^[0-9]{11}$'),
    origin_cbu            text        CHECK (origin_cbu ~ '^[0-9]{22}$'),
    provider_transfer_id  text        NOT NULL UNIQUE CHECK (char_length(provider_transfer_id) BETWEEN 1 AND 100),
    status                text        NOT NULL DEFAULT 'received' CHECK (status IN ('received', 'credited', 'failed')),
    ledger_transaction_id uuid        REFERENCES ledger_transactions (id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now(),
    credited_at           timestamptz,
    FOREIGN KEY (account_id, org_id) REFERENCES accounts (id, org_id) ON DELETE RESTRICT,
    FOREIGN KEY (payer_id, org_id) REFERENCES payers (id, org_id) ON DELETE RESTRICT,
    CHECK (gross_minor = fee_minor + net_minor),
    CHECK ((status = 'credited') = (ledger_transaction_id IS NOT NULL AND credited_at IS NOT NULL))
);
CREATE INDEX payins_org_id_idx ON payins (org_id, created_at DESC, id DESC);
CREATE INDEX payins_account_id_idx ON payins (account_id, org_id);
CREATE INDEX payins_cvu_id_idx ON payins (cvu_id);
CREATE INDEX payins_payer_id_idx ON payins (payer_id, org_id);
CREATE INDEX payins_ledger_transaction_id_idx ON payins (ledger_transaction_id);
CREATE INDEX payins_currency_created_idx ON payins (currency, created_at);

-- +goose Down
DROP TABLE payins;
DROP TABLE psp_webhook_events;
DROP TABLE cvus;
DROP TABLE payers;

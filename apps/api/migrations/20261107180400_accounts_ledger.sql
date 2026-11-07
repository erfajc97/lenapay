-- Accounts and the double-entry ledger (P4: the ledger is the only writer of balances).
-- Invariants enforced by the database, not only by code:
--   * every ledger transaction sums to 0 per currency (deferred constraint trigger);
--   * postings and transactions are append-only;
--   * a posting's currency equals its ledger account's currency (composite FK);
--   * a non-system balance never goes below its holds (CHECK on ledger_balances).

-- +goose Up
CREATE TABLE accounts (
    id              uuid        PRIMARY KEY,
    org_id          uuid        REFERENCES organizations (id) ON DELETE RESTRICT,
    kind            text        NOT NULL CHECK (kind IN ('main', 'project', 'personal', 'system')),
    system_code     text        UNIQUE
                                CHECK (system_code IN ('psp_master', 'fees_revenue', 'payout_clearing', 'suspense')),
    name            text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    erp_project_ref text        CHECK (char_length(erp_project_ref) <= 120),
    archived_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    -- System accounts belong to Lenapay (no org); every other account belongs to one org.
    CHECK ((kind = 'system') = (org_id IS NULL)),
    CHECK ((kind = 'system') = (system_code IS NOT NULL)),
    UNIQUE (id, org_id)
);
CREATE INDEX accounts_org_id_idx ON accounts (org_id, created_at);
-- One main (company) or personal account per org.
CREATE UNIQUE INDEX accounts_single_main_key ON accounts (org_id) WHERE kind IN ('main', 'personal');
CREATE UNIQUE INDEX accounts_erp_project_ref_key ON accounts (org_id, erp_project_ref)
    WHERE erp_project_ref IS NOT NULL;

CREATE TABLE ledger_accounts (
    id         uuid        PRIMARY KEY,
    account_id uuid        NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    currency   char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, currency),
    UNIQUE (id, currency)
);

CREATE TABLE ledger_balances (
    ledger_account_id uuid        PRIMARY KEY REFERENCES ledger_accounts (id) ON DELETE RESTRICT,
    posted_minor      bigint      NOT NULL DEFAULT 0,
    held_minor        bigint      NOT NULL DEFAULT 0 CHECK (held_minor >= 0),
    -- Only system accounts (psp_master, ...) may go negative.
    allow_negative    boolean     NOT NULL DEFAULT false,
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CHECK (allow_negative OR posted_minor - held_minor >= 0)
);

CREATE TABLE ledger_transactions (
    id              uuid        PRIMARY KEY,
    kind            text        NOT NULL
                                CHECK (kind IN ('payin', 'payout', 'internal_transfer', 'reversal', 'adjustment')),
    ref_type        text        NOT NULL CHECK (char_length(ref_type) BETWEEN 1 AND 40),
    ref_id          uuid        NOT NULL,
    org_id          uuid        REFERENCES organizations (id) ON DELETE RESTRICT,
    idempotency_key text        CHECK (char_length(idempotency_key) <= 128),
    memo            text        CHECK (char_length(memo) <= 140),
    created_by      uuid        REFERENCES users (id) ON DELETE RESTRICT,
    created_at      timestamptz NOT NULL DEFAULT now(),
    -- The same business fact never posts twice (MO3).
    UNIQUE (ref_type, ref_id, kind)
);
CREATE INDEX ledger_transactions_org_id_idx ON ledger_transactions (org_id, created_at DESC);
CREATE INDEX ledger_transactions_created_by_idx ON ledger_transactions (created_by);
CREATE INDEX ledger_transactions_created_at_idx ON ledger_transactions (created_at);

CREATE TABLE ledger_postings (
    id                uuid        PRIMARY KEY,
    transaction_id    uuid        NOT NULL REFERENCES ledger_transactions (id) ON DELETE RESTRICT,
    ledger_account_id uuid        NOT NULL,
    currency          char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    amount_minor      bigint      NOT NULL CHECK (amount_minor <> 0),
    created_at        timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (ledger_account_id, currency) REFERENCES ledger_accounts (id, currency) ON DELETE RESTRICT
);
CREATE INDEX ledger_postings_transaction_id_idx ON ledger_postings (transaction_id);
CREATE INDEX ledger_postings_ledger_account_id_idx ON ledger_postings (ledger_account_id, created_at DESC);

CREATE TABLE holds (
    id                uuid        PRIMARY KEY,
    ledger_account_id uuid        NOT NULL,
    currency          char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    amount_minor      bigint      NOT NULL CHECK (amount_minor > 0),
    ref_type          text        NOT NULL CHECK (char_length(ref_type) BETWEEN 1 AND 40),
    ref_id            uuid        NOT NULL,
    status            text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'released', 'captured')),
    created_at        timestamptz NOT NULL DEFAULT now(),
    released_at       timestamptz,
    captured_at       timestamptz,
    FOREIGN KEY (ledger_account_id, currency) REFERENCES ledger_accounts (id, currency) ON DELETE RESTRICT,
    UNIQUE (ref_type, ref_id),
    CHECK ((status = 'released') = (released_at IS NOT NULL)),
    CHECK ((status = 'captured') = (captured_at IS NOT NULL))
);
CREATE INDEX holds_ledger_account_id_idx ON holds (ledger_account_id) WHERE status = 'active';

-- +goose StatementBegin
CREATE FUNCTION ledger_check_transaction_balanced() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    unbalanced char(3);
BEGIN
    SELECT p.currency INTO unbalanced
    FROM ledger_postings p
    WHERE p.transaction_id = NEW.transaction_id
    GROUP BY p.currency
    HAVING sum(p.amount_minor) <> 0
    LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'ledger transaction % does not sum to zero in %', NEW.transaction_id, unbalanced
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ledger_postings_balanced';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- Deferred: the check runs at COMMIT, after every leg of the transaction is inserted.
CREATE CONSTRAINT TRIGGER ledger_postings_balanced
    AFTER INSERT ON ledger_postings
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ledger_check_transaction_balanced();

-- +goose StatementBegin
CREATE FUNCTION reject_append_only_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ledger_postings_append_only
    BEFORE UPDATE OR DELETE ON ledger_postings
    FOR EACH ROW EXECUTE FUNCTION reject_append_only_mutation();
CREATE TRIGGER ledger_transactions_append_only
    BEFORE UPDATE OR DELETE ON ledger_transactions
    FOR EACH ROW EXECUTE FUNCTION reject_append_only_mutation();

-- +goose Down
DROP TRIGGER ledger_transactions_append_only ON ledger_transactions;
DROP TRIGGER ledger_postings_append_only ON ledger_postings;
DROP TRIGGER ledger_postings_balanced ON ledger_postings;
DROP FUNCTION ledger_check_transaction_balanced();
DROP TABLE holds;
DROP TABLE ledger_postings;
DROP TABLE ledger_transactions;
DROP TABLE ledger_balances;
DROP TABLE ledger_accounts;
DROP TABLE accounts;
-- audit_log (00008) also uses this function; goose rolls 00008 back before this file.
DROP FUNCTION reject_append_only_mutation();

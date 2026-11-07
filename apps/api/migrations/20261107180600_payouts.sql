-- Payouts and their status history. Fee and destination are recorded at request time (MO5).

-- +goose Up
CREATE TABLE payouts (
    id                       uuid        PRIMARY KEY,
    org_id                   uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    account_id               uuid        NOT NULL,
    currency                 char(3)     NOT NULL CHECK (currency IN ('ARS', 'USD')),
    amount_minor             bigint      NOT NULL CHECK (amount_minor > 0),
    fee_minor                bigint      NOT NULL CHECK (fee_minor >= 0),
    fee_plan_id              uuid        REFERENCES fee_plans (id) ON DELETE RESTRICT,
    destination_type         text        NOT NULL CHECK (destination_type IN ('cbu', 'cvu', 'alias')),
    destination_value        text        NOT NULL CHECK (char_length(destination_value) BETWEEN 6 AND 22),
    destination_account      char(22)    NOT NULL CHECK (destination_account ~ '^[0-9]{22}$'),
    destination_holder_name  text        NOT NULL CHECK (char_length(destination_holder_name) BETWEEN 1 AND 200),
    destination_holder_cuit  text        NOT NULL CHECK (destination_holder_cuit ~ '^[0-9]{11}$'),
    destination_bank_name    text        CHECK (char_length(destination_bank_name) <= 120),
    concept                  text        NOT NULL CHECK (char_length(concept) BETWEEN 1 AND 140),
    status                   text        NOT NULL CHECK (status IN
                                         ('pending_approval', 'approved', 'sending', 'settled', 'failed', 'rejected', 'cancelled')),
    created_by               uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    approved_by              uuid        REFERENCES users (id) ON DELETE RESTRICT,
    rejected_by              uuid        REFERENCES users (id) ON DELETE RESTRICT,
    reject_reason            text        CHECK (char_length(reject_reason) <= 280),
    hold_id                  uuid        REFERENCES holds (id) ON DELETE RESTRICT,
    attempt_count            integer     NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    provider_attempt_id      text        UNIQUE CHECK (char_length(provider_attempt_id) <= 100),
    provider_transfer_id     text        UNIQUE CHECK (char_length(provider_transfer_id) <= 100),
    failure_code             text        CHECK (char_length(failure_code) <= 80),
    ledger_transaction_id    uuid        REFERENCES ledger_transactions (id) ON DELETE RESTRICT,
    idempotency_key          text        NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 128),
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    approved_at              timestamptz,
    settled_at               timestamptz,
    failed_at                timestamptz,
    FOREIGN KEY (account_id, org_id) REFERENCES accounts (id, org_id) ON DELETE RESTRICT,
    UNIQUE (org_id, idempotency_key),
    UNIQUE (id, org_id),
    -- 4-eyes (R-OUT-4): the creator never approves their own payout.
    CHECK (approved_by IS NULL OR approved_by <> created_by)
);
CREATE INDEX payouts_org_status_idx ON payouts (org_id, status, created_at DESC, id DESC);
CREATE INDEX payouts_org_created_idx ON payouts (org_id, created_at DESC, id DESC);
CREATE INDEX payouts_account_id_idx ON payouts (account_id, org_id);
CREATE INDEX payouts_created_by_idx ON payouts (created_by);
CREATE INDEX payouts_approved_by_idx ON payouts (approved_by);
CREATE INDEX payouts_rejected_by_idx ON payouts (rejected_by);
CREATE INDEX payouts_fee_plan_id_idx ON payouts (fee_plan_id);
CREATE INDEX payouts_hold_id_idx ON payouts (hold_id);
CREATE INDEX payouts_ledger_transaction_id_idx ON payouts (ledger_transaction_id);
-- The sweep re-queries uncertain payouts.
CREATE INDEX payouts_sending_idx ON payouts (updated_at) WHERE status = 'sending';
CREATE INDEX payouts_settled_idx ON payouts (currency, settled_at) WHERE status = 'settled';

CREATE TABLE payout_events (
    id          uuid        PRIMARY KEY,
    payout_id   uuid        NOT NULL,
    org_id      uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    from_status text        CHECK (from_status IN
                            ('pending_approval', 'approved', 'sending', 'settled', 'failed', 'rejected', 'cancelled')),
    to_status   text        NOT NULL CHECK (to_status IN
                            ('pending_approval', 'approved', 'sending', 'settled', 'failed', 'rejected', 'cancelled')),
    actor_id    uuid        REFERENCES users (id) ON DELETE RESTRICT,
    reason      text        CHECK (char_length(reason) <= 280),
    created_at  timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (payout_id, org_id) REFERENCES payouts (id, org_id) ON DELETE RESTRICT
);
CREATE INDEX payout_events_payout_id_idx ON payout_events (payout_id, created_at);
CREATE INDEX payout_events_org_id_idx ON payout_events (org_id);
CREATE INDEX payout_events_actor_id_idx ON payout_events (actor_id);

-- +goose Down
DROP TABLE payout_events;
DROP TABLE payouts;

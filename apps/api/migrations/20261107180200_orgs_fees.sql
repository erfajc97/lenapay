-- Fee plans, organizations, memberships and invites.

-- +goose Up
CREATE TABLE fee_plans (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL UNIQUE CHECK (char_length(name) BETWEEN 1 AND 80),
    is_default boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- At most one default plan.
CREATE UNIQUE INDEX fee_plans_single_default_key ON fee_plans (is_default) WHERE is_default;

CREATE TABLE fee_plan_rates (
    fee_plan_id        uuid    NOT NULL REFERENCES fee_plans (id) ON DELETE CASCADE,
    currency           char(3) NOT NULL CHECK (currency IN ('ARS', 'USD')),
    payin_bps          integer NOT NULL CHECK (payin_bps BETWEEN 0 AND 10000),
    payout_fixed_minor bigint  NOT NULL CHECK (payout_fixed_minor >= 0),
    payout_bps         integer NOT NULL CHECK (payout_bps BETWEEN 0 AND 10000),
    PRIMARY KEY (fee_plan_id, currency)
);

CREATE TABLE organizations (
    id          uuid        PRIMARY KEY,
    kind        text        NOT NULL CHECK (kind IN ('company', 'personal')),
    legal_name  text        NOT NULL CHECK (char_length(legal_name) BETWEEN 2 AND 200),
    cuit        text        CHECK (cuit ~ '^[0-9]{11}$'),
    kyc_status  text        NOT NULL DEFAULT 'draft'
                            CHECK (kyc_status IN ('draft', 'submitted', 'approved', 'rejected')),
    fee_plan_id uuid        REFERENCES fee_plans (id) ON DELETE RESTRICT,
    created_by  uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
-- One approved organization per CUIT (drafts may compete until one is approved).
CREATE UNIQUE INDEX organizations_approved_cuit_key ON organizations (cuit)
    WHERE cuit IS NOT NULL AND kyc_status = 'approved';
CREATE INDEX organizations_cuit_idx ON organizations (cuit);
CREATE INDEX organizations_fee_plan_id_idx ON organizations (fee_plan_id);
CREATE INDEX organizations_created_by_idx ON organizations (created_by);

CREATE TABLE memberships (
    org_id     uuid        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text        NOT NULL CHECK (role IN ('viewer', 'operator', 'approver', 'admin')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX memberships_user_id_idx ON memberships (user_id);

CREATE TABLE invites (
    id          uuid        PRIMARY KEY,
    org_id      uuid        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email       text        NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
    role        text        NOT NULL CHECK (role IN ('viewer', 'operator', 'approver', 'admin')),
    token_hash  bytea       NOT NULL UNIQUE,
    invited_by  uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    expires_at  timestamptz NOT NULL,
    accepted_at timestamptz,
    accepted_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invites_org_id_idx ON invites (org_id, created_at DESC);
CREATE INDEX invites_invited_by_idx ON invites (invited_by);
CREATE INDEX invites_accepted_by_idx ON invites (accepted_by);
CREATE INDEX invites_email_lower_idx ON invites (lower(email));

-- +goose Down
DROP TABLE invites;
DROP TABLE memberships;
DROP TABLE organizations;
DROP TABLE fee_plan_rates;
DROP TABLE fee_plans;

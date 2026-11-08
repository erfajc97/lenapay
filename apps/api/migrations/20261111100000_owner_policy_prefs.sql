-- Owner decisions of 2026-11-07: ONE owner per organization above admin (its creator; existing
-- orgs: the earliest admin), a configurable payout approval policy per org, invites created by
-- the ERP users import (auto-accepted on email verification / login), per-user preferences
-- (default org and account, locale, theme) and an optional full name on users.

-- +goose Up
ALTER TABLE users
    ADD COLUMN full_name text CHECK (char_length(full_name) BETWEEN 1 AND 120);

ALTER TABLE memberships DROP CONSTRAINT memberships_role_check;
ALTER TABLE memberships
    ADD CONSTRAINT memberships_role_check CHECK (role IN ('viewer', 'operator', 'approver', 'admin', 'owner'));

-- Backfill: the earliest admin of each org becomes its owner.
UPDATE memberships m SET role = 'owner', updated_at = now()
FROM (
    SELECT DISTINCT ON (org_id) org_id, user_id
    FROM memberships
    WHERE role = 'admin'
    ORDER BY org_id, created_at, user_id
) first_admin
WHERE m.org_id = first_admin.org_id AND m.user_id = first_admin.user_id;

-- At most one owner per org...
CREATE UNIQUE INDEX memberships_one_owner_key ON memberships (org_id) WHERE role = 'owner';

-- ...and the owner never disappears: demoting or deleting the owner row must be compensated by
-- another owner in the same transaction (ownership transfer). Checked at commit; an org that
-- is being deleted (cascade) is exempt.
-- +goose StatementBegin
CREATE FUNCTION memberships_keep_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM organizations WHERE id = OLD.org_id)
       AND NOT EXISTS (SELECT 1 FROM memberships WHERE org_id = OLD.org_id AND role = 'owner') THEN
        RAISE EXCEPTION 'organization % must keep exactly one owner', OLD.org_id
            USING ERRCODE = '23514', CONSTRAINT = 'memberships_keep_owner';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER memberships_keep_owner
    AFTER UPDATE OF role OR DELETE ON memberships
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW WHEN (OLD.role = 'owner')
    EXECUTE FUNCTION memberships_keep_owner();

-- Invites created by the ERP users import are accepted automatically for their email.
ALTER TABLE invites
    ADD COLUMN source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'erp_import'));
CREATE INDEX invites_org_email_pending_idx ON invites (org_id, lower(email)) WHERE accepted_at IS NULL;

CREATE TABLE approval_policies (
    org_id                    uuid        PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
    mode                      text        NOT NULL CHECK (mode IN ('always', 'threshold')),
    threshold_ars_minor       bigint      CHECK (threshold_ars_minor BETWEEN 1 AND 9007199254740991),
    threshold_usd_minor       bigint      CHECK (threshold_usd_minor BETWEEN 1 AND 9007199254740991),
    allow_owner_self_approval boolean     NOT NULL,
    updated_by                uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    updated_at                timestamptz NOT NULL,
    CONSTRAINT approval_policies_threshold_check
        CHECK (mode = 'always' OR threshold_ars_minor IS NOT NULL OR threshold_usd_minor IS NOT NULL)
);
CREATE INDEX approval_policies_updated_by_idx ON approval_policies (updated_by);

-- 4-eyes stays a DB rule; the one exception is a payout the owner approved alone under the
-- policy (marked self_approved, audited as payout.self_approval).
ALTER TABLE payouts ADD COLUMN self_approved boolean NOT NULL DEFAULT false;
ALTER TABLE payouts DROP CONSTRAINT payouts_check;
ALTER TABLE payouts
    ADD CONSTRAINT payouts_four_eyes_check
        CHECK (approved_by IS NULL OR approved_by <> created_by OR self_approved);

CREATE TABLE user_preferences (
    user_id            uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    default_org_id     uuid        REFERENCES organizations (id) ON DELETE CASCADE,
    default_account_id uuid        REFERENCES accounts (id) ON DELETE SET NULL,
    locale             text        CHECK (locale IN ('es-AR', 'en-US')),
    theme              text        CHECK (theme IN ('system', 'light', 'dark')),
    updated_at         timestamptz NOT NULL,
    CONSTRAINT user_preferences_account_needs_org_check
        CHECK (default_account_id IS NULL OR default_org_id IS NOT NULL)
);
CREATE INDEX user_preferences_default_org_id_idx ON user_preferences (default_org_id);
CREATE INDEX user_preferences_default_account_id_idx ON user_preferences (default_account_id);

-- +goose Down
DROP TABLE user_preferences;

UPDATE payouts SET approved_by = NULL WHERE self_approved;
ALTER TABLE payouts DROP CONSTRAINT payouts_four_eyes_check;
ALTER TABLE payouts ADD CONSTRAINT payouts_check CHECK (approved_by IS NULL OR approved_by <> created_by);
ALTER TABLE payouts DROP COLUMN self_approved;

DROP TABLE approval_policies;

DROP INDEX invites_org_email_pending_idx;
ALTER TABLE invites DROP COLUMN source;

DROP TRIGGER memberships_keep_owner ON memberships;
DROP FUNCTION memberships_keep_owner();
DROP INDEX memberships_one_owner_key;
UPDATE memberships SET role = 'admin' WHERE role = 'owner';
ALTER TABLE memberships DROP CONSTRAINT memberships_role_check;
ALTER TABLE memberships
    ADD CONSTRAINT memberships_role_check CHECK (role IN ('viewer', 'operator', 'approver', 'admin'));

ALTER TABLE users DROP COLUMN full_name;

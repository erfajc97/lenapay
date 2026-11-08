-- On-us payments (P5): a payout to a Lenapay CVU never reaches the bank. It settles as ONE
-- ledger entry between the two accounts (rail 'internal') and the receiver gets a payin row
-- of the same rail linked to the payout. The fee plan gets its own on-us rates (default 0).
-- Holders of personal CVUs are identified by DNI/CUIL, so a payout destination doc is 7-11
-- digits.

-- +goose Up
ALTER TABLE payouts
    ADD COLUMN rail text NOT NULL DEFAULT 'bank' CHECK (rail IN ('bank', 'internal'));
ALTER TABLE payouts DROP CONSTRAINT payouts_destination_holder_cuit_check;
ALTER TABLE payouts
    ADD CONSTRAINT payouts_destination_holder_cuit_check CHECK (destination_holder_cuit ~ '^[0-9]{7,11}$');

ALTER TABLE payins
    ADD COLUMN rail      text NOT NULL DEFAULT 'bank' CHECK (rail IN ('bank', 'internal')),
    ADD COLUMN payout_id uuid REFERENCES payouts (id) ON DELETE RESTRICT,
    ADD CONSTRAINT payins_rail_payout_check CHECK ((rail = 'internal') = (payout_id IS NOT NULL));
-- One receiving payin per on-us payout.
CREATE UNIQUE INDEX payins_payout_id_key ON payins (payout_id) WHERE payout_id IS NOT NULL;

ALTER TABLE fee_plan_rates
    ADD COLUMN internal_fixed_minor bigint  NOT NULL DEFAULT 0 CHECK (internal_fixed_minor >= 0),
    ADD COLUMN internal_bps         integer NOT NULL DEFAULT 0 CHECK (internal_bps BETWEEN 0 AND 10000);

-- +goose Down
ALTER TABLE fee_plan_rates DROP COLUMN internal_bps, DROP COLUMN internal_fixed_minor;

DROP INDEX payins_payout_id_key;
DELETE FROM payins WHERE rail = 'internal';
ALTER TABLE payins DROP CONSTRAINT payins_rail_payout_check, DROP COLUMN payout_id, DROP COLUMN rail;

DELETE FROM payout_attachments WHERE payout_id IN (SELECT id FROM payouts WHERE rail = 'internal');
DELETE FROM payout_events WHERE payout_id IN (SELECT id FROM payouts WHERE rail = 'internal');
DELETE FROM payouts WHERE rail = 'internal';
ALTER TABLE payouts DROP CONSTRAINT payouts_destination_holder_cuit_check;
ALTER TABLE payouts
    ADD CONSTRAINT payouts_destination_holder_cuit_check CHECK (destination_holder_cuit ~ '^[0-9]{11}$');
ALTER TABLE payouts DROP COLUMN rail;

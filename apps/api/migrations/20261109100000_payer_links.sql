-- "Mis pagos" (payer view, B2C bridge): links between a person's personal org (its KYC-verified
-- DNI) and the payer records companies created for that document, so the person sees the
-- companies they pay to. Owned by the payerview feature; the tenant of a link is the personal org.

-- +goose Up
CREATE TABLE payer_links (
    id               uuid        PRIMARY KEY,
    personal_org_id  uuid        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    payer_id         uuid        NOT NULL,
    -- The company that owns the payer (denormalized from payers, kept equal by the FK below).
    org_id           uuid        NOT NULL REFERENCES organizations (id) ON DELETE RESTRICT,
    -- Which payer document matched the verified DNI: the DNI itself, or a CUIT/CUIL around it.
    matched_doc_type text        NOT NULL CHECK (matched_doc_type IN ('dni', 'cuit', 'cuil')),
    status           text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'hidden')),
    matched_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    FOREIGN KEY (payer_id, org_id) REFERENCES payers (id, org_id) ON DELETE CASCADE,
    UNIQUE (personal_org_id, payer_id),
    CHECK (personal_org_id <> org_id)
);
CREATE INDEX payer_links_personal_idx ON payer_links (personal_org_id, matched_at DESC, id DESC);
CREATE INDEX payer_links_payer_idx ON payer_links (payer_id, org_id);
CREATE INDEX payer_links_org_id_idx ON payer_links (org_id);

-- The matching key of a payer across every org: the 8-digit DNI a payer document carries (a DNI
-- left-padded, or the middle 8 digits of an 11-digit CUIT/CUIL). The matching query uses this
-- exact expression.
CREATE INDEX payers_person_dni_idx ON payers ((
    CASE
        WHEN doc_type = 'dni' AND char_length(doc_number) <= 8 THEN lpad(doc_number, 8, '0')
        WHEN doc_type IN ('cuit', 'cuil') AND char_length(doc_number) = 11 THEN substr(doc_number, 3, 8)
    END
));

-- +goose Down
DROP INDEX payers_person_dni_idx;
DROP TABLE payer_links;

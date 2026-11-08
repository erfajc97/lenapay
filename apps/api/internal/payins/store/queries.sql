-- PSP webhook event and payin queries. Provider ids are UNIQUE: duplicates are dropped by the DB.

-- name: InsertWebhookEvent :one
INSERT INTO psp_webhook_events (id, provider_event_id, event_type, raw_body, signature_ok)
VALUES (sqlc.arg(id), sqlc.arg(provider_event_id), sqlc.arg(event_type), sqlc.arg(raw_body), sqlc.arg(signature_ok))
ON CONFLICT (provider_event_id) DO NOTHING
RETURNING id;

-- name: MarkWebhookEventProcessed :exec
UPDATE psp_webhook_events SET processed_at = sqlc.arg(processed_at), error_code = sqlc.narg(error_code)
WHERE id = sqlc.arg(id);

-- name: InsertPayin :exec
INSERT INTO payins (id, org_id, account_id, cvu_id, payer_id, currency, gross_minor, fee_minor, net_minor,
                    origin_name, origin_cuit, origin_cbu, provider_transfer_id, status, ledger_transaction_id, credited_at,
                    rail, payout_id)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(account_id), sqlc.arg(cvu_id), sqlc.narg(payer_id), sqlc.arg(currency),
        sqlc.arg(gross_minor), sqlc.arg(fee_minor), sqlc.arg(net_minor), sqlc.narg(origin_name), sqlc.narg(origin_cuit),
        sqlc.narg(origin_cbu), sqlc.arg(provider_transfer_id), sqlc.arg(status), sqlc.narg(ledger_transaction_id),
        sqlc.narg(credited_at), sqlc.arg(rail), sqlc.narg(payout_id));

-- name: GetPayin :one
SELECT * FROM payins WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: GetPayinByProviderTransferID :one
SELECT * FROM payins WHERE provider_transfer_id = sqlc.arg(provider_transfer_id);

-- name: ListCreditedPayinsBetween :many
-- Bank-rail payins only: on-us credits never touch the master account (reconciliation).
SELECT * FROM payins
WHERE currency = sqlc.arg(currency) AND status = 'credited' AND rail = 'bank'
  AND credited_at >= sqlc.arg(from_time) AND credited_at < sqlc.arg(to_time)
ORDER BY credited_at, id;

// Package payins credits incoming transfers (lane C, R-IN-1…3, AG16, MO4). The PSP webhook is
// hand-mounted (handler_webhook.go): it verifies the HMAC over the raw body with hmac.Equal and
// a 5-minute timestamp tolerance, stores the raw event (unique provider event id) and enqueues
// the processing job in the SAME transaction. The job re-reads the transfer at the PSP by its
// id (never trusting the webhook body), finds the CVU, computes the fee and posts
// psp_master −gross → account +net, fees_revenue +fee, then notifies (SSE + push outbox).
// It owns psp_webhook_events and payins.
package payins

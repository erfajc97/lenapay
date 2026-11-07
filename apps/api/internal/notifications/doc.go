// Package notifications delivers user notifications (lane C, R-NOT-1, R-ID-1): an outbox
// written in the transaction of the fact (payin credited, approval needed, payout result),
// drained by a job through the Sender port (Expo push / SES later; provider/fake logs, and
// with DEV_INBOX_ENABLED also logs emailed codes). It also registers Expo push tokens. It owns
// notifications and push_tokens. It implements identity.CodeSender and orgs.InviteMailer.
package notifications

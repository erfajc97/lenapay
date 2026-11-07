// Package accounts owns org accounts (lane B, R-ACC-1/3/4): the main or personal account
// opened on KYC approval, project accounts (create, rename, archive with zero balance),
// balances per currency and immediate internal transfers between own accounts (posted through
// the ledger with an idempotency key and step-up). It owns the accounts table rows of kind
// main/project/personal; system accounts belong to ledger.
package accounts

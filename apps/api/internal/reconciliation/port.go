package reconciliation

import (
	"context"
	"time"

	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/payins"
	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/money"
)

// The narrow slices of other features' services that reconciliation reads (P5). The full
// services satisfy them; tests pass small fakes.

// Payins lists credited payins (implemented by payins.Service).
type Payins interface {
	CreditedBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]payins.Payin, error)
}

// Payouts lists settled payouts (implemented by payouts.Service).
type Payouts interface {
	SettledBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]payouts.Payout, error)
}

// CVUs finds the owner of a receiving CVU (implemented by payers.Service).
type CVUs interface {
	CVUByNumber(ctx context.Context, number string) (payers.CVU, error)
}

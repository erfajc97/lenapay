package fees

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/fees/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Service is the fee calculator used by payins and payouts (no HTTP operations). Callers post
// the fee to the fees_revenue system account in the SAME ledger entry as the money it is
// charged on (R-FEE-1) and store the Quote with their row (MO5).
type Service interface {
	// Quote returns the fee of op on amount for orgID's plan in amount's currency.
	// money.invalid_amount when amount is not positive; money.currency_mismatch for an
	// unsupported currency.
	Quote(ctx context.Context, orgID uuid.UUID, op Operation, amount money.Amount) (Quote, error)
	// EnsureDefaultPlan creates the default plan and fills its missing currencies with
	// DefaultRates (idempotent; never overwrites a rate; cmd/api, cmd/seed and tests).
	EnsureDefaultPlan(ctx context.Context) error
}

// Deps are the collaborators of the fees service.
type Deps struct {
	Pool *pgxpool.Pool
}

type service struct {
	pool *pgxpool.Pool
}

// NewService returns the fees service.
func NewService(deps Deps) Service { return &service{pool: deps.Pool} }

const defaultPlanName = "default"

func (s *service) Quote(ctx context.Context, orgID uuid.UUID, op Operation, amount money.Amount) (Quote, error) {
	if !amount.IsPositive() {
		return Quote{}, apperr.New(apperr.CodeInvalidAmount, nil)
	}
	if _, err := money.ParseCurrency(string(amount.Currency)); err != nil {
		return Quote{}, apperr.Wrap(apperr.CodeCurrencyMismatch, err, nil)
	}
	rate, err := store.New(s.pool).GetEffectiveFeeRate(ctx, store.GetEffectiveFeeRateParams{
		Currency: string(amount.Currency), OrgID: orgID,
	})
	if err != nil {
		// No rows: the org's plan (or the default) lacks this currency; a configuration bug.
		return Quote{}, fmt.Errorf("fees: rate for %s: %w", amount.Currency, err)
	}
	rates := Rates{PayinBps: int64(rate.PayinBps), PayoutFixedMinor: rate.PayoutFixedMinor, PayoutBps: int64(rate.PayoutBps)}
	fee, err := Compute(op, amount, rates)
	if err != nil {
		return Quote{}, err
	}
	q := Quote{Fee: fee, PlanID: rate.FeePlanID, Bps: rates.PayinBps}
	if op == OperationPayout {
		q.Bps, q.FixedMinor = rates.PayoutBps, rates.PayoutFixedMinor
	}
	return q, nil
}

// Compute is THE fee formula (G12): payin = gross × payin_bps; payout = fixed + amount ×
// payout_bps; percentages rounded with Rounding.
func Compute(op Operation, amount money.Amount, r Rates) (money.Amount, error) {
	switch op {
	case OperationPayin:
		return money.ApplyBps(amount, r.PayinBps, Rounding)
	case OperationPayout:
		pct, err := money.ApplyBps(amount, r.PayoutBps, Rounding)
		if err != nil {
			return money.Amount{}, err
		}
		return money.New(r.PayoutFixedMinor, amount.Currency).Add(pct)
	}
	return money.Amount{}, fmt.Errorf("fees: unknown operation %q", op)
}

func (s *service) EnsureDefaultPlan(ctx context.Context) error {
	return db.WithTx(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := q.InsertDefaultFeePlan(ctx, store.InsertDefaultFeePlanParams{ID: ids.New(), Name: defaultPlanName}); err != nil {
			return fmt.Errorf("fees: insert default plan: %w", err)
		}
		plan, err := q.GetDefaultFeePlan(ctx)
		if err != nil {
			return fmt.Errorf("fees: read default plan: %w", err)
		}
		for currency, r := range DefaultRates() {
			if err := q.InsertFeePlanRateIfMissing(ctx, store.InsertFeePlanRateIfMissingParams{
				FeePlanID: plan.ID, Currency: string(currency),
				PayinBps: int32(r.PayinBps), PayoutFixedMinor: r.PayoutFixedMinor, PayoutBps: int32(r.PayoutBps), //nolint:gosec // DefaultRates are constants ≤ 10 000
			}); err != nil {
				return fmt.Errorf("fees: default %s rate: %w", currency, err)
			}
		}
		return nil
	})
}

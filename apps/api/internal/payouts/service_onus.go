package payouts

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payins"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// failureDestinationUnavailable: the on-us destination CVU stopped receiving (or changed
// currency) between creation and settlement; the hold is released.
const failureDestinationUnavailable = "destination_unavailable"

// cuitDigits is the length of a CUIT/CUIL: the only origin document a payin row stores.
const cuitDigits = 11

// release moves an approved payout on: a bank payout is queued for the PSP (outbox, AG13); an
// on-us payout settles right here, in the approving transaction (no PSP call, P5).
func (s *service) release(ctx context.Context, tx pgx.Tx, payoutID uuid.UUID, rail Rail) error {
	if rail != RailInternal {
		return s.enqueueDispatch(ctx, tx, payoutID)
	}
	q := store.New(tx)
	row, err := q.GetPayoutForUpdate(ctx, payoutID)
	if err != nil {
		return fmt.Errorf("payouts: lock on-us %s: %w", payoutID, err)
	}
	return s.settleInternal(ctx, tx, q, row)
}

// settleInternal settles an approved on-us payout as ONE ledger entry, capturing its hold:
// paying account −(amount+fee), receiving account +amount, fees_revenue +fee. psp_master is
// never touched (reconciliation ignores the internal rail). The receiver gets a credited payin
// of rail internal in the same transaction. Any other status is a no-op (at-least-once jobs).
func (s *service) settleInternal(ctx context.Context, tx pgx.Tx, q *store.Queries, row store.Payout) error {
	if Status(row.Status) != StatusApproved {
		return nil
	}
	if row.HoldID == nil {
		return apperr.New(apperr.CodeHoldNotActive, map[string]any{"payoutId": row.ID.String()})
	}
	po := fromRow(row, nil)
	dest, err := s.deps.CVUs.CVUByNumber(ctx, row.DestinationAccount)
	if err != nil && !apperr.IsCode(err, apperr.CodeCVUNotFound) {
		return err
	}
	if err != nil || !dest.Active || dest.Currency != po.Amount.Currency {
		return s.failInternal(ctx, tx, q, row)
	}
	entry, err := s.internalEntry(ctx, po, dest, row.IdempotencyKey)
	if err != nil {
		return err
	}
	txID, err := s.deps.Ledger.CaptureHold(ctx, tx, *row.HoldID, entry)
	if err != nil {
		return err
	}
	now := s.deps.Clock.Now()
	rows, err := q.SettleInternalPayout(ctx, store.SettleInternalPayoutParams{LedgerTransactionID: &txID, SettledAt: &now, ID: row.ID})
	if err != nil {
		return fmt.Errorf("payouts: settle on-us: %w", err)
	}
	if rows != 1 {
		return invalidState(row)
	}
	credit, err := s.receivingSide(ctx, po, dest, txID)
	if err != nil {
		return err
	}
	if err := s.deps.Credits.CreditInternal(ctx, tx, credit); err != nil {
		return err
	}
	if err := publishBalance(ctx, tx, dest.OrgID, dest.AccountID); err != nil {
		return err
	}
	return s.finish(ctx, tx, q, po, StatusApproved, StatusSettled, nil)
}

// internalEntry is THE ledger entry of a settled on-us payout (a zero fee has no leg).
func (s *service) internalEntry(ctx context.Context, po Payout, dest payers.CVU, idemKey string) (ledger.Entry, error) {
	total, err := po.Total()
	if err != nil {
		return ledger.Entry{}, apperr.Wrap(apperr.CodeInvalidAmount, err, nil)
	}
	legs := []ledger.Leg{
		{AccountID: po.AccountID, Amount: money.New(-total.Minor, total.Currency)},
		{AccountID: dest.AccountID, Amount: po.Amount},
	}
	if po.Fee.IsPositive() {
		revenue, err := s.deps.Ledger.SystemAccountID(ctx, ledger.SystemFeesRevenue)
		if err != nil {
			return ledger.Entry{}, err
		}
		legs = append(legs, ledger.Leg{AccountID: revenue, Amount: po.Fee})
	}
	orgID := po.OrgID
	return ledger.Entry{
		Kind: ledger.KindPayout, RefType: refType, RefID: po.ID, OrgID: &orgID, IdempotencyKey: idemKey,
		CreatedBy: &po.CreatedBy, Memo: "on-us payout " + po.ID.String(), Legs: legs,
	}, nil
}

// receivingSide is the payin the receiver sees: the paying org's holder name, its CUIT (a DNI
// is not stored as origin) and its own CVU in the currency when it has one.
func (s *service) receivingSide(ctx context.Context, po Payout, dest payers.CVU, txID uuid.UUID) (payins.InternalCredit, error) {
	name, doc, err := s.deps.CVUs.Holder(ctx, po.OrgID)
	if err != nil {
		return payins.InternalCredit{}, err
	}
	origin, err := s.originCVU(ctx, po.OrgID, po.AccountID, po.Amount.Currency)
	if err != nil {
		return payins.InternalCredit{}, err
	}
	if len(doc) != cuitDigits {
		doc = ""
	}
	return payins.InternalCredit{
		PayoutID: po.ID, LedgerTransactionID: txID, CVU: dest, Amount: po.Amount,
		OriginName: name, OriginCUIT: doc, OriginCBU: origin,
	}, nil
}

// failInternal releases the hold of an on-us payout whose destination cannot receive.
func (s *service) failInternal(ctx context.Context, tx pgx.Tx, q *store.Queries, row store.Payout) error {
	if err := s.releaseHold(ctx, tx, row); err != nil {
		return err
	}
	code := failureDestinationUnavailable
	now := s.deps.Clock.Now()
	rows, err := q.FailInternalPayout(ctx, store.FailInternalPayoutParams{FailureCode: &code, FailedAt: &now, ID: row.ID})
	if err != nil {
		return fmt.Errorf("payouts: fail on-us: %w", err)
	}
	if rows != 1 {
		return invalidState(row)
	}
	return s.finish(ctx, tx, q, fromRow(row, nil), StatusApproved, StatusFailed, &code)
}

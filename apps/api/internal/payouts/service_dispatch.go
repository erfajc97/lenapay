package payouts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	applog "github.com/erfajc97/lenapay/apps/api/internal/platform/log"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Sweep tuning (R-OUT-5).
const (
	// sweepMinAge leaves a fresh attempt to its own dispatch job before the sweep asks again.
	sweepMinAge = time.Minute
	// sweepBatch bounds one sweep run; the next run takes the rest.
	sweepBatch = 100
	// notReceivedAfter: a reference the PSP still does not know this long after the attempt
	// started was never received, so the hold can be released (a definite answer, not a
	// timeout: MO4 never fails a payout on an uncertain one).
	notReceivedAfter = 15 * time.Minute
)

// Failure codes stored on failed payouts.
const (
	failureRejected    = "psp.rejected"
	failureNotReceived = "psp.not_received"
)

// attempt is the PSP attempt a dispatch works on.
type attempt struct {
	payoutID  uuid.UUID
	reference string // ONE per attempt (MO5): payouts.provider_attempt_id
	startedAt time.Time
	send      bool // false: the attempt already reached the PSP (or may have): re-read it
	request   psp.SendTransferRequest
}

func (s *service) Dispatch(ctx context.Context, payoutID uuid.UUID) error {
	a, ok, err := s.startAttempt(ctx, payoutID)
	if err != nil || !ok {
		return err
	}
	if !a.send {
		res, err := s.deps.PSP.TransferByReference(ctx, a.reference)
		return s.applyOutcome(ctx, a, res, err)
	}
	res, err := s.deps.PSP.SendTransfer(ctx, a.request)
	return s.applyOutcome(ctx, a, res, err)
}

// startAttempt records the attempt reference BEFORE the network call (its own transaction),
// so a crash after sending can never lose the reference the sweep re-reads by. A retried job
// finds the payout already `sending` and only re-reads it: the money is never sent twice.
func (s *service) startAttempt(ctx context.Context, payoutID uuid.UUID) (attempt, bool, error) {
	var a attempt
	var ok bool
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := q.GetPayoutForUpdate(ctx, payoutID)
		if db.IsNoRows(err) {
			return nil // nothing to send: a deleted test row or a wrong id; never retry it
		}
		if err != nil {
			return fmt.Errorf("payouts: dispatch lock: %w", err)
		}
		switch Status(row.Status) {
		case StatusSending:
			if row.ProviderAttemptID == nil {
				return fmt.Errorf("payouts: payout %s is sending without a reference", payoutID)
			}
			a, ok = attempt{payoutID: payoutID, reference: *row.ProviderAttemptID, startedAt: row.UpdatedAt}, true
			return nil
		case StatusApproved:
		default:
			return nil // settled, failed, rejected, cancelled or still pending: nothing to do
		}
		now := s.deps.Clock.Now()
		ref := attemptReference(payoutID, row.AttemptCount+1)
		rows, err := q.StartPayoutAttempt(ctx, store.StartPayoutAttemptParams{ProviderAttemptID: &ref, UpdatedAt: now, ID: payoutID})
		if err != nil {
			return fmt.Errorf("payouts: start attempt: %w", err)
		}
		if rows != 1 {
			return invalidState(row)
		}
		from := StatusApproved
		if err := s.event(ctx, q, payoutID, row.OrgID, &from, StatusSending, nil, nil); err != nil {
			return err
		}
		if err := publish(ctx, tx, row.OrgID, eventPayoutStatusChanged, payoutID, StatusSending); err != nil {
			return err
		}
		c := money.Currency(row.Currency)
		a = attempt{payoutID: payoutID, reference: ref, startedAt: now, send: true, request: psp.SendTransferRequest{
			Reference: ref, AccountNumber: row.DestinationAccount, Amount: money.New(row.AmountMinor, c), Concept: row.Concept,
		}}
		ok = true
		return nil
	})
	return a, ok, err
}

// attemptReference is our id of one attempt (≤ 100 chars, DB CHECK).
func attemptReference(payoutID uuid.UUID, n int32) string {
	return fmt.Sprintf("po-%s-%d", payoutID, n)
}

func (s *service) Sweep(ctx context.Context) error {
	ids, err := store.New(s.deps.Pool).ListSendingPayouts(ctx, store.ListSendingPayoutsParams{
		UpdatedBefore: s.deps.Clock.Now().Add(-sweepMinAge), PageLimit: sweepBatch,
	})
	if err != nil {
		return fmt.Errorf("payouts: sweep list: %w", err)
	}
	var errs []error
	for _, id := range ids {
		// Same path as the dispatch job: a `sending` payout is only ever re-read by reference.
		if err := s.Dispatch(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("payout %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// applyOutcome turns the PSP answer into the next state. Settled and rejected are definite;
// pending and uncertain keep the payout `sending` with its hold (the sweep asks again).
func (s *service) applyOutcome(ctx context.Context, a attempt, res psp.TransferResult, err error) error {
	switch {
	case err == nil && res.Status == psp.TransferSettled:
		return s.settle(ctx, a, res.ProviderID)
	case err == nil && res.Status == psp.TransferRejected,
		a.send && (errors.Is(err, psp.ErrRejected) || errors.Is(err, psp.ErrNotFound)):
		return s.fail(ctx, a, failureRejected)
	case errors.Is(err, psp.ErrNotFound) && s.deps.Clock.Now().Sub(a.startedAt) >= notReceivedAfter:
		return s.fail(ctx, a, failureNotReceived)
	case err == nil, errors.Is(err, psp.ErrNotFound):
		return nil // pending at the PSP, or not visible yet: ask again later
	case errors.Is(err, psp.ErrUncertain):
		slog.WarnContext(ctx, "payout outcome uncertain; the sweep will re-read it", "payout_id", a.payoutID.String())
		return nil
	default:
		// Unknown is not "no" (A9): stay sending, and let River log and retry the re-read.
		return fmt.Errorf("payouts: psp outcome of %s: %w", a.payoutID, err)
	}
}

// settle captures the hold: account −(amount+fee), psp_master +amount, fees_revenue +fee, in
// ONE ledger entry with the payout row update (R-FEE-1, MO3).
func (s *service) settle(ctx context.Context, a attempt, providerID string) error {
	master, err := s.deps.Ledger.SystemAccountID(ctx, ledger.SystemPSPMaster)
	if err != nil {
		return err
	}
	revenue, err := s.deps.Ledger.SystemAccountID(ctx, ledger.SystemFeesRevenue)
	if err != nil {
		return err
	}
	return db.WithSerializable(ctx, s.deps.Pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, ok, err := lockAttempt(ctx, q, a)
		if err != nil || !ok {
			return err
		}
		po := fromRow(row, nil)
		entry, err := settlementEntry(po, master, revenue, row.IdempotencyKey)
		if err != nil {
			return err
		}
		txID, err := s.deps.Ledger.CaptureHold(ctx, tx, *row.HoldID, entry)
		if err != nil {
			return err
		}
		now := s.deps.Clock.Now()
		rows, err := q.SettlePayout(ctx, store.SettlePayoutParams{ProviderTransferID: &providerID, LedgerTransactionID: &txID, SettledAt: &now, ID: a.payoutID})
		if err != nil {
			return fmt.Errorf("payouts: settle: %w", err)
		}
		if rows != 1 {
			return invalidState(row)
		}
		return s.finish(ctx, tx, q, po, StatusSettled, nil)
	})
}

// settlementEntry is THE ledger entry of a settled payout. A zero fee has no leg (the ledger
// refuses zero legs).
func settlementEntry(po Payout, master, revenue uuid.UUID, idemKey string) (ledger.Entry, error) {
	total, err := po.Total()
	if err != nil {
		return ledger.Entry{}, apperr.Wrap(apperr.CodeInvalidAmount, err, nil)
	}
	legs := []ledger.Leg{
		{AccountID: po.AccountID, Amount: money.New(-total.Minor, total.Currency)},
		{AccountID: master, Amount: po.Amount},
	}
	if po.Fee.IsPositive() {
		legs = append(legs, ledger.Leg{AccountID: revenue, Amount: po.Fee})
	}
	orgID := po.OrgID
	return ledger.Entry{
		Kind: ledger.KindPayout, RefType: refType, RefID: po.ID, OrgID: &orgID, IdempotencyKey: idemKey,
		CreatedBy: &po.CreatedBy, Legs: legs,
	}, nil
}

// fail releases the hold: the PSP said the transfer did not (and will not) happen (MO5).
func (s *service) fail(ctx context.Context, a attempt, code string) error {
	return db.WithSerializable(ctx, s.deps.Pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, ok, err := lockAttempt(ctx, q, a)
		if err != nil || !ok {
			return err
		}
		if err := s.releaseHold(ctx, tx, row); err != nil {
			return err
		}
		now := s.deps.Clock.Now()
		rows, err := q.FailPayout(ctx, store.FailPayoutParams{FailureCode: &code, FailedAt: &now, ID: a.payoutID})
		if err != nil {
			return fmt.Errorf("payouts: fail: %w", err)
		}
		if rows != 1 {
			return invalidState(row)
		}
		return s.finish(ctx, tx, q, fromRow(row, nil), StatusFailed, &code)
	})
}

// lockAttempt locks the payout and confirms it is still `sending` on this attempt; otherwise
// another run already applied an outcome and this one is a no-op (River delivers at least once).
func lockAttempt(ctx context.Context, q *store.Queries, a attempt) (store.Payout, bool, error) {
	row, err := q.GetPayoutForUpdate(ctx, a.payoutID)
	if err != nil {
		return store.Payout{}, false, fmt.Errorf("payouts: lock %s: %w", a.payoutID, err)
	}
	if Status(row.Status) != StatusSending || row.ProviderAttemptID == nil || *row.ProviderAttemptID != a.reference {
		return store.Payout{}, false, nil
	}
	if row.HoldID == nil {
		return store.Payout{}, false, apperr.New(apperr.CodeHoldNotActive, map[string]any{"payoutId": a.payoutID.String()})
	}
	return row, true, nil
}

// finish writes the history, audit, creator notification and realtime events of a final state.
func (s *service) finish(ctx context.Context, tx pgx.Tx, q *store.Queries, po Payout, to Status, reason *string) error {
	from := StatusSending
	if err := s.event(ctx, q, po.ID, po.OrgID, &from, to, nil, reason); err != nil {
		return err
	}
	if err := s.audit(ctx, tx, po.OrgID, nil, "payout."+string(to), po.ID, ""); err != nil {
		return err
	}
	if err := s.notify(ctx, tx, po.OrgID, []uuid.UUID{po.CreatedBy}, "payout."+string(to), po.ID, to); err != nil {
		return err
	}
	if err := publishBalance(ctx, tx, po.OrgID, po.AccountID); err != nil {
		return err
	}
	return publish(ctx, tx, po.OrgID, eventPayoutStatusChanged, po.ID, to)
}

// enqueueDispatch queues the dispatch job in the transaction of the approval (outbox, AG13).
func (s *service) enqueueDispatch(ctx context.Context, tx pgx.Tx, payoutID uuid.UUID) error {
	if _, err := s.deps.Jobs.InsertTx(ctx, tx, DispatchArgs{PayoutID: payoutID, RequestID: applog.RequestID(ctx)}, nil); err != nil {
		return fmt.Errorf("payouts: enqueue dispatch: %w", err)
	}
	return nil
}

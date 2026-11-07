package ledger

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/ledger/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Service is the ledger API for other features (no HTTP operations of its own). Every method
// that takes a tx runs inside the caller's transaction, so the money effect, the business row
// and the idempotency record commit or roll back together. Callers that move money run that
// transaction through db.WithSerializable (MO3).
type Service interface {
	// EnsureSystemAccounts creates the system accounts and their ledger accounts in every
	// currency (idempotent; cmd/api and cmd/seed call it at boot).
	EnsureSystemAccounts(ctx context.Context) error
	// SystemAccountID returns the account id of a system account.
	SystemAccountID(ctx context.Context, code SystemCode) (uuid.UUID, error)
	// OpenAccount creates the ledger accounts (ARS and USD) of an org account inside tx
	// (idempotent). Org accounts never go below their holds.
	OpenAccount(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) error
	// Post writes a balanced entry inside tx and updates the cached balances under lock and
	// returns the ledger transaction id. Errors: ledger.insufficient_funds when a non-system
	// leg would overdraw its available balance, account.not_found for an account without a
	// ledger account, money.currency_mismatch (wrapping ErrUnbalanced) for a cross-currency
	// entry, ErrInvalidEntry / ErrUnbalanced for malformed entries, ErrDuplicateRef when the
	// same (RefType, RefID, Kind) already posted.
	Post(ctx context.Context, tx pgx.Tx, e Entry) (uuid.UUID, error)
	// Hold reserves funds inside tx and returns the hold id (ledger.insufficient_funds when
	// available is short; ErrDuplicateRef when (RefType, RefID) already has a hold).
	Hold(ctx context.Context, tx pgx.Tx, in HoldInput) (uuid.UUID, error)
	// ReleaseHold frees a hold only when it is known the effect did not happen (MO5).
	// ledger.hold_not_active when the hold is missing, released or captured.
	ReleaseHold(ctx context.Context, tx pgx.Tx, holdID uuid.UUID) error
	// CaptureHold consumes a hold and posts its entry in the same transaction. The entry must
	// debit exactly the held amount on the held account and currency (ErrCaptureMismatch).
	CaptureHold(ctx context.Context, tx pgx.Tx, holdID uuid.UUID, e Entry) (uuid.UUID, error)
	// Balances returns posted, held and available per account and currency.
	Balances(ctx context.Context, accountIDs []uuid.UUID) ([]Balance, error)
	// LockBalances reads the balances of accountIDs inside tx with their rows locked (FOR
	// UPDATE, global order), so a decision based on them (archive only when empty) holds until
	// commit.
	LockBalances(ctx context.Context, tx pgx.Tx, accountIDs []uuid.UUID) ([]Balance, error)
}

// Deps are the collaborators of the ledger service.
type Deps struct {
	Pool  *pgxpool.Pool
	Clock clock.Clock
}

type service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

// NewService returns the ledger service.
func NewService(deps Deps) Service { return &service{pool: deps.Pool, clock: deps.Clock} }

// Unique constraints named by Postgres for the ref columns (migration 00004).
const (
	transactionRefKey = "ledger_transactions_ref_type_ref_id_kind_key"
	holdRefKey        = "holds_ref_type_ref_id_key"
	maxRefTypeLen     = 40
)

// legKey identifies one ledger account: an account in one currency.
type legKey struct {
	account  uuid.UUID
	currency money.Currency
}

func (s *service) EnsureSystemAccounts(ctx context.Context) error {
	now := s.clock.Now()
	return db.WithTx(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		for _, code := range SystemCodes() {
			systemCode := string(code)
			id, err := q.UpsertSystemAccount(ctx, store.UpsertSystemAccountParams{
				ID: ids.New(), SystemCode: &systemCode, Name: systemCode, Now: now,
			})
			if err != nil {
				return fmt.Errorf("ledger: system account %s: %w", code, err)
			}
			// System accounts mirror Lenapay's own positions and may go negative.
			if err := openLedgerAccounts(ctx, q, id, true, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *service) SystemAccountID(ctx context.Context, code SystemCode) (uuid.UUID, error) {
	systemCode := string(code)
	id, err := store.New(s.pool).GetSystemAccountID(ctx, &systemCode)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ledger: system account %s (run EnsureSystemAccounts): %w", code, err)
	}
	return id, nil
}

func (s *service) OpenAccount(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) error {
	return openLedgerAccounts(ctx, store.New(tx), accountID, false, s.clock.Now())
}

func openLedgerAccounts(ctx context.Context, q *store.Queries, accountID uuid.UUID, allowNegative bool, now time.Time) error {
	for _, c := range Currencies() {
		ledgerID, err := q.EnsureLedgerAccount(ctx, store.EnsureLedgerAccountParams{
			ID: ids.New(), AccountID: accountID, Currency: string(c), CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("ledger: open %s account: %w", c, err)
		}
		if err := q.InsertLedgerBalance(ctx, store.InsertLedgerBalanceParams{
			LedgerAccountID: ledgerID, AllowNegative: allowNegative, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("ledger: open %s balance: %w", c, err)
		}
	}
	return nil
}

func (s *service) Post(ctx context.Context, tx pgx.Tx, e Entry) (uuid.UUID, error) {
	return s.write(ctx, store.New(tx), e, nil)
}

func (s *service) CaptureHold(ctx context.Context, tx pgx.Tx, holdID uuid.UUID, e Entry) (uuid.UUID, error) {
	q := store.New(tx)
	hold, err := activeHold(ctx, q, holdID)
	if err != nil {
		return uuid.Nil, err
	}
	txID, err := s.write(ctx, q, e, &hold)
	if err != nil {
		return uuid.Nil, err
	}
	now := s.clock.Now()
	rows, err := q.CaptureHold(ctx, store.CaptureHoldParams{CapturedAt: &now, ID: holdID})
	if err != nil {
		return uuid.Nil, fmt.Errorf("ledger: capture hold: %w", err)
	}
	if rows != 1 { // impossible while the hold row is locked; never ignore it silently
		return uuid.Nil, apperr.New(apperr.CodeHoldNotActive, map[string]any{"holdId": holdID.String()})
	}
	return txID, nil
}

// write posts e. With capture set, the held amount is released inside the same lock, and e
// must debit exactly that amount on the held ledger account.
func (s *service) write(ctx context.Context, q *store.Queries, e Entry, capture *store.Hold) (uuid.UUID, error) {
	deltas, err := validateEntry(e)
	if err != nil {
		return uuid.Nil, err
	}
	ledgerIDs, err := resolve(ctx, q, slices.Collect(maps.Keys(deltas)))
	if err != nil {
		return uuid.Nil, err
	}
	if capture != nil {
		key := legKey{currency: money.Currency(capture.Currency)}
		for k, id := range ledgerIDs {
			if id == capture.LedgerAccountID {
				key = k
			}
		}
		if delta, ok := deltas[key]; !ok || delta != -capture.AmountMinor {
			return uuid.Nil, ErrCaptureMismatch
		}
	}
	if _, err := q.FindTransactionByRef(ctx, store.FindTransactionByRefParams{
		RefType: e.RefType, RefID: e.RefID, Kind: string(e.Kind),
	}); err == nil {
		return uuid.Nil, ErrDuplicateRef
	} else if !db.IsNoRows(err) {
		return uuid.Nil, fmt.Errorf("ledger: find transaction: %w", err)
	}

	locked, err := lock(ctx, q, slices.Collect(maps.Values(ledgerIDs)))
	if err != nil {
		return uuid.Nil, err
	}
	now := s.clock.Now()
	if capture != nil {
		if err := releaseHeld(ctx, q, capture.LedgerAccountID, capture.AmountMinor, now); err != nil {
			return uuid.Nil, err
		}
		b := locked[capture.LedgerAccountID]
		b.HeldMinor -= capture.AmountMinor
		locked[capture.LedgerAccountID] = b
	}
	for key, delta := range deltas {
		if err := checkAvailable(locked[ledgerIDs[key]], key, delta); err != nil {
			return uuid.Nil, err
		}
	}

	txID := ids.New()
	if err := q.InsertLedgerTransaction(ctx, store.InsertLedgerTransactionParams{
		ID: txID, Kind: string(e.Kind), RefType: e.RefType, RefID: e.RefID, OrgID: e.OrgID,
		IdempotencyKey: optional(e.IdempotencyKey), Memo: optional(e.Memo), CreatedBy: e.CreatedBy, CreatedAt: now,
	}); err != nil {
		if db.IsUniqueViolation(err, transactionRefKey) {
			return uuid.Nil, ErrDuplicateRef
		}
		return uuid.Nil, fmt.Errorf("ledger: insert transaction: %w", err)
	}
	for _, leg := range e.Legs {
		if err := q.InsertLedgerPosting(ctx, store.InsertLedgerPostingParams{
			ID: ids.New(), TransactionID: txID, LedgerAccountID: ledgerIDs[legKey{leg.AccountID, leg.Amount.Currency}],
			Currency: string(leg.Amount.Currency), AmountMinor: leg.Amount.Minor, CreatedAt: now,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("ledger: insert posting: %w", err)
		}
	}
	for _, key := range sortedKeys(deltas, ledgerIDs) {
		rows, err := q.ApplyPosting(ctx, store.ApplyPostingParams{
			AmountMinor: deltas[key], UpdatedAt: now, LedgerAccountID: ledgerIDs[key],
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("ledger: apply posting: %w", err)
		}
		if rows != 1 {
			return uuid.Nil, insufficient(key.account)
		}
	}
	return txID, nil
}

// validateEntry checks the shape of e and returns the net amount per (account, currency).
func validateEntry(e Entry) (map[legKey]int64, error) {
	if len(e.Legs) < 2 || !e.Kind.valid() || e.RefID == uuid.Nil || e.RefType == "" || len(e.RefType) > maxRefTypeLen {
		return nil, ErrInvalidEntry
	}
	deltas := map[legKey]int64{}
	sums := map[money.Currency]money.Amount{}
	for _, leg := range e.Legs {
		if _, err := money.ParseCurrency(string(leg.Amount.Currency)); err != nil {
			return nil, apperr.Wrap(apperr.CodeCurrencyMismatch, err, nil)
		}
		if leg.Amount.IsZero() || leg.AccountID == uuid.Nil {
			return nil, ErrInvalidEntry
		}
		key := legKey{leg.AccountID, leg.Amount.Currency}
		net, err := money.New(deltas[key], key.currency).Add(leg.Amount)
		if err != nil {
			return nil, fmt.Errorf("ledger: leg: %w", err)
		}
		deltas[key] = net.Minor
		running, seen := sums[key.currency]
		if !seen {
			running = money.Zero(key.currency)
		}
		sum, err := running.Add(leg.Amount)
		if err != nil {
			return nil, fmt.Errorf("ledger: sum: %w", err)
		}
		sums[key.currency] = sum
	}
	for _, sum := range sums {
		if sum.IsZero() {
			continue
		}
		if len(sums) > 1 { // value moved from one currency to another: never without FX legs
			return nil, apperr.Wrap(apperr.CodeCurrencyMismatch, ErrUnbalanced, nil)
		}
		return nil, ErrUnbalanced
	}
	return deltas, nil
}

// resolve maps every (account, currency) to its ledger account (account.not_found if missing).
func resolve(ctx context.Context, q *store.Queries, keys []legKey) (map[legKey]uuid.UUID, error) {
	accountIDs := make([]uuid.UUID, 0, len(keys))
	for _, k := range keys {
		accountIDs = append(accountIDs, k.account)
	}
	rows, err := q.ResolveLedgerAccounts(ctx, db.SortedIDs(accountIDs))
	if err != nil {
		return nil, fmt.Errorf("ledger: resolve accounts: %w", err)
	}
	known := make(map[legKey]uuid.UUID, len(rows))
	for _, r := range rows {
		known[legKey{r.AccountID, money.Currency(r.Currency)}] = r.ID
	}
	out := make(map[legKey]uuid.UUID, len(keys))
	for _, k := range keys {
		id, ok := known[k]
		if !ok {
			return nil, apperr.New(apperr.CodeAccountNotFound, map[string]any{"accountId": k.account.String()})
		}
		out[k] = id
	}
	return out, nil
}

// lock takes the balance rows FOR UPDATE in ascending id order (one global order: no deadlock
// between two writers of the same pair of accounts).
func lock(ctx context.Context, q *store.Queries, ledgerIDs []uuid.UUID) (map[uuid.UUID]store.LedgerBalance, error) {
	sorted := db.SortedIDs(ledgerIDs)
	rows, err := q.LockLedgerBalances(ctx, sorted)
	if err != nil {
		return nil, fmt.Errorf("ledger: lock balances: %w", err)
	}
	if len(rows) != len(sorted) {
		return nil, fmt.Errorf("ledger: %d balance rows for %d ledger accounts", len(rows), len(sorted))
	}
	out := make(map[uuid.UUID]store.LedgerBalance, len(rows))
	for _, r := range rows {
		out[r.LedgerAccountID] = r
	}
	return out, nil
}

// checkAvailable refuses a delta that would take a customer balance below its holds.
func checkAvailable(b store.LedgerBalance, key legKey, delta int64) error {
	if b.AllowNegative || delta >= 0 {
		return nil
	}
	available := b.PostedMinor - b.HeldMinor // posted >= held >= 0 (DB CHECK): no overflow
	if available+delta < 0 {                 // delta < 0 and available >= 0: no overflow
		return insufficient(key.account)
	}
	return nil
}

func (s *service) Hold(ctx context.Context, tx pgx.Tx, in HoldInput) (uuid.UUID, error) {
	if !in.Amount.IsPositive() {
		return uuid.Nil, apperr.New(apperr.CodeInvalidAmount, nil)
	}
	if _, err := money.ParseCurrency(string(in.Amount.Currency)); err != nil {
		return uuid.Nil, apperr.Wrap(apperr.CodeCurrencyMismatch, err, nil)
	}
	if in.RefID == uuid.Nil || in.RefType == "" || len(in.RefType) > maxRefTypeLen {
		return uuid.Nil, ErrInvalidEntry
	}
	q := store.New(tx)
	key := legKey{in.AccountID, in.Amount.Currency}
	ledgerIDs, err := resolve(ctx, q, []legKey{key})
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := q.FindHoldByRef(ctx, store.FindHoldByRefParams{RefType: in.RefType, RefID: in.RefID}); err == nil {
		return uuid.Nil, ErrDuplicateRef
	} else if !db.IsNoRows(err) {
		return uuid.Nil, fmt.Errorf("ledger: find hold: %w", err)
	}
	ledgerID := ledgerIDs[key]
	locked, err := lock(ctx, q, []uuid.UUID{ledgerID})
	if err != nil {
		return uuid.Nil, err
	}
	if err := checkAvailable(locked[ledgerID], key, -in.Amount.Minor); err != nil {
		return uuid.Nil, err
	}
	now := s.clock.Now()
	holdID := ids.New()
	if err := q.InsertHold(ctx, store.InsertHoldParams{
		ID: holdID, LedgerAccountID: ledgerID, Currency: string(in.Amount.Currency), AmountMinor: in.Amount.Minor,
		RefType: in.RefType, RefID: in.RefID, CreatedAt: now,
	}); err != nil {
		if db.IsUniqueViolation(err, holdRefKey) {
			return uuid.Nil, ErrDuplicateRef
		}
		return uuid.Nil, fmt.Errorf("ledger: insert hold: %w", err)
	}
	ok, err := adjustHeld(ctx, q, ledgerID, in.Amount.Minor, now)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, insufficient(in.AccountID)
	}
	return holdID, nil
}

func (s *service) ReleaseHold(ctx context.Context, tx pgx.Tx, holdID uuid.UUID) error {
	q := store.New(tx)
	hold, err := activeHold(ctx, q, holdID)
	if err != nil {
		return err
	}
	if _, err := lock(ctx, q, []uuid.UUID{hold.LedgerAccountID}); err != nil {
		return err
	}
	now := s.clock.Now()
	if err := releaseHeld(ctx, q, hold.LedgerAccountID, hold.AmountMinor, now); err != nil {
		return err
	}
	rows, err := q.ReleaseHold(ctx, store.ReleaseHoldParams{ReleasedAt: &now, ID: holdID})
	if err != nil {
		return fmt.Errorf("ledger: release hold: %w", err)
	}
	if rows != 1 { // impossible while the hold row is locked; never ignore it silently
		return apperr.New(apperr.CodeHoldNotActive, map[string]any{"holdId": holdID.String()})
	}
	return nil
}

// activeHold locks the hold row (always before its balance row) and requires it active.
func activeHold(ctx context.Context, q *store.Queries, holdID uuid.UUID) (store.Hold, error) {
	hold, err := q.GetHoldForUpdate(ctx, holdID)
	if db.IsNoRows(err) || (err == nil && hold.Status != "active") {
		return store.Hold{}, apperr.New(apperr.CodeHoldNotActive, map[string]any{"holdId": holdID.String()})
	}
	if err != nil {
		return store.Hold{}, fmt.Errorf("ledger: get hold: %w", err)
	}
	return hold, nil
}

// adjustHeld changes held_minor by delta under the conditional update; false means held
// would go negative or above the posted balance of a customer account.
func adjustHeld(ctx context.Context, q *store.Queries, ledgerID uuid.UUID, delta int64, now time.Time) (bool, error) {
	rows, err := q.AdjustHeld(ctx, store.AdjustHeldParams{DeltaMinor: delta, UpdatedAt: now, LedgerAccountID: ledgerID})
	if err != nil {
		return false, fmt.Errorf("ledger: adjust held: %w", err)
	}
	return rows == 1, nil
}

// releaseHeld gives back the amount of an active hold; failing means held_minor no longer
// covers the hold, a broken invariant that must stop the transaction.
func releaseHeld(ctx context.Context, q *store.Queries, ledgerID uuid.UUID, amount int64, now time.Time) error {
	ok, err := adjustHeld(ctx, q, ledgerID, -amount, now)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("ledger: held balance of %s does not cover the hold", ledgerID)
	}
	return nil
}

func (s *service) Balances(ctx context.Context, accountIDs []uuid.UUID) ([]Balance, error) {
	rows, err := store.New(s.pool).ListBalancesForAccounts(ctx, db.SortedIDs(accountIDs))
	if err != nil {
		return nil, fmt.Errorf("ledger: balances: %w", err)
	}
	out := make([]Balance, 0, len(rows))
	for _, r := range rows {
		out = append(out, newBalance(r.AccountID, r.Currency, r.PostedMinor, r.HeldMinor))
	}
	return out, nil
}

func (s *service) LockBalances(ctx context.Context, tx pgx.Tx, accountIDs []uuid.UUID) ([]Balance, error) {
	rows, err := store.New(tx).LockBalancesForAccounts(ctx, db.SortedIDs(accountIDs))
	if err != nil {
		return nil, fmt.Errorf("ledger: lock balances: %w", err)
	}
	out := make([]Balance, 0, len(rows))
	for _, r := range rows {
		out = append(out, newBalance(r.AccountID, r.Currency, r.PostedMinor, r.HeldMinor))
	}
	return out, nil
}

func insufficient(accountID uuid.UUID) error {
	return apperr.New(apperr.CodeInsufficientFunds, map[string]any{"accountId": accountID.String()})
}

// sortedKeys orders the legs by ledger account id, the same order as the locks.
func sortedKeys(deltas map[legKey]int64, ledgerIDs map[legKey]uuid.UUID) []legKey {
	keys := slices.Collect(maps.Keys(deltas))
	slices.SortFunc(keys, func(a, b legKey) int {
		x, y := ledgerIDs[a], ledgerIDs[b]
		return slices.Compare(x[:], y[:])
	})
	return keys
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

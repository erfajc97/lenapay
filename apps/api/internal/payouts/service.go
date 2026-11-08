package payouts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Service is the payouts API used by the handler, the worker and reconciliation.
type Service interface {
	ResolveDestination(ctx context.Context, p auth.Principal, to string) (psp.Destination, error)
	Quote(ctx context.Context, p auth.Principal, in QuoteInput) (Quote, error)
	// Create holds amount + fee and records the payout (idempotent by key, MO2/MO3/MO5).
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Payout, error)
	Get(ctx context.Context, p auth.Principal, payoutID uuid.UUID) (Payout, error)
	// Approve is 4-eyes (payout.self_approval_forbidden for the creator) and queues dispatch.
	Approve(ctx context.Context, p auth.Principal, payoutID uuid.UUID, idempotencyKey string) (Payout, error)
	Reject(ctx context.Context, p auth.Principal, payoutID uuid.UUID, reason string) (Payout, error)
	// Cancel lets the creator cancel while pending approval (releases the hold).
	Cancel(ctx context.Context, p auth.Principal, payoutID uuid.UUID) (Payout, error)
	ListApprovals(ctx context.Context, p auth.Principal, req page.Request) (page.Page[Payout], error)
	// UploadAttachment stores a supporting document of a payout (PDF/JPEG/PNG by magic bytes,
	// at most MaxAttachments per payout).
	UploadAttachment(ctx context.Context, p auth.Principal, payoutID uuid.UUID, in AttachmentUpload) (Attachment, error)
	// Proof renders the PDF proof of a settled payout (R-OUT-6).
	Proof(ctx context.Context, p auth.Principal, payoutID uuid.UUID) ([]byte, error)

	// Dispatch is the dispatch_payout job: one PSP attempt, then settle, fail or stay sending.
	Dispatch(ctx context.Context, payoutID uuid.UUID) error
	// Sweep is the periodic job that re-reads `sending` payouts at the PSP (R-OUT-5).
	Sweep(ctx context.Context) error
	// SettledBetween lists settled payouts of a currency in [from, to) (reconciliation).
	SettledBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]Payout, error)
}

// Deps are the collaborators of the payouts service.
type Deps struct {
	Pool          *pgxpool.Pool
	Clock         clock.Clock
	Audit         *audit.Logger
	Idempotency   *idempotency.Store
	Jobs          jobs.Enqueuer
	PSP           psp.Provider
	Accounts      Accounts
	Ledger        ledger.Service
	Fees          Fees
	Notifications Notifier
	Orgs          Members
	Documents     DocumentStore
	// CVUs and Credits route payouts to Lenapay CVUs on the internal rail (nil: always bank).
	CVUs    CVUs
	Credits Credits
	// Location is the business time zone of the proof's dates (UTC when nil).
	Location *time.Location
}

type service struct {
	deps Deps
}

// NewService returns the payouts service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// refType is the ledger reference type of every payout hold and posting.
const refType = "payout"

// Statuses stored with an idempotency key and replayed with it (the handler answers 201 for
// a create and 200 for an approval; the store keeps the number for any replaying door).
const (
	replayStatusCreated = 201
	replayStatusOK      = 200
)

// requireMoney is the service-side copy of the route rule of money mutations (R-KYC-3, R-ID-3,
// R-OUT-4): the guard enforces it first; this keeps the service safe for any other caller.
func requireMoney(p auth.Principal, minRole auth.Role, stepUp bool) error {
	if !p.HasOrg() {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if !p.Role.AtLeast(minRole) {
		return apperr.New(apperr.CodeForbidden, nil)
	}
	if p.KYCStatus != auth.KYCApproved {
		return apperr.New(apperr.CodeKYCNotApproved, nil)
	}
	if stepUp && !p.StepUp {
		return apperr.New(apperr.CodeStepUpRequired, nil)
	}
	return nil
}

func (s *service) ResolveDestination(ctx context.Context, p auth.Principal, to string) (psp.Destination, error) {
	if err := requireMoney(p, auth.RoleOperator, false); err != nil {
		return psp.Destination{}, err
	}
	t, err := s.resolve(ctx, to)
	return t.dest, err
}

func (s *service) Quote(ctx context.Context, p auth.Principal, in QuoteInput) (Quote, error) {
	if err := requireMoney(p, auth.RoleOperator, false); err != nil {
		return Quote{}, err
	}
	pr, err := s.prepare(ctx, p, in.AccountID, in.Amount, in.To)
	if err != nil {
		return Quote{}, err
	}
	balances, err := s.deps.Ledger.Balances(ctx, []uuid.UUID{in.AccountID})
	if err != nil {
		return Quote{}, fmt.Errorf("payouts: balances: %w", err)
	}
	available := money.Zero(in.Amount.Currency)
	for _, b := range balances {
		if b.Available.Currency == in.Amount.Currency {
			available = b.Available
		}
	}
	after, err := available.Sub(pr.total)
	if err != nil {
		return Quote{}, apperr.Wrap(apperr.CodeInvalidAmount, err, nil)
	}
	return Quote{Amount: in.Amount, Fee: pr.fee.Fee, Total: pr.total, Destination: pr.dest, AvailableAfter: after, Rail: pr.rail}, nil
}

// prepared is everything a payout is decided on, read BEFORE the transaction (no network
// call inside it, AG11) and recorded with the payout (MO5).
type prepared struct {
	account accounts.Account
	dest    psp.Destination
	rail    Rail
	fee     fees.Quote
	total   money.Amount
}

func (s *service) prepare(ctx context.Context, p auth.Principal, accountID uuid.UUID, amount money.Amount, to string) (prepared, error) {
	if _, err := money.ParseCurrency(string(amount.Currency)); err != nil {
		return prepared{}, apperr.Wrap(apperr.CodeCurrencyMismatch, err, nil)
	}
	if !amount.IsPositive() {
		return prepared{}, apperr.New(apperr.CodeInvalidAmount, nil)
	}
	account, err := s.deps.Accounts.Get(ctx, p.OrgID, accountID)
	if err != nil {
		return prepared{}, err
	}
	if account.Archived {
		return prepared{}, apperr.New(apperr.CodeAccountArchived, nil)
	}
	t, err := s.resolve(ctx, to)
	if err != nil {
		return prepared{}, err
	}
	op := fees.OperationPayout
	if t.onUs != nil {
		if err := checkOnUs(*t.onUs, accountID, amount.Currency); err != nil {
			return prepared{}, err
		}
		op = fees.OperationPayoutInternal
	}
	quote, err := s.deps.Fees.Quote(ctx, p.OrgID, op, amount)
	if err != nil {
		return prepared{}, err
	}
	total, err := amount.Add(quote.Fee)
	if err != nil {
		return prepared{}, apperr.Wrap(apperr.CodeInvalidAmount, err, nil)
	}
	return prepared{account: account, dest: t.dest, rail: t.rail(), fee: quote, total: total}, nil
}

// checkOnUs: an on-us payout goes to a CVU of the same currency on ANOTHER account (moving
// money between own accounts is an internal transfer).
func checkOnUs(dest payers.CVU, accountID uuid.UUID, currency money.Currency) error {
	if dest.Currency != currency {
		return apperr.New(apperr.CodeCurrencyMismatch, map[string]any{"field": "to"})
	}
	if dest.AccountID == accountID {
		return apperr.New(apperr.CodeDestinationInvalid, map[string]any{"reason": "same_account"})
	}
	return nil
}

// createRequest is what the idempotency key is bound to (MO2): same key + other content → 409.
type createRequest struct {
	AccountID uuid.UUID `json:"accountId"`
	Minor     int64     `json:"minor"`
	Currency  string    `json:"currency"`
	To        string    `json:"to"`
	Concept   string    `json:"concept"`
}

func (s *service) Create(ctx context.Context, p auth.Principal, in CreateInput) (Payout, error) {
	if err := requireMoney(p, auth.RoleOperator, true); err != nil {
		return Payout{}, err
	}
	concept := strings.TrimSpace(in.Concept)
	if err := validateCreate(in.IdempotencyKey, concept); err != nil {
		return Payout{}, err
	}
	pr, err := s.prepare(ctx, p, in.AccountID, in.Amount, in.To)
	if err != nil {
		return Payout{}, err
	}
	// Personal wallets send directly; company payouts wait for a second person (R-OUT-4).
	personal := pr.account.Kind == accounts.KindPersonal
	var approvers []uuid.UUID
	if !personal {
		if approvers, err = s.approvers(ctx, p); err != nil {
			return Payout{}, err
		}
	}
	key, err := idemKey(p.OrgID, "payouts.create", in.IdempotencyKey, createRequest{
		AccountID: in.AccountID, Minor: in.Amount.Minor, Currency: string(in.Amount.Currency),
		To: strings.TrimSpace(in.To), Concept: concept,
	})
	if err != nil {
		return Payout{}, err
	}

	var out Payout
	err = db.WithSerializable(ctx, s.deps.Pool, func(tx pgx.Tx) error {
		replay, err := s.deps.Idempotency.Claim(ctx, tx, key)
		if err != nil || replay != nil {
			return decodeReplay(replay, &out, err)
		}
		id, now := ids.New(), s.deps.Clock.Now()
		holdID, err := s.deps.Ledger.Hold(ctx, tx, ledger.HoldInput{AccountID: in.AccountID, Amount: pr.total, RefType: refType, RefID: id})
		if err != nil {
			return err
		}
		status := StatusPendingApproval
		if personal {
			status = StatusApproved
		}
		q := store.New(tx)
		if err := q.InsertPayout(ctx, insertParams(id, p, in, concept, pr, status, holdID, now)); err != nil {
			return fmt.Errorf("payouts: insert: %w", err)
		}
		if err := s.event(ctx, q, id, p.OrgID, nil, status, &p.UserID, nil); err != nil {
			return err
		}
		if err := s.afterCreate(ctx, tx, p, id, in, status, pr.rail, approvers); err != nil {
			return err
		}
		if out, err = load(ctx, q, id, p.OrgID); err != nil {
			return err
		}
		return complete(ctx, s.deps.Idempotency, tx, key, replayStatusCreated, out)
	})
	return out, err
}

func validateCreate(key, concept string) error {
	if n := len(key); n < minIdemKey || n > maxIdemKey {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "Idempotency-Key"})
	}
	if n := len([]rune(concept)); n < 1 || n > maxConcept {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "concept"})
	}
	return nil
}

func insertParams(id uuid.UUID, p auth.Principal, in CreateInput, concept string, pr prepared, status Status, holdID uuid.UUID, now time.Time) store.InsertPayoutParams {
	planID := pr.fee.PlanID
	var bank *string
	if pr.dest.BankName != "" {
		bank = &pr.dest.BankName
	}
	return store.InsertPayoutParams{
		ID: id, OrgID: p.OrgID, AccountID: in.AccountID, Currency: string(in.Amount.Currency),
		AmountMinor: in.Amount.Minor, FeeMinor: pr.fee.Fee.Minor, FeePlanID: &planID,
		DestinationType: string(pr.dest.Type), DestinationValue: pr.dest.Value, DestinationAccount: pr.dest.AccountNumber,
		DestinationHolderName: pr.dest.HolderName, DestinationHolderCuit: pr.dest.HolderCUIT, DestinationBankName: bank,
		Concept: concept, Status: string(status), CreatedBy: p.UserID, HoldID: &holdID,
		IdempotencyKey: in.IdempotencyKey, CreatedAt: now, Rail: string(pr.rail),
	}
}

// afterCreate writes the side effects of a new payout in its transaction: audit, dispatch job
// or on-us settlement (personal), approvers' notification, realtime events.
func (s *service) afterCreate(ctx context.Context, tx pgx.Tx, p auth.Principal, id uuid.UUID, in CreateInput, status Status, rail Rail, approvers []uuid.UUID) error {
	if err := s.audit(ctx, tx, p.OrgID, &p.UserID, "payout.create", id, in.IdempotencyKey); err != nil {
		return err
	}
	if status == StatusApproved {
		if err := s.release(ctx, tx, id, rail); err != nil {
			return err
		}
	}
	if status == StatusPendingApproval {
		if err := s.notify(ctx, tx, p.OrgID, approvers, "approval.requested", id, status); err != nil {
			return err
		}
		if err := publishPayout(ctx, tx, eventApprovalRequested, id, p.OrgID); err != nil {
			return err
		}
	}
	if err := publishBalance(ctx, tx, p.OrgID, in.AccountID); err != nil {
		return err
	}
	return publishPayout(ctx, tx, eventPayoutUpdated, id, p.OrgID)
}

// approvers are the members who can approve, except the caller (4-eyes).
func (s *service) approvers(ctx context.Context, p auth.Principal) ([]uuid.UUID, error) {
	members, err := s.deps.Orgs.ListMembers(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("payouts: approvers: %w", err)
	}
	var out []uuid.UUID
	for _, m := range members {
		if m.Role.AtLeast(auth.RoleApprover) && m.UserID != p.UserID {
			out = append(out, m.UserID)
		}
	}
	return out, nil
}

type approveRequest struct {
	PayoutID uuid.UUID `json:"payoutId"`
}

func (s *service) Approve(ctx context.Context, p auth.Principal, payoutID uuid.UUID, idempotencyKey string) (Payout, error) {
	if err := requireMoney(p, auth.RoleApprover, true); err != nil {
		return Payout{}, err
	}
	if n := len(idempotencyKey); n < minIdemKey || n > maxIdemKey {
		return Payout{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "Idempotency-Key"})
	}
	key, err := idemKey(p.OrgID, "payouts.approve", idempotencyKey, approveRequest{PayoutID: payoutID})
	if err != nil {
		return Payout{}, err
	}
	var out Payout
	err = db.WithSerializable(ctx, s.deps.Pool, func(tx pgx.Tx) error {
		replay, err := s.deps.Idempotency.Claim(ctx, tx, key)
		if err != nil || replay != nil {
			return decodeReplay(replay, &out, err)
		}
		q := store.New(tx)
		row, err := lockInOrg(ctx, q, payoutID, p.OrgID)
		if err != nil {
			return err
		}
		if row.CreatedBy == p.UserID {
			return apperr.New(apperr.CodePayoutSelfApproval, map[string]any{"payoutId": payoutID.String()})
		}
		now := s.deps.Clock.Now()
		rows, err := q.ApprovePayout(ctx, store.ApprovePayoutParams{ApprovedBy: &p.UserID, ApprovedAt: &now, ID: payoutID, OrgID: p.OrgID})
		if err != nil {
			return fmt.Errorf("payouts: approve: %w", err)
		}
		if rows != 1 {
			return invalidState(row)
		}
		from := StatusPendingApproval
		if err := s.event(ctx, q, payoutID, p.OrgID, &from, StatusApproved, &p.UserID, nil); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, p.OrgID, &p.UserID, "payout.approve", payoutID, idempotencyKey); err != nil {
			return err
		}
		if err := s.release(ctx, tx, payoutID, Rail(row.Rail)); err != nil {
			return err
		}
		if err := publishPayout(ctx, tx, eventPayoutUpdated, payoutID, p.OrgID); err != nil {
			return err
		}
		if out, err = load(ctx, q, payoutID, p.OrgID); err != nil {
			return err
		}
		return complete(ctx, s.deps.Idempotency, tx, key, replayStatusOK, out)
	})
	return out, err
}

func (s *service) Reject(ctx context.Context, p auth.Principal, payoutID uuid.UUID, reason string) (Payout, error) {
	if err := requireMoney(p, auth.RoleApprover, true); err != nil {
		return Payout{}, err
	}
	reason = strings.TrimSpace(reason)
	if n := len([]rune(reason)); n < 1 || n > maxRejectReason {
		return Payout{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "reason"})
	}
	return s.endPending(ctx, p, payoutID, StatusRejected, &reason)
}

func (s *service) Cancel(ctx context.Context, p auth.Principal, payoutID uuid.UUID) (Payout, error) {
	if err := requireMoney(p, auth.RoleOperator, false); err != nil {
		return Payout{}, err
	}
	return s.endPending(ctx, p, payoutID, StatusCancelled, nil)
}

// endPending rejects or cancels a pending payout and releases its hold: the money never left,
// so it is known the effect did not happen (MO5).
func (s *service) endPending(ctx context.Context, p auth.Principal, payoutID uuid.UUID, to Status, reason *string) (Payout, error) {
	var out Payout
	err := db.WithSerializable(ctx, s.deps.Pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := lockInOrg(ctx, q, payoutID, p.OrgID)
		if err != nil {
			return err
		}
		if to == StatusCancelled && row.CreatedBy != p.UserID {
			return apperr.New(apperr.CodeForbidden, map[string]any{"payoutId": payoutID.String()})
		}
		now := s.deps.Clock.Now()
		var rows int64
		if to == StatusRejected {
			rows, err = q.RejectPayout(ctx, store.RejectPayoutParams{RejectedBy: &p.UserID, RejectReason: reason, UpdatedAt: now, ID: payoutID, OrgID: p.OrgID})
		} else {
			rows, err = q.CancelPayout(ctx, store.CancelPayoutParams{UpdatedAt: now, ID: payoutID, OrgID: p.OrgID, CreatedBy: p.UserID})
		}
		if err != nil {
			return fmt.Errorf("payouts: %s: %w", to, err)
		}
		if rows != 1 {
			return invalidState(row)
		}
		if err := s.releaseHold(ctx, tx, row); err != nil {
			return err
		}
		from := StatusPendingApproval
		if err := s.event(ctx, q, payoutID, p.OrgID, &from, to, &p.UserID, reason); err != nil {
			return err
		}
		action := map[Status]string{StatusRejected: "payout.reject", StatusCancelled: "payout.cancel"}[to]
		if err := s.audit(ctx, tx, p.OrgID, &p.UserID, action, payoutID, ""); err != nil {
			return err
		}
		if to == StatusRejected {
			if err := s.notify(ctx, tx, p.OrgID, []uuid.UUID{row.CreatedBy}, "payout.rejected", payoutID, to); err != nil {
				return err
			}
		}
		if err := publishBalance(ctx, tx, p.OrgID, row.AccountID); err != nil {
			return err
		}
		if err := publishPayout(ctx, tx, eventPayoutUpdated, payoutID, p.OrgID); err != nil {
			return err
		}
		out, err = load(ctx, q, payoutID, p.OrgID)
		return err
	})
	return out, err
}

func (s *service) releaseHold(ctx context.Context, tx pgx.Tx, row store.Payout) error {
	if row.HoldID == nil { // every payout row is inserted with its hold
		return fmt.Errorf("payouts: payout %s has no hold", row.ID)
	}
	return s.deps.Ledger.ReleaseHold(ctx, tx, *row.HoldID)
}

func (s *service) Get(ctx context.Context, p auth.Principal, payoutID uuid.UUID) (Payout, error) {
	if !p.HasOrg() {
		return Payout{}, apperr.New(apperr.CodeOrgNotFound, nil)
	}
	return load(ctx, store.New(s.deps.Pool), payoutID, p.OrgID)
}

func (s *service) ListApprovals(ctx context.Context, p auth.Principal, req page.Request) (page.Page[Payout], error) {
	if !p.HasOrg() {
		return page.Page[Payout]{}, apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if !p.Role.AtLeast(auth.RoleApprover) {
		return page.Page[Payout]{}, apperr.New(apperr.CodeForbidden, nil)
	}
	params := store.ListPendingApprovalsParams{OrgID: p.OrgID, PageLimit: req.FetchLimit()}
	if req.After != nil {
		params.CursorCreatedAt, params.CursorID = &req.After.CreatedAt, &req.After.ID
	}
	rows, err := store.New(s.deps.Pool).ListPendingApprovals(ctx, params)
	if err != nil {
		return page.Page[Payout]{}, fmt.Errorf("payouts: list approvals: %w", err)
	}
	items := make([]Payout, 0, len(rows))
	for _, r := range rows {
		items = append(items, fromRow(r, nil))
	}
	return page.Build(items, req.Limit, func(p Payout) page.Position {
		return page.Position{CreatedAt: p.CreatedAt, ID: p.ID}
	}), nil
}

func (s *service) Proof(ctx context.Context, p auth.Principal, payoutID uuid.UUID) ([]byte, error) {
	po, err := s.Get(ctx, p, payoutID)
	if err != nil {
		return nil, err
	}
	if po.Status != StatusSettled {
		return nil, apperr.New(apperr.CodePayoutProofUnavailable, map[string]any{"payoutId": payoutID.String()})
	}
	loc := s.deps.Location
	if loc == nil {
		loc = time.UTC
	}
	return renderProof(po, loc)
}

func (s *service) SettledBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]Payout, error) {
	rows, err := store.New(s.deps.Pool).ListSettledPayoutsBetween(ctx, store.ListSettledPayoutsBetweenParams{
		Currency: string(currency), FromTime: &from, ToTime: &to,
	})
	if err != nil {
		return nil, fmt.Errorf("payouts: settled between: %w", err)
	}
	out := make([]Payout, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r, nil))
	}
	return out, nil
}

// --- helpers shared by the request and job paths ---

func idemKey(orgID uuid.UUID, op, value string, req any) (idempotency.Key, error) {
	hash, err := idempotency.HashRequest(req)
	if err != nil {
		return idempotency.Key{}, err
	}
	return idempotency.Key{Scope: idempotency.Scope(orgID, op), Value: value, RequestHash: hash}, nil
}

// decodeReplay returns the stored response of a completed key (R-OUT-3) or the claim error.
func decodeReplay(replay *idempotency.Stored, out *Payout, err error) error {
	if err != nil {
		return err
	}
	if uerr := json.Unmarshal(replay.Body, out); uerr != nil {
		return fmt.Errorf("payouts: decode replay: %w", uerr)
	}
	return nil
}

func complete(ctx context.Context, idem *idempotency.Store, tx pgx.Tx, key idempotency.Key, status int, out Payout) error {
	body, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("payouts: encode response: %w", err)
	}
	return idem.Complete(ctx, tx, key, status, body)
}

func lockInOrg(ctx context.Context, q *store.Queries, id, orgID uuid.UUID) (store.Payout, error) {
	row, err := q.GetPayoutForUpdateInOrg(ctx, store.GetPayoutForUpdateInOrgParams{ID: id, OrgID: orgID})
	if db.IsNoRows(err) {
		return store.Payout{}, apperr.New(apperr.CodePayoutNotFound, map[string]any{"payoutId": id.String()})
	}
	if err != nil {
		return store.Payout{}, fmt.Errorf("payouts: lock: %w", err)
	}
	return row, nil
}

func invalidState(row store.Payout) error {
	return apperr.New(apperr.CodePayoutInvalidState, map[string]any{"payoutId": row.ID.String(), "status": row.Status})
}

// load reads a payout of orgID with its history (payout.not_found when missing or foreign).
func load(ctx context.Context, q *store.Queries, id, orgID uuid.UUID) (Payout, error) {
	row, err := q.GetPayout(ctx, store.GetPayoutParams{ID: id, OrgID: orgID})
	if db.IsNoRows(err) {
		return Payout{}, apperr.New(apperr.CodePayoutNotFound, map[string]any{"payoutId": id.String()})
	}
	if err != nil {
		return Payout{}, fmt.Errorf("payouts: get: %w", err)
	}
	events, err := q.ListPayoutEvents(ctx, store.ListPayoutEventsParams{PayoutID: id, OrgID: orgID})
	if err != nil {
		return Payout{}, fmt.Errorf("payouts: events: %w", err)
	}
	attachments, err := q.ListPayoutAttachments(ctx, store.ListPayoutAttachmentsParams{PayoutID: id, OrgID: orgID})
	if err != nil {
		return Payout{}, fmt.Errorf("payouts: attachments: %w", err)
	}
	out := fromRow(row, events)
	out.Attachments = attachmentsFromRows(attachments)
	return out, nil
}

func fromRow(r store.Payout, events []store.PayoutEvent) Payout {
	c := money.Currency(r.Currency)
	dest := psp.Destination{
		Type: psp.DestinationType(r.DestinationType), Value: r.DestinationValue, AccountNumber: r.DestinationAccount,
		HolderName: r.DestinationHolderName, HolderCUIT: r.DestinationHolderCuit,
	}
	if r.DestinationBankName != nil {
		dest.BankName = *r.DestinationBankName
	}
	out := Payout{
		ID: r.ID, OrgID: r.OrgID, AccountID: r.AccountID, Amount: money.New(r.AmountMinor, c), Fee: money.New(r.FeeMinor, c),
		Destination: dest, Concept: r.Concept, Status: Status(r.Status), Rail: Rail(r.Rail), CreatedBy: r.CreatedBy, ApprovedBy: r.ApprovedBy,
		RejectReason: r.RejectReason, FailureCode: r.FailureCode, Events: make([]Event, 0, len(events)), Attachments: []Attachment{},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SettledAt: r.SettledAt,
		ProviderAttemptID: r.ProviderAttemptID, ProviderTransferID: r.ProviderTransferID,
	}
	for _, e := range events {
		ev := Event{ToStatus: Status(e.ToStatus), ActorID: e.ActorID, Reason: e.Reason, CreatedAt: e.CreatedAt}
		if e.FromStatus != nil {
			from := Status(*e.FromStatus)
			ev.FromStatus = &from
		}
		out.Events = append(out.Events, ev)
	}
	return out
}

func (s *service) event(ctx context.Context, q *store.Queries, id, orgID uuid.UUID, from *Status, to Status, actor *uuid.UUID, reason *string) error {
	var fromText *string
	if from != nil {
		f := string(*from)
		fromText = &f
	}
	if err := q.InsertPayoutEvent(ctx, store.InsertPayoutEventParams{
		ID: ids.New(), PayoutID: id, OrgID: orgID, FromStatus: fromText, ToStatus: string(to),
		ActorID: actor, Reason: reason, CreatedAt: s.deps.Clock.Now(),
	}); err != nil {
		return fmt.Errorf("payouts: event: %w", err)
	}
	return nil
}

func (s *service) audit(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, actor *uuid.UUID, action string, id uuid.UUID, key string) error {
	kind := audit.ActorUser
	if actor == nil {
		kind = audit.ActorSystem
	}
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &orgID, ActorID: actor, ActorKind: kind, Action: action, TargetType: "payout", TargetID: &id,
		IdempotencyKey: key, Payload: map[string]string{"payoutId": id.String()},
	})
}

func (s *service) notify(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, users []uuid.UUID, kind string, id uuid.UUID, status Status) error {
	if len(users) == 0 {
		return nil
	}
	return s.deps.Notifications.Notify(ctx, tx, notificationFor(orgID, users, kind, id, status))
}

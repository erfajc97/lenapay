package payins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payins/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/realtime"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Service is the payins API used by the handlers, the worker and reconciliation.
type Service interface {
	// ReceiveWebhook verifies, stores (deduplicated) and enqueues a PSP transfer event.
	ReceiveWebhook(ctx context.Context, d WebhookDelivery) error
	// ProcessTransfer is the job: re-read at the PSP, credit through the ledger, notify. eventID
	// is the stored webhook event it settles (uuid.Nil when none).
	ProcessTransfer(ctx context.Context, eventID uuid.UUID, providerTransferID string) error
	// Get returns a payin of p.OrgID (payin.not_found when missing or foreign).
	Get(ctx context.Context, p auth.Principal, payinID uuid.UUID) (Payin, error)
	// CreditedBetween lists credited bank-rail payins of a currency in [from, to) (reconciliation).
	CreditedBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]Payin, error)
	// CreditInternal records the receiving side of an on-us payout inside its settlement
	// transaction (the ledger entry is the payout's): a credited payin of rail internal, the
	// receivers' notification, the realtime payin.credited and the audit entry.
	CreditInternal(ctx context.Context, tx pgx.Tx, in InternalCredit) error
}

// Deps are the collaborators of the payins service.
type Deps struct {
	Pool          *pgxpool.Pool
	Clock         clock.Clock
	Audit         *audit.Logger
	Jobs          jobs.Enqueuer
	PSP           psp.Provider
	Payers        payers.Service
	Accounts      accounts.Service
	Members       Members
	Ledger        ledger.Service
	Fees          fees.Service
	Notifications notifications.Service
	WebhookSecret config.Secret
	Tolerance     time.Duration
}

type service struct {
	deps Deps
}

// NewService returns the payins service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// EventPayinCredited is the realtime and notification kind of a credited payin.
const EventPayinCredited = "payin.credited"

// ReceiveWebhook stores a verified event once and queues its processing in the SAME
// transaction (AG16): a redelivery inserts nothing and queues nothing.
func (s *service) ReceiveWebhook(ctx context.Context, d WebhookDelivery) error {
	if err := verifySignature([]byte(s.deps.WebhookSecret.Reveal()), d, s.deps.Clock.Now(), s.deps.Tolerance); err != nil {
		return apperr.Wrap(apperr.CodeWebhookSignature, err, nil)
	}
	ev, ok := decodeEvent(d.RawBody)
	if !ok {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	return db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		eventID, err := store.New(tx).InsertWebhookEvent(ctx, store.InsertWebhookEventParams{
			ID: ids.New(), ProviderEventID: ev.EventID, EventType: ev.Type, RawBody: d.RawBody, SignatureOk: true,
		})
		if db.IsNoRows(err) {
			return nil // already received: ON CONFLICT DO NOTHING returned no row
		}
		if err != nil {
			return fmt.Errorf("store webhook event: %w", err)
		}
		if _, err := s.deps.Jobs.InsertTx(ctx, tx, ProcessPayinArgs{EventID: eventID, TransferID: ev.TransferID}, nil); err != nil {
			return fmt.Errorf("queue payin: %w", err)
		}
		return nil
	})
}

// credit is everything decided before the credit transaction (no network call inside it).
type credit struct {
	payinID  uuid.UUID
	eventID  uuid.UUID
	payoutID *uuid.UUID // on-us: the payout this payin receives (rail internal)
	transfer psp.IncomingTransfer
	cvu      payers.CVU
	fee      money.Amount
	net      money.Amount
	master   uuid.UUID
	users    []uuid.UUID
	view     payinView
}

// ProcessTransfer credits an incoming transfer exactly once (MO4): the amount, the CVU and the
// origin come from the PSP's answer, never from the webhook. Uncertain answers return an error
// so River retries; definitive problems (unknown transfer or CVU) settle the event with a code.
func (s *service) ProcessTransfer(ctx context.Context, eventID uuid.UUID, transferID string) error {
	t, err := s.deps.PSP.GetTransfer(ctx, transferID)
	if errors.Is(err, psp.ErrNotFound) {
		return s.settleEvent(ctx, eventID, "psp.transfer_not_found")
	}
	if err != nil {
		return fmt.Errorf("re-read transfer %s: %w", transferID, err)
	}
	if done, err := s.alreadyCredited(ctx, transferID); err != nil || done {
		if err == nil {
			err = s.settleEvent(ctx, eventID, "")
		}
		return err
	}
	c, code, err := s.prepare(ctx, eventID, t)
	if err != nil {
		return err
	}
	if code != "" {
		slog.ErrorContext(ctx, "payin not credited", "transfer_id", transferID, "code", code)
		return s.settleEvent(ctx, eventID, code)
	}
	return db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error { return s.creditTx(ctx, tx, c) })
}

func (s *service) alreadyCredited(ctx context.Context, transferID string) (bool, error) {
	_, err := store.New(s.deps.Pool).GetPayinByProviderTransferID(ctx, transferID)
	if db.IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

// prepare resolves the CVU, the fee, the system account and who to notify. A non-empty code
// is a definitive reason not to credit.
func (s *service) prepare(ctx context.Context, eventID uuid.UUID, t psp.IncomingTransfer) (credit, string, error) {
	cvu, err := s.deps.Payers.CVUByNumber(ctx, t.CVU)
	if apperr.IsCode(err, apperr.CodeCVUNotFound) {
		return credit{}, string(apperr.CodeCVUNotFound), nil
	}
	if err != nil {
		return credit{}, "", err
	}
	if cvu.Currency != t.Amount.Currency {
		return credit{}, string(apperr.CodeCurrencyMismatch), nil
	}
	if !t.Amount.IsPositive() {
		return credit{}, string(apperr.CodeInvalidAmount), nil
	}
	quote, err := s.deps.Fees.Quote(ctx, cvu.OrgID, fees.OperationPayin, t.Amount)
	if err != nil {
		return credit{}, "", fmt.Errorf("payin fee: %w", err)
	}
	net, err := t.Amount.Sub(quote.Fee)
	if err != nil || net.IsNegative() || quote.Fee.IsNegative() {
		return credit{}, "", fmt.Errorf("payin fee %s on %s: %w", quote.Fee, t.Amount, errors.Join(err, errors.New("fee out of range")))
	}
	master, err := s.deps.Ledger.SystemAccountID(ctx, ledger.SystemPSPMaster)
	if err != nil {
		return credit{}, "", fmt.Errorf("psp master account: %w", err)
	}
	members, err := s.deps.Members.ListMembers(ctx, auth.Principal{OrgID: cvu.OrgID})
	if err != nil {
		return credit{}, "", fmt.Errorf("payin recipients: %w", err)
	}
	c := credit{payinID: ids.New(), eventID: eventID, transfer: t, cvu: cvu, fee: quote.Fee, net: net, master: master}
	for _, m := range members {
		c.users = append(c.users, m.UserID)
	}
	c.view = s.view(ctx, c)
	return c, "", nil
}

// creditTx posts the ledger entry, the payin row, the notification outbox, the realtime event,
// the audit entry and the event's settlement in ONE transaction. A transaction-scoped lock on
// the transfer id serializes concurrent jobs of the same transfer (two webhooks with different
// event ids); the UNIQUE provider_transfer_id is the second barrier.
func (s *service) creditTx(ctx context.Context, tx pgx.Tx, c credit) error {
	q := store.New(tx)
	if err := db.AdvisoryXactLock(ctx, tx, "payin:"+c.transfer.ID); err != nil {
		return err
	}
	if _, err := q.GetPayinByProviderTransferID(ctx, c.transfer.ID); err == nil {
		return s.settleEventTx(ctx, q, c.eventID, "")
	} else if !db.IsNoRows(err) {
		return fmt.Errorf("payin by transfer: %w", err)
	}
	legs := []ledger.Leg{
		{AccountID: c.master, Amount: money.New(-c.transfer.Amount.Minor, c.transfer.Amount.Currency)},
		{AccountID: c.cvu.AccountID, Amount: c.net},
	}
	if c.fee.IsPositive() {
		feesAccount, err := s.deps.Ledger.SystemAccountID(ctx, ledger.SystemFeesRevenue)
		if err != nil {
			return fmt.Errorf("fees revenue account: %w", err)
		}
		legs = append(legs, ledger.Leg{AccountID: feesAccount, Amount: c.fee})
	}
	orgID := c.cvu.OrgID
	ledgerTx, err := s.deps.Ledger.Post(ctx, tx, ledger.Entry{
		Kind: ledger.KindPayin, RefType: "payin", RefID: c.payinID, OrgID: &orgID,
		IdempotencyKey: "payin:" + c.transfer.ID, Memo: "payin " + c.transfer.ID, Legs: legs,
	})
	if err != nil {
		return fmt.Errorf("post payin: %w", err)
	}
	if err := s.record(ctx, tx, c, ledgerTx, audit.ActorPSP); err != nil {
		return err
	}
	return s.settleEventTx(ctx, q, c.eventID, "")
}

// Rails of a payin (DB CHECK payins.rail).
const (
	railBank     = "bank"
	railInternal = "internal"
)

// record writes the credited payin row, the notification outbox, the realtime event and the
// audit entry of a payin whose ledger entry ledgerTx is already posted (in tx).
func (s *service) record(ctx context.Context, tx pgx.Tx, c credit, ledgerTx uuid.UUID, actor audit.ActorKind) error {
	payinID, orgID, now := c.payinID, c.cvu.OrgID, s.deps.Clock.Now()
	rail := railBank
	if c.payoutID != nil {
		rail = railInternal
	}
	err := store.New(tx).InsertPayin(ctx, store.InsertPayinParams{
		ID: payinID, OrgID: orgID, AccountID: c.cvu.AccountID, CvuID: c.cvu.ID, PayerID: c.cvu.PayerID,
		Currency: string(c.transfer.Amount.Currency), GrossMinor: c.transfer.Amount.Minor, FeeMinor: c.fee.Minor, NetMinor: c.net.Minor,
		OriginName: nonEmpty(c.transfer.OriginName), OriginCuit: nonEmpty(c.transfer.OriginCUIT), OriginCbu: nonEmpty(c.transfer.OriginCBU),
		ProviderTransferID: c.transfer.ID, Status: string(StatusCredited), LedgerTransactionID: &ledgerTx, CreditedAt: &now,
		Rail: rail, PayoutID: c.payoutID,
	})
	if err != nil {
		return fmt.Errorf("insert payin: %w", err)
	}
	if err := s.deps.Notifications.Notify(ctx, tx, notifications.Notification{
		OrgID: &orgID, UserIDs: c.users, Kind: EventPayinCredited,
		Data: map[string]string{
			"payinId": payinID.String(), "accountId": c.cvu.AccountID.String(),
			"currency": string(c.transfer.Amount.Currency), "amountMinor": fmt.Sprint(c.transfer.Amount.Minor),
		},
	}); err != nil {
		return err
	}
	if err := s.publish(ctx, tx, c); err != nil {
		return err
	}
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &orgID, ActorKind: actor, Action: "payin.credit", TargetType: "payin", TargetID: &payinID,
		IdempotencyKey: "payin:" + c.transfer.ID, Payload: map[string]any{"gross": c.transfer.Amount.Minor, "fee": c.fee.Minor, "rail": rail},
	})
}

// CreditInternal: the receiver pays no fee on-us (the payer's plan charged it, if any); the
// provider id is "internal:<payoutId>", unique like any provider transfer id.
func (s *service) CreditInternal(ctx context.Context, tx pgx.Tx, in InternalCredit) error {
	if !in.Amount.IsPositive() || in.CVU.Currency != in.Amount.Currency {
		return apperr.New(apperr.CodeCurrencyMismatch, nil)
	}
	members, err := s.deps.Members.ListMembers(ctx, auth.Principal{OrgID: in.CVU.OrgID})
	if err != nil {
		return fmt.Errorf("payin recipients: %w", err)
	}
	payoutID := in.PayoutID
	c := credit{
		payinID: ids.New(), payoutID: &payoutID, cvu: in.CVU, fee: money.Zero(in.Amount.Currency), net: in.Amount,
		transfer: psp.IncomingTransfer{
			ID: "internal:" + in.PayoutID.String(), CVU: in.CVU.Number, Amount: in.Amount,
			OriginName: in.OriginName, OriginCUIT: in.OriginCUIT, OriginCBU: in.OriginCBU, OccurredAt: s.deps.Clock.Now(),
		},
	}
	for _, m := range members {
		c.users = append(c.users, m.UserID)
	}
	c.view = s.view(ctx, c)
	return s.record(ctx, tx, c, in.LedgerTransactionID, audit.ActorSystem)
}

// settleEvent records the outcome of a webhook event (no-op without an event).
func (s *service) settleEvent(ctx context.Context, eventID uuid.UUID, code string) error {
	return s.settleEventTx(ctx, store.New(s.deps.Pool), eventID, code)
}

func (s *service) settleEventTx(ctx context.Context, q *store.Queries, eventID uuid.UUID, code string) error {
	if eventID == uuid.Nil {
		return nil
	}
	now := s.deps.Clock.Now()
	if err := q.MarkWebhookEventProcessed(ctx, store.MarkWebhookEventProcessedParams{ID: eventID, ProcessedAt: &now, ErrorCode: nonEmpty(code)}); err != nil {
		return fmt.Errorf("settle webhook event: %w", err)
	}
	return nil
}

func (s *service) Get(ctx context.Context, p auth.Principal, payinID uuid.UUID) (Payin, error) {
	row, err := store.New(s.deps.Pool).GetPayin(ctx, store.GetPayinParams{ID: payinID, OrgID: p.OrgID})
	if db.IsNoRows(err) {
		return Payin{}, apperr.New(apperr.CodePayinNotFound, nil)
	}
	if err != nil {
		return Payin{}, fmt.Errorf("get payin: %w", err)
	}
	return toPayin(row), nil
}

func (s *service) CreditedBetween(ctx context.Context, currency money.Currency, from, to time.Time) ([]Payin, error) {
	rows, err := store.New(s.deps.Pool).ListCreditedPayinsBetween(ctx, store.ListCreditedPayinsBetweenParams{
		Currency: string(currency), FromTime: &from, ToTime: &to,
	})
	if err != nil {
		return nil, fmt.Errorf("credited payins: %w", err)
	}
	out := make([]Payin, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPayin(r))
	}
	return out, nil
}

func toPayin(r store.Payin) Payin {
	c := money.Currency(r.Currency)
	return Payin{
		ID: r.ID, OrgID: r.OrgID, AccountID: r.AccountID, PayerID: r.PayerID, CVUID: r.CvuID,
		Gross: money.New(r.GrossMinor, c), Fee: money.New(r.FeeMinor, c), Net: money.New(r.NetMinor, c),
		Origin:             Origin{Name: r.OriginName, CUIT: r.OriginCuit, CBU: r.OriginCbu},
		ProviderTransferID: r.ProviderTransferID, Status: Status(r.Status), CreatedAt: r.CreatedAt, CreditedAt: r.CreditedAt,
	}
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// payinView is the payin as the app's live banner reads it (apps/mobile LiveEvent "payin").
type payinView struct {
	ID          uuid.UUID  `json:"id"`
	CVUID       uuid.UUID  `json:"cvuId"`
	PayerID     *uuid.UUID `json:"payerId"`
	PayerName   string     `json:"payerName"`
	AccountID   uuid.UUID  `json:"accountId"`
	AccountName string     `json:"accountName"`
	Currency    string     `json:"currency"`
	Amount      int64      `json:"amount"`
	Fee         int64      `json:"fee"`
	Net         int64      `json:"net"`
	OriginName  string     `json:"originName"`
	OriginCUIT  string     `json:"originCuit"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// view builds the banner data. Names are best effort: a lookup failure never blocks money.
func (s *service) view(ctx context.Context, c credit) payinView {
	v := payinView{
		ID: c.payinID, CVUID: c.cvu.ID, PayerID: c.cvu.PayerID, AccountID: c.cvu.AccountID,
		Currency: string(c.transfer.Amount.Currency), Amount: c.transfer.Amount.Minor, Fee: c.fee.Minor, Net: c.net.Minor,
		OriginName: c.transfer.OriginName, OriginCUIT: c.transfer.OriginCUIT, CreatedAt: s.deps.Clock.Now(),
	}
	system := auth.Principal{OrgID: c.cvu.OrgID}
	if c.cvu.PayerID != nil {
		if payer, err := s.deps.Payers.Get(ctx, system, *c.cvu.PayerID); err == nil {
			v.PayerName = payer.Name
		} else {
			slog.WarnContext(ctx, "payin view: payer name", "err", err.Error())
		}
	}
	if acc, err := s.deps.Accounts.Get(ctx, c.cvu.OrgID, c.cvu.AccountID); err == nil {
		v.AccountName = acc.Name
	} else {
		slog.WarnContext(ctx, "payin view: account name", "err", err.Error())
	}
	return v
}

// publish queues the live event inside the credit transaction (delivered at commit only).
func (s *service) publish(ctx context.Context, tx pgx.Tx, c credit) error {
	data, err := json.Marshal(struct {
		PayinID   uuid.UUID  `json:"payinId"`
		AccountID uuid.UUID  `json:"accountId"`
		CVUID     uuid.UUID  `json:"cvuId"`
		PayerID   *uuid.UUID `json:"payerId"`
		Payin     payinView  `json:"payin"`
	}{c.payinID, c.cvu.AccountID, c.cvu.ID, c.cvu.PayerID, c.view})
	if err != nil {
		return fmt.Errorf("payin event: %w", err)
	}
	return realtime.Publish(ctx, tx, c.cvu.OrgID, realtime.Event{Type: EventPayinCredited, Data: data})
}

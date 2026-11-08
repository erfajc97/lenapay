package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/accounts/store"
	"lenapay/apps/api/internal/ledger"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/idempotency"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/platform/money"
)

const (
	opTransfer = "accounts.internal_transfer"
	// refTypeInternalTransfer is the ledger ref_type of internal transfers.
	refTypeInternalTransfer = "internal_transfer"
	// statusCreated is replayed with the stored body; the service never imports net/http.
	statusCreated = 201
)

// transferRequest is everything that changes the effect: the idempotency key is bound to it.
type transferRequest struct {
	From     uuid.UUID `json:"from"`
	To       uuid.UUID `json:"to"`
	Minor    int64     `json:"minor"`
	Currency string    `json:"currency"`
	Concept  *string   `json:"concept"`
}

// storedTransfer is the replayable response kept with the idempotency key.
type storedTransfer struct {
	ID        uuid.UUID `json:"id"`
	From      uuid.UUID `json:"from"`
	To        uuid.UUID `json:"to"`
	Minor     int64     `json:"minor"`
	Currency  string    `json:"currency"`
	Concept   *string   `json:"concept"`
	CreatedAt time.Time `json:"createdAt"`
}

// Transfer moves money between two accounts of the caller's org in one currency, immediately
// and once per idempotency key: claim, posting, audit and stored response share ONE
// serializable transaction (MO2, MO3).
func (s *service) Transfer(ctx context.Context, p auth.Principal, in TransferInput) (Transfer, error) {
	req, err := validateTransfer(p, in)
	if err != nil {
		return Transfer{}, err
	}
	hash, err := idempotency.HashRequest(req)
	if err != nil {
		return Transfer{}, err
	}
	key := idempotency.Key{Scope: idempotency.Scope(p.OrgID, opTransfer), Value: in.IdempotencyKey, RequestHash: hash}

	var result storedTransfer
	err = db.WithSerializable(ctx, s.deps.Pool, func(tx pgx.Tx) error {
		replay, err := s.deps.Idempotency.Claim(ctx, tx, key)
		if err != nil {
			return err
		}
		if replay != nil {
			return json.Unmarshal(replay.Body, &result)
		}
		if err := s.checkTransferAccounts(ctx, tx, p.OrgID, req); err != nil {
			return err
		}
		result = storedTransfer{
			ID: ids.New(), From: req.From, To: req.To, Minor: req.Minor, Currency: req.Currency,
			Concept: req.Concept, CreatedAt: s.deps.Clock.Now(),
		}
		if err := s.postTransfer(ctx, tx, p, in.IdempotencyKey, result); err != nil {
			return err
		}
		body, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("accounts: encode transfer: %w", err)
		}
		return s.deps.Idempotency.Complete(ctx, tx, key, statusCreated, body)
	})
	if err != nil {
		return Transfer{}, err
	}
	return Transfer{
		ID: result.ID, FromAccountID: result.From, ToAccountID: result.To,
		Amount: money.New(result.Minor, money.Currency(result.Currency)), Concept: result.Concept, CreatedAt: result.CreatedAt,
	}, nil
}

func validateTransfer(p auth.Principal, in TransferInput) (transferRequest, error) {
	if !p.StepUp { // the guard requires it too; a money move never runs without it (R-ID-3)
		return transferRequest{}, apperr.New(apperr.CodeStepUpRequired, nil)
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return transferRequest{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "Idempotency-Key"})
	}
	if _, err := money.ParseCurrency(string(in.Amount.Currency)); err != nil {
		return transferRequest{}, apperr.Wrap(apperr.CodeCurrencyMismatch, err, nil)
	}
	if !in.Amount.IsPositive() {
		return transferRequest{}, apperr.New(apperr.CodeInvalidAmount, nil)
	}
	if in.FromAccountID == in.ToAccountID {
		return transferRequest{}, apperr.New(apperr.CodeTransferSameAccount, nil)
	}
	var concept *string
	if c := strings.TrimSpace(in.Concept); c != "" {
		if utf8.RuneCountInString(c) > maxConceptLen {
			return transferRequest{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "concept"})
		}
		concept = &c
	}
	return transferRequest{
		From: in.FromAccountID, To: in.ToAccountID, Minor: in.Amount.Minor, Currency: string(in.Amount.Currency), Concept: concept,
	}, nil
}

// checkTransferAccounts requires both accounts in the caller's org (else 404, T2) and active,
// holding their rows shared until commit so an archive cannot interleave.
func (s *service) checkTransferAccounts(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, req transferRequest) error {
	rows, err := store.New(tx).LockAccountsShared(ctx, store.LockAccountsSharedParams{
		OrgID: &orgID, Ids: db.SortedIDs([]uuid.UUID{req.From, req.To}),
	})
	if err != nil {
		return fmt.Errorf("accounts: lock transfer accounts: %w", err)
	}
	found := map[uuid.UUID]store.Account{}
	for _, r := range rows {
		found[r.ID] = r
	}
	for _, id := range []uuid.UUID{req.From, req.To} {
		acc, ok := found[id]
		if !ok {
			return apperr.New(apperr.CodeAccountNotFound, map[string]any{"accountId": id.String()})
		}
		if acc.ArchivedAt != nil {
			return apperr.New(apperr.CodeAccountArchived, map[string]any{"accountId": id.String()})
		}
	}
	return nil
}

func (s *service) postTransfer(ctx context.Context, tx pgx.Tx, p auth.Principal, idemKey string, t storedTransfer) error {
	currency := money.Currency(t.Currency)
	memo := ""
	if t.Concept != nil {
		memo = *t.Concept
	}
	if _, err := s.deps.Ledger.Post(ctx, tx, ledger.Entry{
		Kind: ledger.KindInternalTransfer, RefType: refTypeInternalTransfer, RefID: t.ID,
		OrgID: &p.OrgID, IdempotencyKey: idemKey, Memo: memo, CreatedBy: &p.UserID,
		Legs: []ledger.Leg{
			{AccountID: t.From, Amount: money.New(-t.Minor, currency)},
			{AccountID: t.To, Amount: money.New(t.Minor, currency)},
		},
	}); err != nil {
		return err
	}
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &p.OrgID, ActorID: &p.UserID, ActorKind: audit.ActorUser,
		Action: "internal_transfer.create", TargetType: "internal_transfer", TargetID: &t.ID,
		IdempotencyKey: idemKey, Payload: t,
	})
}

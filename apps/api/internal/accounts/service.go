package accounts

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts/store"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Service is the accounts API used by the handler and by other features.
type Service interface {
	// List returns p.OrgID's accounts with balances.
	List(ctx context.Context, p auth.Principal, includeArchived bool) ([]Account, error)
	// Create opens a project account (R-ACC-3).
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Account, error)
	// Update renames or archives an account (archive only project accounts with zero balance).
	Update(ctx context.Context, p auth.Principal, accountID uuid.UUID, in UpdateInput) (Account, error)
	// Summary returns totals per currency and per active account.
	Summary(ctx context.Context, p auth.Principal) (Summary, error)
	// Transfer posts an internal transfer (R-ACC-4, idempotent, MO2/MO3).
	Transfer(ctx context.Context, p auth.Principal, in TransferInput) (Transfer, error)

	// Get returns an account of orgID (account.not_found when missing or foreign, T2).
	Get(ctx context.Context, orgID, accountID uuid.UUID) (Account, error)
	// OnKYCApproved opens the main (company) or personal account (kyc.ApprovalHook).
	OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
	// ImportProject creates (or returns) the project account of an ERP project (erplink import).
	ImportProject(ctx context.Context, orgID uuid.UUID, name, erpProjectRef string) (Account, error)
}

// Deps are the collaborators of the accounts service.
type Deps struct {
	Pool        *pgxpool.Pool
	Clock       clock.Clock
	Audit       *audit.Logger
	Idempotency *idempotency.Store
	Ledger      ledger.Service
}

type service struct {
	deps Deps
}

// NewService returns the accounts service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// Caps equal to the DB CHECKs and the contract (G12).
const (
	maxNameLen    = 80
	maxConceptLen = 140
	maxERPRefLen  = 120
)

// Stored names of the accounts opened on KYC approval. The app labels main and personal
// accounts by kind through i18n (P3); these are only the editable defaults.
const (
	mainAccountName     = "Main account"
	personalAccountName = "Personal account"
)

func (s *service) List(ctx context.Context, p auth.Principal, includeArchived bool) ([]Account, error) {
	rows, err := store.New(s.deps.Pool).ListAccounts(ctx, store.ListAccountsParams{OrgID: &p.OrgID, IncludeArchived: includeArchived})
	if err != nil {
		return nil, fmt.Errorf("accounts: list: %w", err)
	}
	return s.withBalances(ctx, rows)
}

func (s *service) Get(ctx context.Context, orgID, accountID uuid.UUID) (Account, error) {
	row, err := store.New(s.deps.Pool).GetAccount(ctx, store.GetAccountParams{ID: accountID, OrgID: &orgID})
	if err != nil {
		return Account{}, notFound(err, accountID)
	}
	out, err := s.withBalances(ctx, []store.Account{row})
	if err != nil {
		return Account{}, err
	}
	return out[0], nil
}

func (s *service) Create(ctx context.Context, p auth.Principal, in CreateInput) (Account, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return Account{}, err
	}
	id := ids.New()
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.open(ctx, tx, p.OrgID, id, KindProject, name, nil); err != nil {
			return err
		}
		return s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &p.OrgID, ActorID: &p.UserID, ActorKind: audit.ActorUser,
			Action: "account.create", TargetType: "account", TargetID: &id,
		})
	})
	if err != nil {
		return Account{}, err
	}
	return s.Get(ctx, p.OrgID, id)
}

func (s *service) Update(ctx context.Context, p auth.Principal, accountID uuid.UUID, in UpdateInput) (Account, error) {
	var name string
	if in.Name != nil {
		var err error
		if name, err = cleanName(*in.Name); err != nil {
			return Account{}, err
		}
	}
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		acc, err := q.GetAccountForUpdate(ctx, store.GetAccountForUpdateParams{ID: accountID, OrgID: &p.OrgID})
		if err != nil {
			return notFound(err, accountID)
		}
		now := s.deps.Clock.Now()
		if in.Name != nil {
			if err := checkRows(q.RenameAccount(ctx, store.RenameAccountParams{Name: name, UpdatedAt: now, ID: accountID, OrgID: &p.OrgID})); err != nil {
				return err
			}
		}
		if in.Archived != nil {
			if err := s.setArchived(ctx, tx, acc, *in.Archived); err != nil {
				return err
			}
		}
		return s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &p.OrgID, ActorID: &p.UserID, ActorKind: audit.ActorUser,
			Action: "account.update", TargetType: "account", TargetID: &accountID, Payload: in,
		})
	})
	if err != nil {
		return Account{}, err
	}
	return s.Get(ctx, p.OrgID, accountID)
}

// setArchived archives only a project account whose every balance (posted and held) is zero,
// read under lock so no posting can land between the check and the archive.
func (s *service) setArchived(ctx context.Context, tx pgx.Tx, acc store.Account, archived bool) error {
	q := store.New(tx)
	now := s.deps.Clock.Now()
	params := store.SetAccountArchivedParams{UpdatedAt: now, ID: acc.ID, OrgID: acc.OrgID}
	if !archived {
		return checkRows(q.SetAccountArchived(ctx, params))
	}
	if acc.ArchivedAt != nil {
		return nil
	}
	if Kind(acc.Kind) != KindProject {
		return apperr.New(apperr.CodeAccountNotArchivable, map[string]any{"kind": acc.Kind})
	}
	balances, err := s.deps.Ledger.LockBalances(ctx, tx, []uuid.UUID{acc.ID})
	if err != nil {
		return err
	}
	for _, b := range balances {
		if !b.Posted.IsZero() || !b.Held.IsZero() {
			return apperr.New(apperr.CodeAccountNotEmpty, map[string]any{"accountId": acc.ID.String(), "currency": string(b.Posted.Currency)})
		}
	}
	params.ArchivedAt = &now
	return checkRows(q.SetAccountArchived(ctx, params))
}

func (s *service) Summary(ctx context.Context, p auth.Principal) (Summary, error) {
	accounts, err := s.List(ctx, p, false)
	if err != nil {
		return Summary{}, err
	}
	totals := map[money.Currency]CurrencyBalance{}
	for _, c := range ledger.Currencies() {
		totals[c] = CurrencyBalance{Posted: money.Zero(c), Held: money.Zero(c), Available: money.Zero(c)}
	}
	for _, acc := range accounts {
		for _, b := range acc.Balances {
			t := totals[b.Posted.Currency]
			if t, err = addBalance(t, b); err != nil {
				return Summary{}, err
			}
			totals[b.Posted.Currency] = t
		}
	}
	out := Summary{Accounts: accounts}
	for _, c := range ledger.Currencies() {
		out.Totals = append(out.Totals, totals[c])
	}
	return out, nil
}

func addBalance(t, b CurrencyBalance) (CurrencyBalance, error) {
	var err error
	if t.Posted, err = t.Posted.Add(b.Posted); err != nil {
		return t, fmt.Errorf("accounts: total posted: %w", err)
	}
	if t.Held, err = t.Held.Add(b.Held); err != nil {
		return t, fmt.Errorf("accounts: total held: %w", err)
	}
	if t.Available, err = t.Available.Add(b.Available); err != nil {
		return t, fmt.Errorf("accounts: total available: %w", err)
	}
	return t, nil
}

func (s *service) OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	kind, name := KindMain, mainAccountName
	switch orgKind {
	case "company":
	case "personal":
		kind, name = KindPersonal, personalAccountName
	default:
		return fmt.Errorf("accounts: unknown org kind %q", orgKind)
	}
	_, err := store.New(tx).GetMainAccount(ctx, &orgID)
	if err == nil {
		return nil // approved before (re-run of the hook): one main/personal account per org
	}
	if !db.IsNoRows(err) {
		return fmt.Errorf("accounts: main account: %w", err)
	}
	id := ids.New()
	if err := s.open(ctx, tx, orgID, id, kind, name, nil); err != nil {
		return err
	}
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &orgID, ActorKind: audit.ActorSystem, Action: "account.open", TargetType: "account", TargetID: &id,
	})
}

func (s *service) ImportProject(ctx context.Context, orgID uuid.UUID, name, erpProjectRef string) (Account, error) {
	clean, err := cleanName(name)
	if err != nil {
		return Account{}, err
	}
	ref := strings.TrimSpace(erpProjectRef)
	if ref == "" || utf8.RuneCountInString(ref) > maxERPRefLen {
		return Account{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "erpProjectRef"})
	}
	var id uuid.UUID
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		existing, err := store.New(tx).GetAccountByERPRef(ctx, store.GetAccountByERPRefParams{OrgID: &orgID, ErpProjectRef: &ref})
		if err == nil {
			id = existing.ID // re-import: the snapshot already has this project (R-ERP-3)
			return nil
		}
		if !db.IsNoRows(err) {
			return fmt.Errorf("accounts: find erp project: %w", err)
		}
		id = ids.New()
		if err := s.open(ctx, tx, orgID, id, KindProject, clean, &ref); err != nil {
			return err
		}
		return s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &orgID, ActorKind: audit.ActorSystem, Action: "account.import", TargetType: "account", TargetID: &id,
		})
	})
	if err != nil {
		return Account{}, err
	}
	return s.Get(ctx, orgID, id)
}

// open inserts an org account and its ledger accounts in tx.
func (s *service) open(ctx context.Context, tx pgx.Tx, orgID, id uuid.UUID, kind Kind, name string, erpRef *string) error {
	if err := store.New(tx).InsertAccount(ctx, store.InsertAccountParams{
		ID: id, OrgID: &orgID, Kind: string(kind), Name: name, ErpProjectRef: erpRef, CreatedAt: s.deps.Clock.Now(),
	}); err != nil {
		return fmt.Errorf("accounts: insert: %w", err)
	}
	return s.deps.Ledger.OpenAccount(ctx, tx, id)
}

// withBalances attaches the ledger balances (ARS, USD) to each account.
func (s *service) withBalances(ctx context.Context, rows []store.Account) ([]Account, error) {
	accountIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		accountIDs = append(accountIDs, r.ID)
	}
	balances, err := s.deps.Ledger.Balances(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	byAccount := map[uuid.UUID][]CurrencyBalance{}
	for _, b := range balances {
		byAccount[b.AccountID] = append(byAccount[b.AccountID], CurrencyBalance{Posted: b.Posted, Held: b.Held, Available: b.Available})
	}
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, Account{
			ID: r.ID, OrgID: *r.OrgID, Kind: Kind(r.Kind), Name: r.Name, Archived: r.ArchivedAt != nil,
			ERPProjectRef: r.ErpProjectRef, Balances: byAccount[r.ID], CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func cleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxNameLen {
		return "", apperr.New(apperr.CodeValidation, map[string]any{"field": "name"})
	}
	return name, nil
}

// notFound maps a missing (or another org's) account to account.not_found (T2).
func notFound(err error, accountID uuid.UUID) error {
	if db.IsNoRows(err) {
		return apperr.New(apperr.CodeAccountNotFound, map[string]any{"accountId": accountID.String()})
	}
	return fmt.Errorf("accounts: get: %w", err)
}

// checkRows requires exactly one row from a conditional update on a row locked earlier.
func checkRows(rows int64, err error) error {
	if err != nil {
		return fmt.Errorf("accounts: update: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("accounts: update touched %d rows, want 1", rows)
	}
	return nil
}

package payers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/payers/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/idempotency"
	"lenapay/apps/api/internal/platform/ids"
	"lenapay/apps/api/internal/platform/jobs"
	"lenapay/apps/api/internal/platform/page"
	"lenapay/apps/api/internal/psp"
)

// Service is the payers/CVU API used by the handler and by other features.
type Service interface {
	List(ctx context.Context, p auth.Principal, in ListInput) (page.Page[Payer], error)
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Payer, error)
	Get(ctx context.Context, p auth.Principal, payerID uuid.UUID) (Payer, error)
	Update(ctx context.Context, p auth.Principal, payerID uuid.UUID, in UpdateInput) (Payer, error)
	// CreateCVU issues the payer's CVU in a currency at the PSP (one per payer per currency).
	CreateCVU(ctx context.Context, p auth.Principal, payerID uuid.UUID, in CreateCVUInput) (CVU, error)
	UpdateCVUAlias(ctx context.Context, p auth.Principal, cvuID uuid.UUID, alias string) (CVU, error)
	DisableCVU(ctx context.Context, p auth.Principal, cvuID uuid.UUID) error

	// CVUByNumber resolves a receiving CVU for payins (system scope; cvu.not_found otherwise).
	CVUByNumber(ctx context.Context, number string) (CVU, error)
	// ImportPayer creates or returns the payer of an ERP buyer (erplink import, idempotent by ERPRef).
	ImportPayer(ctx context.Context, orgID uuid.UUID, in ImportInput) (Payer, error)
	// OnKYCApproved queues the personal CVUs of a personal org (kyc.ApprovalHook).
	OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
	// CreatePersonalCVUs is the job behind OnKYCApproved: one CVU per currency (R-CVU-3).
	CreatePersonalCVUs(ctx context.Context, orgID uuid.UUID) error
	// OnUsDestination finds an ACTIVE Lenapay CVU by number or alias (payouts route it on the
	// internal rail, never through the PSP); found = false otherwise. System scope.
	OnUsDestination(ctx context.Context, to string) (dest OnUs, found bool, err error)
	// Holder returns an org's holder name and document as the PSP shows it (system scope).
	Holder(ctx context.Context, orgID uuid.UUID) (name, doc string, err error)
	// ReceivingCVUs lists the org's account-owned CVUs (accounts.ReceivingCVUs: Account.cvus).
	ReceivingCVUs(ctx context.Context, orgID uuid.UUID) ([]accounts.ReceivingCVU, error)
	// UpdateAccountCVUAlias changes the alias of an account's own CVU (updateAccountCvu); a
	// payer's CVU or another account's is cvu.not_found.
	UpdateAccountCVUAlias(ctx context.Context, p auth.Principal, accountID, cvuID uuid.UUID, alias string) (accounts.ReceivingCVU, error)
}

// Deps are the collaborators of the payers service.
type Deps struct {
	Pool        *pgxpool.Pool
	Clock       clock.Clock
	Audit       *audit.Logger
	Idempotency *idempotency.Store
	Jobs        jobs.Enqueuer
	PSP         psp.Provider
	Accounts    accounts.Service
	Orgs        Orgs
	// ShareBaseURL prefixes CVU share links (config CVU_SHARE_BASE_URL).
	ShareBaseURL string
}

type service struct {
	deps Deps
}

// NewService returns the payers service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// Name caps of the payers table (migration CHECK, contract minLength/maxLength).
const nameMin, nameMax = 2, 120

func (s *service) List(ctx context.Context, p auth.Principal, in ListInput) (page.Page[Payer], error) {
	params := store.ListPayersParams{OrgID: p.OrgID, AccountID: in.AccountID, PageLimit: in.Page.FetchLimit()}
	if in.Query != nil {
		if q := strings.TrimSpace(*in.Query); q != "" {
			params.Search = &q
		}
	}
	if in.Page.After != nil {
		params.CursorCreatedAt, params.CursorID = &in.Page.After.CreatedAt, &in.Page.After.ID
	}
	rows, err := store.New(s.deps.Pool).ListPayers(ctx, params)
	if err != nil {
		return page.Page[Payer]{}, fmt.Errorf("list payers: %w", err)
	}
	pg := page.Build(rows, in.Page.Limit, func(r store.Payer) page.Position {
		return page.Position{CreatedAt: r.CreatedAt, ID: r.ID}
	})
	payers, err := s.withCVUs(ctx, store.New(s.deps.Pool), p.OrgID, pg.Items)
	if err != nil {
		return page.Page[Payer]{}, err
	}
	return page.Page[Payer]{Items: payers, NextCursor: pg.NextCursor}, nil
}

// withCVUs maps rows to payers with their CVUs (one query for the whole page).
func (s *service) withCVUs(ctx context.Context, q *store.Queries, orgID uuid.UUID, rows []store.Payer) ([]Payer, error) {
	idList := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		idList = append(idList, r.ID)
	}
	cvuRows, err := q.ListCVUsForPayers(ctx, store.ListCVUsForPayersParams{OrgID: orgID, PayerIds: idList})
	if err != nil {
		return nil, fmt.Errorf("list cvus: %w", err)
	}
	byPayer := map[uuid.UUID][]CVU{}
	for _, c := range cvuRows {
		byPayer[*c.PayerID] = append(byPayer[*c.PayerID], s.toCVU(c))
	}
	out := make([]Payer, 0, len(rows))
	for _, r := range rows {
		payer := toPayer(r)
		payer.CVUs = append(payer.CVUs, byPayer[r.ID]...)
		out = append(out, payer)
	}
	return out, nil
}

func (s *service) Create(ctx context.Context, p auth.Principal, in CreateInput) (Payer, error) {
	name, err := validName(in.Name)
	if err != nil {
		return Payer{}, err
	}
	if err := validDoc(in.DocType, in.DocNumber); err != nil {
		return Payer{}, err
	}
	if err := s.usableAccount(ctx, p.OrgID, in.AccountID); err != nil {
		return Payer{}, err
	}
	row := store.InsertPayerParams{
		ID: ids.New(), OrgID: p.OrgID, AccountID: in.AccountID, Name: name,
		DocType: string(in.DocType), DocNumber: in.DocNumber, Email: trimmedEmail(in.Email),
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := store.New(tx).InsertPayer(ctx, row); err != nil {
			return fmt.Errorf("insert payer: %w", err)
		}
		return s.audit(ctx, tx, p, "payer.create", "payer", row.ID, "")
	})
	if err != nil {
		return Payer{}, err
	}
	return s.Get(ctx, p, row.ID)
}

// usableAccount checks the account belongs to the org (T6: 404 otherwise) and is open.
func (s *service) usableAccount(ctx context.Context, orgID, accountID uuid.UUID) error {
	acc, err := s.deps.Accounts.Get(ctx, orgID, accountID)
	if err != nil {
		return err
	}
	if acc.Archived {
		return apperr.New(apperr.CodeAccountArchived, nil)
	}
	return nil
}

func (s *service) Get(ctx context.Context, p auth.Principal, payerID uuid.UUID) (Payer, error) {
	q := store.New(s.deps.Pool)
	row, err := q.GetPayer(ctx, store.GetPayerParams{ID: payerID, OrgID: p.OrgID})
	if db.IsNoRows(err) {
		return Payer{}, apperr.New(apperr.CodePayerNotFound, nil)
	}
	if err != nil {
		return Payer{}, fmt.Errorf("get payer: %w", err)
	}
	payers, err := s.withCVUs(ctx, q, p.OrgID, []store.Payer{row})
	if err != nil {
		return Payer{}, err
	}
	return payers[0], nil
}

// Update merges the patch over the stored payer and validates the merged state (A4).
func (s *service) Update(ctx context.Context, p auth.Principal, payerID uuid.UUID, in UpdateInput) (Payer, error) {
	current, err := s.Get(ctx, p, payerID)
	if err != nil {
		return Payer{}, err
	}
	name, email := current.Name, current.Email
	if in.Name != nil {
		if name, err = validName(*in.Name); err != nil {
			return Payer{}, err
		}
	}
	switch {
	case in.ClearEmail:
		email = nil
	case in.Email != nil:
		email = trimmedEmail(in.Email)
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		n, err := store.New(tx).UpdatePayer(ctx, store.UpdatePayerParams{
			ID: payerID, OrgID: p.OrgID, Name: name, Email: email, UpdatedAt: s.deps.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("update payer: %w", err)
		}
		if n != 1 {
			return apperr.New(apperr.CodePayerNotFound, nil)
		}
		return s.audit(ctx, tx, p, "payer.update", "payer", payerID, "")
	})
	if err != nil {
		return Payer{}, err
	}
	return s.Get(ctx, p, payerID)
}

// ImportPayer is idempotent by (org, ERP ref): a second import returns the first payer.
func (s *service) ImportPayer(ctx context.Context, orgID uuid.UUID, in ImportInput) (Payer, error) {
	system := auth.Principal{OrgID: orgID}
	q := store.New(s.deps.Pool)
	if row, err := q.GetPayerByERPRef(ctx, store.GetPayerByERPRefParams{OrgID: orgID, ErpRef: &in.ERPRef}); err == nil {
		return s.Get(ctx, system, row.ID)
	} else if !db.IsNoRows(err) {
		return Payer{}, fmt.Errorf("payer by erp ref: %w", err)
	}
	name, err := validName(in.Name)
	if err != nil {
		return Payer{}, err
	}
	if err := validDoc(in.DocType, in.DocNumber); err != nil {
		return Payer{}, err
	}
	if in.ERPRef == "" || len(in.ERPRef) > 120 {
		return Payer{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "erpRef"})
	}
	if err := s.usableAccount(ctx, orgID, in.AccountID); err != nil {
		return Payer{}, err
	}
	id := ids.New()
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := store.New(tx).InsertPayer(ctx, store.InsertPayerParams{
			ID: id, OrgID: orgID, AccountID: in.AccountID, Name: name, DocType: string(in.DocType),
			DocNumber: in.DocNumber, Email: trimmedEmail(in.Email), ErpRef: &in.ERPRef,
		}); err != nil {
			return fmt.Errorf("insert imported payer: %w", err)
		}
		return s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &orgID, ActorKind: audit.ActorSystem, Action: "payer.import", TargetType: "payer", TargetID: &id,
		})
	})
	if db.IsUniqueViolation(err, "payers_erp_ref_key") { // a concurrent import won: return its payer
		row, getErr := q.GetPayerByERPRef(ctx, store.GetPayerByERPRefParams{OrgID: orgID, ErpRef: &in.ERPRef})
		if getErr != nil {
			return Payer{}, fmt.Errorf("payer by erp ref after conflict: %w", getErr)
		}
		id, err = row.ID, nil
	}
	if err != nil {
		return Payer{}, err
	}
	return s.Get(ctx, system, id)
}

func (s *service) audit(ctx context.Context, tx pgx.Tx, p auth.Principal, action, target string, id uuid.UUID, key string) error {
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &p.OrgID, ActorID: &p.UserID, ActorKind: audit.ActorUser, Action: action,
		TargetType: target, TargetID: &id, IdempotencyKey: key,
	})
}

func validName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := len([]rune(name)); n < nameMin || n > nameMax {
		return "", apperr.New(apperr.CodeValidation, map[string]any{"field": "name"})
	}
	return name, nil
}

func validDoc(t DocType, number string) error {
	if t != DocCUIT && t != DocCUIL && t != DocDNI {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "docType"})
	}
	if n := len(number); n < 7 || n > 11 || strings.Trim(number, "0123456789") != "" {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "docNumber"})
	}
	return nil
}

func trimmedEmail(email *string) *string {
	if email == nil {
		return nil
	}
	e := strings.TrimSpace(*email)
	if e == "" {
		return nil
	}
	return &e
}

func toPayer(r store.Payer) Payer {
	return Payer{
		ID: r.ID, OrgID: r.OrgID, AccountID: r.AccountID, Name: r.Name, DocType: DocType(r.DocType),
		DocNumber: r.DocNumber, Email: r.Email, ERPRef: r.ErpRef, CVUs: []CVU{}, CreatedAt: r.CreatedAt,
	}
}

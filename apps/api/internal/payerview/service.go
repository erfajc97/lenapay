package payerview

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/payerview/store"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Service is the "Mis pagos" API used by the handler and, as an approval hook, by kyc.
type Service interface {
	// OnKYCApproved links the payers matching a personal org's verified DNI (kyc.ApprovalHook).
	OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
	// ListCompanies links new matches, then lists the person's companies (hidden ones on request).
	ListCompanies(ctx context.Context, p auth.Principal, includeHidden bool) ([]Company, error)
	// SetHidden hides or shows one of the person's companies.
	SetHidden(ctx context.Context, p auth.Principal, linkID uuid.UUID, hidden bool) (Company, error)
	// ListPayins pages the person's payins to that company's CVUs, newest first.
	ListPayins(ctx context.Context, p auth.Principal, linkID uuid.UUID, req page.Request) (page.Page[Payin], error)
	// Installments returns the person's ERP plan at that company.
	Installments(ctx context.Context, p auth.Principal, linkID uuid.UUID) (Installments, error)
	// Pay creates a personal payout to the person's CVU at that company.
	Pay(ctx context.Context, p auth.Principal, linkID uuid.UUID, in PayInput) (payouts.Payout, error)
}

// Deps are the collaborators of the payer view.
type Deps struct {
	Pool    *pgxpool.Pool
	Clock   clock.Clock
	Audit   *audit.Logger
	ERP     ERP
	Payouts Payouts
}

type service struct {
	deps Deps
}

// NewService returns the payer view service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// person resolves the caller's verified person from the session (T3). Without an approved
// personal org carrying a verified DNI there is nothing to match: kyc.not_approved.
func (s *service) person(ctx context.Context, p auth.Principal) (person, error) {
	row, err := store.New(s.deps.Pool).GetPersonForUser(ctx, p.UserID)
	if db.IsNoRows(err) {
		return person{}, apperr.New(apperr.CodeKYCNotApproved, nil)
	}
	if err != nil {
		return person{}, fmt.Errorf("payerview: person: %w", err)
	}
	return person{orgID: row.ID, dni: row.Dni}, nil
}

// link loads one of the person's links; another person's link answers like a missing one (T2).
func (s *service) link(ctx context.Context, who person, linkID uuid.UUID) (store.ListLinksRow, error) {
	row, err := store.New(s.deps.Pool).GetLink(ctx, store.GetLinkParams{ID: linkID, PersonalOrgID: who.orgID})
	if db.IsNoRows(err) {
		return store.ListLinksRow{}, apperr.New(apperr.CodePayerLinkNotFound, nil)
	}
	if err != nil {
		return store.ListLinksRow{}, fmt.Errorf("payerview: link: %w", err)
	}
	return store.ListLinksRow(row), nil
}

func (s *service) OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	if orgKind != "personal" {
		return nil
	}
	dni, err := store.New(tx).GetVerifiedDNI(ctx, orgID)
	if db.IsNoRows(err) {
		return nil // approved without a verified DNI: nothing can match
	}
	if err != nil {
		return fmt.Errorf("payerview: verified dni: %w", err)
	}
	return s.linkMatches(ctx, tx, person{orgID: orgID, dni: dni})
}

// linkMatches stores a link for every company payer carrying the person's verified DNI that is
// not linked yet (the matching rule is the SQL of FindUnlinkedMatches, one place, G12).
func (s *service) linkMatches(ctx context.Context, tx pgx.Tx, who person) error {
	key, ok := dni8(who.dni)
	if !ok {
		return nil
	}
	q := store.New(tx)
	matches, err := q.FindUnlinkedMatches(ctx, store.FindUnlinkedMatchesParams{Dni8: key, PersonalOrgID: who.orgID})
	if err != nil {
		return fmt.Errorf("payerview: find matches: %w", err)
	}
	now := s.deps.Clock.Now()
	for _, m := range matches {
		id := ids.New()
		if err := q.InsertPayerLink(ctx, store.InsertPayerLinkParams{
			ID: id, PersonalOrgID: who.orgID, PayerID: m.ID, OrgID: m.OrgID, MatchedDocType: m.DocType, MatchedAt: now,
		}); err != nil {
			return fmt.Errorf("payerview: insert link: %w", err)
		}
		if err := s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &who.orgID, ActorKind: audit.ActorSystem, Action: "payer_link.create", TargetType: "payer_link", TargetID: &id,
			Payload: map[string]string{"payerId": m.ID.String(), "orgId": m.OrgID.String(), "matchedDocType": m.DocType},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) ListCompanies(ctx context.Context, p auth.Principal, includeHidden bool) ([]Company, error) {
	who, err := s.person(ctx, p)
	if err != nil {
		return nil, err
	}
	// Lazy matching: payers created after the KYC approval are linked on the next read.
	if err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error { return s.linkMatches(ctx, tx, who) }); err != nil {
		return nil, err
	}
	rows, err := store.New(s.deps.Pool).ListLinks(ctx, store.ListLinksParams{PersonalOrgID: who.orgID, IncludeHidden: includeHidden})
	if err != nil {
		return nil, fmt.Errorf("payerview: list links: %w", err)
	}
	return s.companies(ctx, rows)
}

func (s *service) SetHidden(ctx context.Context, p auth.Principal, linkID uuid.UUID, hidden bool) (Company, error) {
	who, err := s.person(ctx, p)
	if err != nil {
		return Company{}, err
	}
	status := statusActive
	if hidden {
		status = statusHidden
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		n, err := store.New(tx).SetLinkStatus(ctx, store.SetLinkStatusParams{
			Status: status, UpdatedAt: s.deps.Clock.Now(), ID: linkID, PersonalOrgID: who.orgID,
		})
		if err != nil {
			return fmt.Errorf("payerview: set status: %w", err)
		}
		if n != 1 {
			return apperr.New(apperr.CodePayerLinkNotFound, nil)
		}
		return s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &who.orgID, ActorID: &p.UserID, ActorKind: audit.ActorUser, Action: "payer_link." + status,
			TargetType: "payer_link", TargetID: &linkID,
		})
	})
	if err != nil {
		return Company{}, err
	}
	row, err := s.link(ctx, who, linkID)
	if err != nil {
		return Company{}, err
	}
	out, err := s.companies(ctx, []store.ListLinksRow{row})
	if err != nil {
		return Company{}, err
	}
	return out[0], nil
}

// companies assembles what the person may see of each linked company: name, project, their own
// active CVUs, their credited totals and the ERP flag. Nothing else of the company is returned.
func (s *service) companies(ctx context.Context, rows []store.ListLinksRow) ([]Company, error) {
	out := make([]Company, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	payerIDs, orgIDs := make([]uuid.UUID, 0, len(rows)), make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		payerIDs = append(payerIDs, r.PayerID)
		if !slices.Contains(orgIDs, r.OrgID) {
			orgIDs = append(orgIDs, r.OrgID)
		}
	}
	q := store.New(s.deps.Pool)
	cvus, err := q.ListActiveCVUsForPayers(ctx, payerIDs)
	if err != nil {
		return nil, fmt.Errorf("payerview: cvus: %w", err)
	}
	totals, err := q.PayinTotalsForPayers(ctx, payerIDs)
	if err != nil {
		return nil, fmt.Errorf("payerview: totals: %w", err)
	}
	linked, err := s.deps.ERP.LinkedOrgs(ctx, orgIDs)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		c := Company{
			LinkID: r.ID, CompanyName: r.CompanyName, CVUs: []CVU{}, TotalPaid: []money.Amount{},
			ERPLinked: linked[r.OrgID], Hidden: r.Status == statusHidden,
		}
		if r.AccountKind == "project" {
			name := r.AccountName
			c.ProjectName = &name
		}
		for _, cv := range cvus {
			if cv.PayerID != nil && *cv.PayerID == r.PayerID {
				c.CVUs = append(c.CVUs, CVU{Currency: money.Currency(cv.Currency), Number: cv.Cvu, Alias: cv.Alias})
			}
		}
		for _, t := range totals {
			if t.PayerID == nil || *t.PayerID != r.PayerID {
				continue
			}
			c.TotalPaid = append(c.TotalPaid, money.New(t.TotalMinor, money.Currency(t.Currency)))
			if last := t.LastAt; c.LastPaymentAt == nil || last.After(*c.LastPaymentAt) {
				c.LastPaymentAt = &last
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *service) ListPayins(ctx context.Context, p auth.Principal, linkID uuid.UUID, req page.Request) (page.Page[Payin], error) {
	who, err := s.person(ctx, p)
	if err != nil {
		return page.Page[Payin]{}, err
	}
	l, err := s.link(ctx, who, linkID)
	if err != nil {
		return page.Page[Payin]{}, err
	}
	params := store.ListPayinsForPayerParams{PayerID: &l.PayerID, OrgID: l.OrgID, PageLimit: req.FetchLimit()}
	if req.After != nil {
		params.CursorCreatedAt, params.CursorID = &req.After.CreatedAt, &req.After.ID
	}
	rows, err := store.New(s.deps.Pool).ListPayinsForPayer(ctx, params)
	if err != nil {
		return page.Page[Payin]{}, fmt.Errorf("payerview: payins: %w", err)
	}
	items := make([]Payin, 0, len(rows))
	for _, r := range rows {
		items = append(items, Payin{
			ID: r.ID, CVU: r.Cvu, Amount: money.New(r.GrossMinor, money.Currency(r.Currency)), Status: r.Status,
			CreatedAt: r.CreatedAt, CreditedAt: r.CreditedAt,
		})
	}
	return page.Build(items, req.Limit, func(pi Payin) page.Position { return page.Position{CreatedAt: pi.CreatedAt, ID: pi.ID} }), nil
}

func (s *service) Installments(ctx context.Context, p auth.Principal, linkID uuid.UUID) (Installments, error) {
	who, err := s.person(ctx, p)
	if err != nil {
		return Installments{}, err
	}
	l, err := s.link(ctx, who, linkID)
	if err != nil {
		return Installments{}, err
	}
	linked, plan, err := s.deps.ERP.BuyerInstallments(ctx, l.OrgID, l.ErpRef)
	if err != nil {
		return Installments{}, err
	}
	return Installments{ERPLinked: linked, Items: plan}, nil
}

// installmentConcept is the transfer concept in the business language (es-AR): it reaches the
// company's bank statement, so it is data, not app copy.
const installmentConcept = "Cuota %d - %s"

func (s *service) Pay(ctx context.Context, p auth.Principal, linkID uuid.UUID, in PayInput) (payouts.Payout, error) {
	who, err := s.person(ctx, p)
	if err != nil {
		return payouts.Payout{}, err
	}
	l, err := s.link(ctx, who, linkID)
	if err != nil {
		return payouts.Payout{}, err
	}
	q := store.New(s.deps.Pool)
	cvu, err := q.GetActivePayerCVU(ctx, store.GetActivePayerCVUParams{PayerID: &l.PayerID, OrgID: l.OrgID, Currency: string(in.Amount.Currency)})
	if db.IsNoRows(err) {
		return payouts.Payout{}, apperr.New(apperr.CodeCurrencyMismatch, map[string]any{"currency": string(in.Amount.Currency)})
	}
	if err != nil {
		return payouts.Payout{}, fmt.Errorf("payerview: cvu: %w", err)
	}
	concept, err := s.concept(ctx, l, in)
	if err != nil {
		return payouts.Payout{}, err
	}
	accountID, err := q.GetPersonalAccountID(ctx, &who.orgID)
	if db.IsNoRows(err) {
		return payouts.Payout{}, apperr.New(apperr.CodeAccountNotFound, nil)
	}
	if err != nil {
		return payouts.Payout{}, fmt.Errorf("payerview: personal account: %w", err)
	}
	// The payout belongs to the personal org the caller owns (resolved above, never from input).
	owner := auth.Principal{
		UserID: p.UserID, SessionID: p.SessionID, OrgID: who.orgID, Role: auth.RoleAdmin, KYCStatus: auth.KYCApproved, StepUp: p.StepUp,
	}
	return s.deps.Payouts.Create(ctx, owner, payouts.CreateInput{
		AccountID: accountID, Amount: in.Amount, To: cvu, Concept: concept, IdempotencyKey: in.IdempotencyKey,
	})
}

// concept is what the person typed (or the company name), prefixed with the installment it
// pays, which must be one of the person's installments in the amount's currency.
func (s *service) concept(ctx context.Context, l store.ListLinksRow, in PayInput) (string, error) {
	base := l.CompanyName
	if in.Concept != nil {
		typed := strings.TrimSpace(*in.Concept)
		if n := len([]rune(typed)); n < 1 || n > maxUserConcept {
			return "", apperr.New(apperr.CodeValidation, map[string]any{"field": "concept"})
		}
		base = typed
	}
	if in.InstallmentNumber == nil {
		return truncate(base, maxConcept), nil
	}
	n := *in.InstallmentNumber
	if n < 1 || n > maxInstallment {
		return "", apperr.New(apperr.CodeValidation, map[string]any{"field": "installmentNumber"})
	}
	_, plan, err := s.deps.ERP.BuyerInstallments(ctx, l.OrgID, l.ErpRef)
	if err != nil {
		return "", err
	}
	i := slices.IndexFunc(plan, func(x erplink.Installment) bool { return x.Number == n })
	if i < 0 {
		return "", apperr.New(apperr.CodeInstallmentNotFound, map[string]any{"installmentNumber": n})
	}
	if plan[i].Amount.Currency != in.Amount.Currency {
		return "", apperr.New(apperr.CodeCurrencyMismatch, map[string]any{"currency": string(in.Amount.Currency)})
	}
	return truncate(fmt.Sprintf(installmentConcept, n, base), maxConcept), nil
}

func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}

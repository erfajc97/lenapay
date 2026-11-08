package erplink

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/erplink/store"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	applog "github.com/erfajc97/lenapay/apps/api/internal/platform/log"
)

// Service is the ERP link API used by the handler and the worker.
type Service interface {
	// Candidate looks the org's CUIT up in the identity index (R-ERP-1).
	Candidate(ctx context.Context, p auth.Principal) (Candidate, error)
	// Link checks the ERP credentials, stores the link and queues the import (R-ERP-2).
	Link(ctx context.Context, p auth.Principal, c Credentials) (Link, error)
	// Get returns the org's link (erp_link.not_found when none).
	Get(ctx context.Context, p auth.Principal) (Link, error)
	// Import is the erp_import job: projects → accounts, buyers → payers (R-ERP-3).
	Import(ctx context.Context, orgID uuid.UUID) error
	// LoadIdentityIndex refreshes erp_identity_index from the Directory (periodic job).
	LoadIdentityIndex(ctx context.Context) error
	// LinkedOrgs reports which of orgIDs are linked to the ERP (system scope: the caller already
	// decided the person may see these orgs; payerview).
	LinkedOrgs(ctx context.Context, orgIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	// BuyerInstallments returns the plan of the ERP buyer behind a payer of orgID. linked is false
	// (and the plan empty) when orgID has no ERP link; a payer without an ERP ref has no plan.
	BuyerInstallments(ctx context.Context, orgID uuid.UUID, buyerRef *string) (linked bool, plan []Installment, err error)
}

// Deps are the collaborators of the ERP link service.
type Deps struct {
	Pool      *pgxpool.Pool
	Clock     clock.Clock
	Audit     *audit.Logger
	Jobs      jobs.Enqueuer
	Directory Directory
	Orgs      Orgs
	Accounts  ProjectImporter
	Payers    PayerImporter
}

type service struct {
	deps Deps
}

// NewService returns the ERP link service.
func NewService(deps Deps) Service { return &service{deps: deps} }

const docTypeCUIT = "cuit"

// Unique constraints of erp_links (migration 00007): one link per org and per ERP org.
const (
	linkOrgKey    = "erp_links_org_id_key"
	linkERPOrgKey = "erp_links_erp_org_id_key"
)

// requireAdmin is the service-side copy of the route rule (admin + approved KYC, R-ERP-2).
func requireAdmin(p auth.Principal) error {
	if !p.HasOrg() {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if !p.Role.AtLeast(auth.RoleAdmin) {
		return apperr.New(apperr.CodeForbidden, nil)
	}
	if p.KYCStatus != auth.KYCApproved {
		return apperr.New(apperr.CodeKYCNotApproved, nil)
	}
	return nil
}

func (s *service) Candidate(ctx context.Context, p auth.Principal) (Candidate, error) {
	if err := requireAdmin(p); err != nil {
		return Candidate{}, err
	}
	entry, found, err := s.lookup(ctx, p.OrgID)
	if err != nil || !found {
		return Candidate{}, err
	}
	return Candidate{Found: true, ERPOrgID: &entry.ErpOrgID, DisplayName: &entry.DisplayName}, nil
}

// lookup reads the index by the org's CUIT (primary key: one index probe).
func (s *service) lookup(ctx context.Context, orgID uuid.UUID) (store.ErpIdentityIndex, bool, error) {
	cuit, err := s.deps.Orgs.OrgCUIT(ctx, orgID)
	if err != nil {
		return store.ErpIdentityIndex{}, false, fmt.Errorf("erplink: org cuit: %w", err)
	}
	cuit = digits(cuit)
	if cuit == "" {
		return store.ErpIdentityIndex{}, false, nil
	}
	entry, err := store.New(s.deps.Pool).GetIdentity(ctx, store.GetIdentityParams{DocType: docTypeCUIT, DocNumber: cuit})
	if db.IsNoRows(err) {
		return store.ErpIdentityIndex{}, false, nil
	}
	if err != nil {
		return store.ErpIdentityIndex{}, false, fmt.Errorf("erplink: identity: %w", err)
	}
	return entry, true, nil
}

func (s *service) Link(ctx context.Context, p auth.Principal, c Credentials) (Link, error) {
	if err := requireAdmin(p); err != nil {
		return Link{}, err
	}
	username := strings.TrimSpace(c.Username)
	if username == "" || len(username) > maxUsername || c.Password.Reveal() == "" || len(c.Password.Reveal()) > maxPassword {
		return Link{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "credentials"})
	}
	if _, err := store.New(s.deps.Pool).GetErpLinkByOrg(ctx, p.OrgID); err == nil {
		return Link{}, apperr.New(apperr.CodeERPAlreadyLinked, nil)
	} else if !db.IsNoRows(err) {
		return Link{}, fmt.Errorf("erplink: get link: %w", err)
	}
	entry, found, err := s.lookup(ctx, p.OrgID)
	if err != nil {
		return Link{}, err
	}
	if !found {
		return Link{}, apperr.New(apperr.CodeERPNoCandidate, nil)
	}
	// The extra confirmation (R-ERP-2): the ERP itself vouches for this person. Called before
	// the transaction: no network call inside it (AG11).
	if err := s.deps.Directory.VerifyCredentials(ctx, entry.ErpOrgID, username, c.Password); err != nil {
		return Link{}, err
	}
	var out Link
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		id := ids.New()
		if err := q.InsertErpLink(ctx, store.InsertErpLinkParams{
			ID: id, OrgID: p.OrgID, ErpOrgID: entry.ErpOrgID, DisplayName: entry.DisplayName, LinkedBy: p.UserID, LinkedAt: s.deps.Clock.Now(),
		}); err != nil {
			if db.IsUniqueViolation(err, linkOrgKey) || db.IsUniqueViolation(err, linkERPOrgKey) {
				return apperr.Wrap(apperr.CodeERPAlreadyLinked, err, nil)
			}
			return fmt.Errorf("erplink: insert link: %w", err)
		}
		if _, err := s.deps.Jobs.InsertTx(ctx, tx, ImportArgs{OrgID: p.OrgID, RequestID: applog.RequestID(ctx)}, nil); err != nil {
			return fmt.Errorf("erplink: enqueue import: %w", err)
		}
		if err := s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &p.OrgID, ActorID: &p.UserID, ActorKind: audit.ActorUser, Action: "erp_link.create",
			TargetType: "erp_link", TargetID: &id, Payload: map[string]string{"erpOrgId": entry.ErpOrgID},
		}); err != nil {
			return err
		}
		row, err := q.GetErpLinkByOrg(ctx, p.OrgID)
		if err != nil {
			return fmt.Errorf("erplink: read link: %w", err)
		}
		out = fromRow(row)
		return nil
	})
	return out, err
}

func (s *service) Get(ctx context.Context, p auth.Principal) (Link, error) {
	if !p.HasOrg() {
		return Link{}, apperr.New(apperr.CodeOrgNotFound, nil)
	}
	row, err := store.New(s.deps.Pool).GetErpLinkByOrg(ctx, p.OrgID)
	if db.IsNoRows(err) {
		return Link{}, apperr.New(apperr.CodeERPLinkNotFound, nil)
	}
	if err != nil {
		return Link{}, fmt.Errorf("erplink: get link: %w", err)
	}
	return fromRow(row), nil
}

// importSummary is the snapshot stored on the link after an import (counts and refs only).
type importSummary struct {
	Projects      int    `json:"projects"`
	Buyers        int    `json:"buyers"`
	SkippedBuyers int    `json:"skippedBuyers"`
	Error         string `json:"error,omitempty"`
}

func (s *service) Import(ctx context.Context, orgID uuid.UUID) error {
	q := store.New(s.deps.Pool)
	link, err := q.GetErpLinkByOrg(ctx, orgID)
	if db.IsNoRows(err) {
		return apperr.New(apperr.CodeERPLinkNotFound, map[string]any{"orgId": orgID.String()})
	}
	if err != nil {
		return fmt.Errorf("erplink: import link: %w", err)
	}
	if ImportStatus(link.ImportStatus) == ImportCompleted {
		return nil // the snapshot was already copied (River delivers at least once)
	}
	if err := s.setStatus(ctx, orgID, ImportRunning, importSummary{}); err != nil {
		return err
	}
	summary, err := s.copySnapshot(ctx, orgID, link.ErpOrgID)
	if err != nil {
		summary.Error = string(apperr.CodeOf(err))
		if serr := s.setStatus(ctx, orgID, ImportFailed, summary); serr != nil {
			return fmt.Errorf("erplink: import: %w (and recording the failure: %w)", err, serr)
		}
		return err // River retries; the importers are idempotent by ERP ref
	}
	return s.setStatus(ctx, orgID, ImportCompleted, summary)
}

// copySnapshot imports projects as project accounts, then each buyer under its project's
// account. A buyer of an unknown project is skipped (counted), never guessed.
func (s *service) copySnapshot(ctx context.Context, orgID uuid.UUID, erpOrgID string) (importSummary, error) {
	var sum importSummary
	projects, err := s.deps.Directory.Projects(ctx, erpOrgID)
	if err != nil {
		return sum, fmt.Errorf("erplink: projects: %w", err)
	}
	accountOf := make(map[string]uuid.UUID, len(projects))
	for _, pr := range projects {
		acct, err := s.deps.Accounts.ImportProject(ctx, orgID, pr.Name, pr.Ref)
		if err != nil {
			return sum, fmt.Errorf("erplink: import project %s: %w", pr.Ref, err)
		}
		accountOf[pr.Ref] = acct.ID
		sum.Projects++
	}
	buyers, err := s.deps.Directory.Buyers(ctx, erpOrgID)
	if err != nil {
		return sum, fmt.Errorf("erplink: buyers: %w", err)
	}
	for _, b := range buyers {
		accountID, ok := accountOf[b.ProjectRef]
		if !ok {
			sum.SkippedBuyers++
			continue
		}
		if _, err := s.deps.Payers.ImportPayer(ctx, orgID, payers.ImportInput{
			AccountID: accountID, Name: b.Name, DocType: payers.DocType(b.DocType), DocNumber: digits(b.DocNumber),
			Email: b.Email, ERPRef: b.Ref,
		}); err != nil {
			return sum, fmt.Errorf("erplink: import buyer %s: %w", b.Ref, err)
		}
		sum.Buyers++
	}
	return sum, nil
}

func (s *service) setStatus(ctx context.Context, orgID uuid.UUID, st ImportStatus, sum importSummary) error {
	snapshot, err := json.Marshal(sum)
	if err != nil {
		return fmt.Errorf("erplink: snapshot: %w", err)
	}
	params := store.SetErpImportStatusParams{ImportStatus: string(st), Snapshot: snapshot, OrgID: orgID}
	if st == ImportCompleted {
		now := s.deps.Clock.Now()
		params.ImportedAt = &now
	}
	if err := store.New(s.deps.Pool).SetErpImportStatus(ctx, params); err != nil {
		return fmt.Errorf("erplink: import status: %w", err)
	}
	return nil
}

func (s *service) LoadIdentityIndex(ctx context.Context) error {
	entries, err := s.deps.Directory.IdentityIndex(ctx)
	if err != nil {
		return fmt.Errorf("erplink: identity index: %w", err)
	}
	now := s.deps.Clock.Now()
	return db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		for _, e := range entries {
			if err := q.UpsertIdentity(ctx, store.UpsertIdentityParams{
				DocType: e.DocType, DocNumber: digits(e.DocNumber), ErpOrgID: e.ERPOrgID, ErpClientID: e.ERPClientID,
				DisplayName: e.DisplayName, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("erplink: upsert identity %s: %w", e.ERPOrgID, err)
			}
		}
		return nil
	})
}

func (s *service) LinkedOrgs(ctx context.Context, orgIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	linked := make(map[uuid.UUID]bool, len(orgIDs))
	if len(orgIDs) == 0 {
		return linked, nil
	}
	rows, err := store.New(s.deps.Pool).ListLinkedOrgIDs(ctx, orgIDs)
	if err != nil {
		return nil, fmt.Errorf("erplink: linked orgs: %w", err)
	}
	for _, id := range rows {
		linked[id] = true
	}
	return linked, nil
}

func (s *service) BuyerInstallments(ctx context.Context, orgID uuid.UUID, buyerRef *string) (bool, []Installment, error) {
	link, err := store.New(s.deps.Pool).GetErpLinkByOrg(ctx, orgID)
	if db.IsNoRows(err) {
		return false, []Installment{}, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("erplink: get link: %w", err)
	}
	if buyerRef == nil {
		return true, []Installment{}, nil
	}
	plan, err := s.deps.Directory.Installments(ctx, link.ErpOrgID, *buyerRef)
	if err != nil {
		return true, nil, fmt.Errorf("erplink: installments: %w", err)
	}
	return true, plan, nil
}

// digits keeps the digits of a document ("30-71234567-8" → "30712345678").
func digits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func fromRow(r store.ErpLink) Link {
	return Link{
		ID: r.ID, ERPOrgID: r.ErpOrgID, DisplayName: r.DisplayName, LinkedBy: r.LinkedBy, LinkedAt: r.LinkedAt,
		ImportStatus: ImportStatus(r.ImportStatus), ImportedAt: r.ImportedAt,
	}
}

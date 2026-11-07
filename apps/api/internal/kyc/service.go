package kyc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/erfajc97/lenapay/apps/api/internal/kyc/store"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	applog "github.com/erfajc97/lenapay/apps/api/internal/platform/log"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/upload"
)

// Service is the KYC API used by the handler, the review job and orgs.
type Service interface {
	// Get returns the case of p.OrgID.
	Get(ctx context.Context, p auth.Principal) (Case, error)
	// Update replaces the case data while draft or rejected.
	Update(ctx context.Context, p auth.Principal, in UpdateInput) (Case, error)
	// UploadDocument stores a capped, type-sniffed file privately (A13).
	UploadDocument(ctx context.Context, p auth.Principal, in DocumentUpload) (Document, error)
	// Submit moves a complete case to submitted and runs the vendor check.
	Submit(ctx context.Context, p auth.Principal) (Case, error)
	// Decide records the ops decision (internal API key route); approval runs the hooks.
	Decide(ctx context.Context, caseID uuid.UUID, d Decision) (Case, error)

	// OpenCase creates the draft case of a new org inside its creation transaction (orgs.KYCCases).
	OpenCase(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error
	// Review is the kyc_review job: the vendor decides a submitted person case; a submitted
	// company case is auto-approved only when AutoApproveAfter is set (development).
	Review(ctx context.Context, caseID uuid.UUID) error
}

// Deps are the collaborators of the KYC service.
type Deps struct {
	Pool      *pgxpool.Pool
	Clock     clock.Clock
	Audit     *audit.Logger
	Jobs      jobs.Enqueuer
	Provider  Provider
	Documents DocumentStore
	Orgs      OrgStatus
	OnApprove []ApprovalHook
	// AutoApproveAfter > 0 auto-approves submitted company cases after that delay
	// (DEV_KYC_AUTO_APPROVE_SECONDS; config.DevKYCAutoApproveDelay is 0 in production).
	AutoApproveAfter time.Duration
}

// Limits and timings.
const (
	// MaxDocumentBytes is the contract's 10 MB per document.
	MaxDocumentBytes = upload.MaxBytes
	// personReviewBackstop re-runs the vendor check when the synchronous one at submit failed.
	personReviewBackstop = 30 * time.Second
	// reasonDevAutoApprove is the decision reason of the development auto-approval.
	reasonDevAutoApprove = "dev_auto_approve"
)

type service struct {
	deps Deps
}

// NewService returns the KYC service.
func NewService(deps Deps) Service { return &service{deps: deps} }

func (s *service) now() time.Time { return s.deps.Clock.Now() }

func (s *service) OpenCase(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	kind, ok := caseKindFor(orgKind)
	if !ok {
		return fmt.Errorf("kyc: unknown org kind %q", orgKind)
	}
	return store.New(tx).InsertKYCCase(ctx, store.InsertKYCCaseParams{ID: ids.New(), OrgID: orgID, Kind: kind, CreatedAt: s.now()})
}

func (s *service) Get(ctx context.Context, p auth.Principal) (Case, error) {
	q := store.New(s.deps.Pool)
	row, err := q.GetKYCCaseByOrg(ctx, p.OrgID)
	if err != nil {
		return Case{}, notFoundAs(err, apperr.CodeKYCCaseNotFound)
	}
	return loadCase(ctx, q, row)
}

// loadCase builds the domain case from its row and documents (the storage key never leaves).
func loadCase(ctx context.Context, q *store.Queries, row store.KycCase) (Case, error) {
	var data caseData
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return Case{}, fmt.Errorf("kyc: decode case data: %w", err)
	}
	rows, err := q.ListKYCDocuments(ctx, store.ListKYCDocumentsParams{CaseID: row.ID, OrgID: row.OrgID})
	if err != nil {
		return Case{}, err
	}
	docs := make([]Document, 0, len(rows))
	for _, d := range rows {
		docs = append(docs, Document{
			ID: d.ID, Kind: d.Kind, RepresentativeDNI: d.RepresentativeDni, FileName: d.FileName,
			ContentType: d.ContentType, SizeBytes: d.SizeBytes, CreatedAt: d.CreatedAt,
		})
	}
	return Case{
		ID: row.ID, OrgID: row.OrgID, Kind: row.Kind, Status: Status(row.Status), Company: data.Company, Person: data.Person,
		Documents: docs, DecisionReason: row.DecisionReason, SubmittedAt: row.SubmittedAt, DecidedAt: row.DecidedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *service) Update(ctx context.Context, p auth.Principal, in UpdateInput) (Case, error) {
	var out Case
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := s.editableCase(ctx, q, p.OrgID, StatusDraft)
		if err != nil {
			return err
		}
		data, err := dataFor(row.Kind, in)
		if err != nil {
			return err
		}
		if n, err := q.UpdateKYCCaseData(ctx, store.UpdateKYCCaseDataParams{ID: row.ID, OrgID: p.OrgID, Data: data, UpdatedAt: s.now()}); err != nil || n != 1 {
			return stateConflict(err, n)
		}
		if row.Status != string(StatusDraft) {
			if err := s.deps.Orgs.SetKYCStatus(ctx, tx, p.OrgID, string(StatusDraft)); err != nil {
				return err
			}
		}
		if err := s.record(ctx, tx, p, row.ID, "kyc.update"); err != nil {
			return err
		}
		row, err = q.GetKYCCase(ctx, row.ID)
		if err != nil {
			return err
		}
		out, err = loadCase(ctx, q, row)
		return err
	})
	return out, err
}

// editableCase locks the org's case and checks it may move to `to` (T1: by org).
func (s *service) editableCase(ctx context.Context, q *store.Queries, orgID uuid.UUID, to Status) (store.KycCase, error) {
	row, err := q.GetKYCCaseByOrgForUpdate(ctx, orgID)
	if err != nil {
		return store.KycCase{}, notFoundAs(err, apperr.CodeKYCCaseNotFound)
	}
	if !transitionAllowed(Status(row.Status), to) {
		return store.KycCase{}, apperr.New(apperr.CodeKYCInvalidState, map[string]any{"status": row.Status})
	}
	return row, nil
}

// dataFor accepts exactly the data block of the case's kind.
func dataFor(kind string, in UpdateInput) ([]byte, error) {
	switch {
	case kind == KindCompany && in.Company != nil && in.Person == nil:
	case kind == KindPerson && in.Person != nil && in.Company == nil:
	case kind == KindCompany:
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "/company"})
	default:
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "/person"})
	}
	return json.Marshal(caseData(in))
}

// stateConflict maps a conditional update that changed no row to kyc.invalid_state.
func stateConflict(err error, rows int64) error {
	if err != nil {
		return err
	}
	if rows != 1 {
		return apperr.New(apperr.CodeKYCInvalidState, nil)
	}
	return nil
}

func (s *service) record(ctx context.Context, tx pgx.Tx, p auth.Principal, caseID uuid.UUID, action string) error {
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &p.OrgID, ActorID: &p.UserID, ActorKind: audit.ActorUser, Action: action, TargetType: "kyc_case", TargetID: &caseID,
	})
}

func (s *service) UploadDocument(ctx context.Context, p auth.Principal, in DocumentUpload) (Document, error) {
	if !validDocKind(in.Kind) {
		return Document{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/kind"})
	}
	content, err := io.ReadAll(io.LimitReader(in.Content, MaxDocumentBytes+1))
	if err != nil {
		return Document{}, err
	}
	if len(content) > MaxDocumentBytes {
		return Document{}, apperr.New(apperr.CodeBodyTooLarge, nil)
	}
	contentType := upload.Sniff(content)
	if contentType == "" {
		return Document{}, apperr.New(apperr.CodeKYCDocumentInvalid, nil)
	}
	sum := upload.SHA256(content)
	doc := Document{
		ID: ids.New(), Kind: in.Kind, RepresentativeDNI: in.RepresentativeDNI, FileName: upload.CleanFileName(in.FileName),
		ContentType: contentType, SizeBytes: int64(len(content)), CreatedAt: s.now(),
	}
	key := "org/" + p.OrgID.String() + "/kyc/" + doc.ID.String() // T9: the tenant is the owner segment
	// Stored before the transaction (no network call inside it, AG11); an object whose row
	// never commits is unreachable, because only rows point at objects.
	if err := s.deps.Documents.Put(ctx, key, contentType, bytes.NewReader(content)); err != nil {
		return Document{}, err
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := s.editableCase(ctx, q, p.OrgID, StatusDraft)
		if err != nil {
			return err
		}
		if err := checkRepresentative(row.Kind, in); err != nil {
			return err
		}
		if err := q.InsertKYCDocument(ctx, store.InsertKYCDocumentParams{
			ID: doc.ID, OrgID: p.OrgID, CaseID: row.ID, Kind: doc.Kind, RepresentativeDni: doc.RepresentativeDNI,
			FileName: doc.FileName, ContentType: doc.ContentType, SizeBytes: doc.SizeBytes, Sha256: sum, StorageKey: key,
			CreatedAt: doc.CreatedAt,
		}); err != nil {
			return err
		}
		return s.record(ctx, tx, p, row.ID, "kyc.document_upload")
	})
	if err != nil {
		return Document{}, err
	}
	return doc, nil
}

// checkRepresentative: on a company case, identity documents belong to a representative
// (representativeDni required); a person case has no representatives.
func checkRepresentative(caseKind string, in DocumentUpload) error {
	isIdentity := in.Kind == DocDNIFront || in.Kind == DocDNIBack || in.Kind == DocSelfie
	switch {
	case caseKind == KindPerson && in.RepresentativeDNI != nil,
		caseKind == KindCompany && isIdentity && in.RepresentativeDNI == nil,
		caseKind == KindCompany && !isIdentity && in.RepresentativeDNI != nil:
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/representativeDni"})
	}
	return nil
}

func (s *service) Submit(ctx context.Context, p auth.Principal) (Case, error) {
	var caseID uuid.UUID
	var kind string
	var submittedAt time.Time
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := s.editableCase(ctx, q, p.OrgID, StatusSubmitted)
		if err != nil {
			return err
		}
		c, err := loadCase(ctx, q, row)
		if err != nil {
			return err
		}
		if missing := missingItems(row.Kind, caseData{Company: c.Company, Person: c.Person}, c.Documents); len(missing) > 0 {
			return apperr.New(apperr.CodeKYCIncomplete, map[string]any{"missing": missing})
		}
		caseID, kind, submittedAt = row.ID, row.Kind, s.now()
		if n, err := q.SubmitKYCCase(ctx, store.SubmitKYCCaseParams{ID: row.ID, OrgID: p.OrgID, SubmittedAt: &submittedAt}); err != nil || n != 1 {
			return stateConflict(err, n)
		}
		if err := s.deps.Orgs.SetKYCStatus(ctx, tx, p.OrgID, string(StatusSubmitted)); err != nil {
			return err
		}
		if err := s.enqueueReview(ctx, tx, row.ID, row.Kind, submittedAt); err != nil {
			return err
		}
		return s.record(ctx, tx, p, row.ID, "kyc.submit")
	})
	if err != nil {
		return Case{}, err
	}
	if kind == KindPerson {
		// The vendor answers persons right away (R-KYC-2); if it fails, the case stays
		// submitted and the review job asks again (uncertain is not "rejected").
		if err := s.Review(ctx, caseID); err != nil {
			slog.ErrorContext(ctx, "kyc vendor check failed; the review job retries", "case_id", caseID.String(), "err", err.Error())
		}
	}
	return s.Get(ctx, p)
}

// enqueueReview queues the kyc_review job in the submit transaction (AG13): always for a
// person (backstop of the synchronous check), for a company only with dev auto-approval.
func (s *service) enqueueReview(ctx context.Context, tx pgx.Tx, caseID uuid.UUID, kind string, submittedAt time.Time) error {
	delay := personReviewBackstop
	if kind == KindCompany {
		if s.deps.AutoApproveAfter <= 0 {
			return nil
		}
		delay = s.deps.AutoApproveAfter
	}
	args := ReviewArgs{CaseID: caseID, SubmittedAt: submittedAt, RequestID: applog.RequestID(ctx)}
	_, err := s.deps.Jobs.InsertTx(ctx, tx, args, &river.InsertOpts{ScheduledAt: submittedAt.Add(delay)})
	if err != nil {
		return fmt.Errorf("kyc: enqueue review: %w", err)
	}
	return nil
}

func (s *service) Review(ctx context.Context, caseID uuid.UUID) error {
	q := store.New(s.deps.Pool)
	row, err := q.GetKYCCase(ctx, caseID)
	if db.IsNoRows(err) {
		return nil // the org was deleted: nothing left to review
	}
	if err != nil {
		return err
	}
	if Status(row.Status) != StatusSubmitted {
		return nil // already decided (idempotent: River delivers at least once)
	}
	verdict, err := s.vendorVerdict(ctx, q, row)
	if err != nil || verdict == nil {
		return err
	}
	_, err = s.decide(ctx, caseID, decision{
		Decision: Decision{Approved: verdict.Approved, Reason: nonEmpty(verdict.Reason)}, providerRef: nonEmpty(verdict.ProviderRef),
		actor: audit.ActorSystem,
	})
	if apperr.IsCode(err, apperr.CodeKYCInvalidState) {
		return nil // decided concurrently (ops or the synchronous check)
	}
	return err
}

// vendorVerdict asks the vendor about a submitted case; nil means "no automatic decision"
// (a company case without dev auto-approval waits for ops).
func (s *service) vendorVerdict(ctx context.Context, q *store.Queries, row store.KycCase) (*Verdict, error) {
	c, err := loadCase(ctx, q, row)
	if err != nil {
		return nil, err
	}
	ref := row.ID.String() + ":" + submissionStamp(row.SubmittedAt)
	if row.Kind == KindPerson && c.Person != nil {
		v, err := s.deps.Provider.VerifyPerson(ctx, PersonCheck{
			Reference: ref, DNI: c.Person.DNI, FullName: c.Person.FullName, SelfieKey: selfieKey(row.OrgID, c.Documents),
		})
		return &v, err
	}
	if row.Kind == KindCompany && c.Company != nil && s.deps.AutoApproveAfter > 0 {
		v, err := s.deps.Provider.VerifyCompany(ctx, CompanyCheck{Reference: ref, CUIT: c.Company.CUIT, LegalName: c.Company.LegalName})
		if err == nil && v.Approved && v.Reason == "" {
			v.Reason = reasonDevAutoApprove
		}
		return &v, err
	}
	return nil, nil
}

// submissionStamp makes the vendor reference unique per submission, so a case fixed after a
// rejection is checked again instead of replaying the old verdict.
func submissionStamp(t *time.Time) string {
	if t == nil {
		return "0"
	}
	return strconv.FormatInt(t.UnixMicro(), 10)
}

// selfieKey is the storage key of the person's selfie (built the same way as at upload).
func selfieKey(orgID uuid.UUID, docs []Document) string {
	for _, d := range docs {
		if d.Kind == DocSelfie && d.RepresentativeDNI == nil {
			return "org/" + orgID.String() + "/kyc/" + d.ID.String()
		}
	}
	return ""
}

func (s *service) Decide(ctx context.Context, caseID uuid.UUID, d Decision) (Case, error) {
	return s.decide(ctx, caseID, decision{Decision: d, actor: audit.ActorInternal})
}

type decision struct {
	Decision
	providerRef *string
	actor       audit.ActorKind
}

// decide records a verdict on a submitted case, mirrors it on the org and, on approval, runs
// every ApprovalHook in the SAME transaction (R-KYC-3: accounts open with the approval).
func (s *service) decide(ctx context.Context, caseID uuid.UUID, d decision) (Case, error) {
	to := StatusRejected
	if d.Approved {
		to = StatusApproved
	}
	var out Case
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := q.GetKYCCaseForUpdate(ctx, caseID)
		if err != nil {
			return notFoundAs(err, apperr.CodeKYCCaseNotFound)
		}
		if !transitionAllowed(Status(row.Status), to) {
			return apperr.New(apperr.CodeKYCInvalidState, map[string]any{"status": row.Status})
		}
		now := s.now()
		if n, err := q.DecideKYCCase(ctx, store.DecideKYCCaseParams{
			ID: caseID, Status: string(to), DecisionReason: d.Reason, ProviderRef: d.providerRef, DecidedAt: &now,
		}); err != nil || n != 1 {
			return stateConflict(err, n)
		}
		if err := s.applyDecision(ctx, tx, row, to, d.actor); err != nil {
			return err
		}
		row, err = q.GetKYCCase(ctx, caseID)
		if err != nil {
			return err
		}
		out, err = loadCase(ctx, q, row)
		return err
	})
	return out, err
}

func (s *service) applyDecision(ctx context.Context, tx pgx.Tx, row store.KycCase, to Status, actor audit.ActorKind) error {
	if err := s.deps.Orgs.SetKYCStatus(ctx, tx, row.OrgID, string(to)); err != nil {
		return err
	}
	if to == StatusApproved {
		id, err := verifiedIdentity(row)
		if err != nil {
			return err
		}
		if err := s.deps.Orgs.ApplyVerifiedIdentity(ctx, tx, row.OrgID, id); err != nil {
			return fmt.Errorf("kyc: apply verified identity: %w", err)
		}
		for _, hook := range s.deps.OnApprove {
			if err := hook.OnKYCApproved(ctx, tx, row.OrgID, orgKindFor(row.Kind)); err != nil {
				return fmt.Errorf("kyc: approval hook: %w", err)
			}
		}
	}
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &row.OrgID, ActorKind: actor, Action: "kyc." + string(to), TargetType: "kyc_case", TargetID: &row.ID,
	})
}

// verifiedIdentity is what the approved case proved: the company's CUIT and legal name, or the
// person's DNI and full name (copied onto the org so the ERP link finds it by CUIT).
func verifiedIdentity(row store.KycCase) (orgs.VerifiedIdentity, error) {
	var data caseData
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return orgs.VerifiedIdentity{}, fmt.Errorf("kyc: decode case data: %w", err)
	}
	switch {
	case data.Company != nil:
		return orgs.VerifiedIdentity{LegalName: data.Company.LegalName, DocType: orgs.DocTypeCUIT, DocNumber: data.Company.CUIT}, nil
	case data.Person != nil:
		return orgs.VerifiedIdentity{LegalName: data.Person.FullName, DocType: orgs.DocTypeDNI, DocNumber: data.Person.DNI}, nil
	}
	return orgs.VerifiedIdentity{}, apperr.New(apperr.CodeKYCIncomplete, nil) // submit requires the data
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// notFoundAs maps "no rows" to the feature's not-found code and passes other errors through.
func notFoundAs(err error, code apperr.Code) error {
	if db.IsNoRows(err) {
		return apperr.Wrap(code, err, nil)
	}
	return err
}

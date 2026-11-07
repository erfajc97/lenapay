package orgs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
)

// Service is the orgs API used by the handler and by other features.
type Service interface {
	// List returns the caller's orgs (R-ORG-1).
	List(ctx context.Context, p auth.Principal) ([]Organization, error)
	// Create creates an org, makes the caller its admin and opens its KYC case in draft.
	Create(ctx context.Context, p auth.Principal, in CreateInput) (Organization, error)
	// Get returns the principal's org (p.OrgID).
	Get(ctx context.Context, p auth.Principal) (Organization, error)
	// ListMembers lists the members of p.OrgID.
	ListMembers(ctx context.Context, p auth.Principal) ([]Member, error)
	// AddMember adds a registered user with a role not above the caller's (T4).
	AddMember(ctx context.Context, p auth.Principal, in MemberInput) (Member, error)
	// CreateInvite emails a single-use invite valid 7 days (R-ORG-2).
	CreateInvite(ctx context.Context, p auth.Principal, in MemberInput) (Invite, error)
	// AcceptInvite makes the caller a member (token hashed, single use, email must match).
	AcceptInvite(ctx context.Context, p auth.Principal, token string) (identity.MembershipSummary, error)

	// MembershipsForUser implements identity.Memberships.
	MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]identity.MembershipSummary, error)
	// SetKYCStatus records the org's KYC status inside the kyc transaction (kyc.OrgStatus).
	SetKYCStatus(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, status string) error
	// OrgCUIT returns the org's CUIT for the ERP identity lookup ("" when it has none).
	OrgCUIT(ctx context.Context, orgID uuid.UUID) (string, error)
}

// Deps are the collaborators of the orgs service.
type Deps struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Audit  *audit.Logger
	Users  Users
	Mailer InviteMailer
	KYC    KYCCases
	Config config.Config
}

// InviteTTL is how long an invite can be accepted (R-ORG-2).
const InviteTTL = 7 * 24 * time.Hour

type service struct {
	deps Deps
}

// NewService returns the orgs service.
func NewService(deps Deps) Service { return &service{deps: deps} }

func (s *service) now() time.Time { return s.deps.Clock.Now() }

func (s *service) List(ctx context.Context, p auth.Principal) ([]Organization, error) {
	rows, err := store.New(s.deps.Pool).ListOrganizationsForUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]Organization, 0, len(rows))
	for _, r := range rows {
		out = append(out, Organization{
			ID: r.ID, Kind: Kind(r.Kind), LegalName: r.LegalName, CUIT: r.Cuit, KYCStatus: r.KycStatus,
			Role: auth.Role(r.Role), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *service) Create(ctx context.Context, p auth.Principal, in CreateInput) (Organization, error) {
	if in.Kind == KindPersonal && !s.deps.Config.PersonalModeEnabled {
		return Organization{}, apperr.New(apperr.CodePersonalModeDisabled, nil)
	}
	org := Organization{
		ID: ids.New(), Kind: in.Kind, LegalName: strings.TrimSpace(in.LegalName), CUIT: in.CUIT,
		KYCStatus: auth.KYCDraft, Role: auth.RoleAdmin, CreatedAt: s.now(),
	}
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		if in.Kind == KindPersonal {
			if err := s.ensureNoPersonalOrg(ctx, tx, q, p.UserID); err != nil {
				return err
			}
		}
		if err := q.InsertOrganization(ctx, store.InsertOrganizationParams{
			ID: org.ID, Kind: string(org.Kind), LegalName: org.LegalName, Cuit: org.CUIT, CreatedBy: p.UserID, CreatedAt: org.CreatedAt,
		}); err != nil {
			return err
		}
		if err := q.InsertMembership(ctx, store.InsertMembershipParams{
			OrgID: org.ID, UserID: p.UserID, Role: string(auth.RoleAdmin), CreatedAt: org.CreatedAt,
		}); err != nil {
			return err
		}
		if err := s.deps.KYC.OpenCase(ctx, tx, org.ID, string(org.Kind)); err != nil {
			return err
		}
		return s.record(ctx, tx, p, org.ID, "org.create", "organization", org.ID, map[string]any{"kind": org.Kind})
	})
	if err != nil {
		return Organization{}, err
	}
	return org, nil
}

// ensureNoPersonalOrg: a person has ONE personal wallet (R-ACC-1). The advisory lock
// serializes two concurrent creations by the same user.
func (s *service) ensureNoPersonalOrg(ctx context.Context, tx pgx.Tx, q *store.Queries, userID uuid.UUID) error {
	if err := db.AdvisoryXactLock(ctx, tx, "personal-org:"+userID.String()); err != nil {
		return err
	}
	n, err := q.CountPersonalOrgsCreatedBy(ctx, userID)
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.New(apperr.CodeConflict, map[string]any{"kind": string(KindPersonal)})
	}
	return nil
}

func (s *service) record(ctx context.Context, tx pgx.Tx, p auth.Principal, orgID uuid.UUID, action, targetType string, targetID uuid.UUID, payload any) error {
	return s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &orgID, ActorID: &p.UserID, ActorKind: audit.ActorUser,
		Action: action, TargetType: targetType, TargetID: &targetID, Payload: payload,
	})
}

func (s *service) Get(ctx context.Context, p auth.Principal) (Organization, error) {
	r, err := store.New(s.deps.Pool).GetOrganizationForUser(ctx, store.GetOrganizationForUserParams{OrgID: p.OrgID, UserID: p.UserID})
	if err != nil {
		return Organization{}, notFoundAs(err, apperr.CodeOrgNotFound)
	}
	return Organization{
		ID: r.ID, Kind: Kind(r.Kind), LegalName: r.LegalName, CUIT: r.Cuit, KYCStatus: r.KycStatus,
		Role: auth.Role(r.Role), CreatedAt: r.CreatedAt,
	}, nil
}

func (s *service) ListMembers(ctx context.Context, p auth.Principal) ([]Member, error) {
	rows, err := store.New(s.deps.Pool).ListMembers(ctx, p.OrgID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, 0, len(rows))
	for _, r := range rows {
		out = append(out, Member{UserID: r.UserID, Email: r.Email, Role: auth.Role(r.Role), JoinedAt: r.CreatedAt})
	}
	return out, nil
}

// checkGrant is the T4 rule for every door that grants a role (add member, invite): nobody
// grants a role above their own.
func checkGrant(p auth.Principal, role auth.Role) error {
	if _, ok := auth.ParseRole(string(role)); !ok {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/role"})
	}
	if !p.Role.AtLeast(role) {
		return apperr.New(apperr.CodeRoleEscalation, map[string]any{"role": string(role)})
	}
	return nil
}

func (s *service) AddMember(ctx context.Context, p auth.Principal, in MemberInput) (Member, error) {
	if err := checkGrant(p, in.Role); err != nil {
		return Member{}, err
	}
	userID, err := s.deps.Users.UserIDByEmail(ctx, in.Email)
	if err != nil {
		return Member{}, err
	}
	var m Member
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		err := q.InsertMembership(ctx, store.InsertMembershipParams{OrgID: p.OrgID, UserID: userID, Role: string(in.Role), CreatedAt: s.now()})
		if db.IsUniqueViolation(err, "") {
			return apperr.Wrap(apperr.CodeMemberAlreadyExists, err, nil)
		}
		if err != nil {
			return err
		}
		row, err := q.GetMember(ctx, store.GetMemberParams{OrgID: p.OrgID, UserID: userID})
		if err != nil {
			return err
		}
		m = Member{UserID: row.UserID, Email: row.Email, Role: auth.Role(row.Role), JoinedAt: row.CreatedAt}
		return s.record(ctx, tx, p, p.OrgID, "member.add", "user", userID, map[string]any{"role": in.Role})
	})
	if err != nil {
		return Member{}, err
	}
	return m, nil
}

func (s *service) CreateInvite(ctx context.Context, p auth.Principal, in MemberInput) (Invite, error) {
	if err := checkGrant(p, in.Role); err != nil {
		return Invite{}, err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if err := s.ensureNotMember(ctx, p.OrgID, email); err != nil {
		return Invite{}, err
	}
	now := s.now()
	token := newInviteToken()
	inv := Invite{ID: ids.New(), Email: email, Role: in.Role, ExpiresAt: now.Add(InviteTTL), CreatedAt: now}
	var orgName string
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		org, err := q.GetOrganization(ctx, p.OrgID)
		if err != nil {
			return notFoundAs(err, apperr.CodeOrgNotFound)
		}
		orgName = org.LegalName
		if err := q.InsertInvite(ctx, store.InsertInviteParams{
			ID: inv.ID, OrgID: p.OrgID, Email: email, Role: string(in.Role), TokenHash: inviteTokenHash(token),
			InvitedBy: p.UserID, ExpiresAt: inv.ExpiresAt, CreatedAt: now,
		}); err != nil {
			return err
		}
		return s.record(ctx, tx, p, p.OrgID, "invite.create", "invite", inv.ID, map[string]any{"role": in.Role})
	})
	if err != nil {
		return Invite{}, err
	}
	// After commit (no network call inside a transaction, AG11): the invite exists either way;
	// a failed delivery is reported in the log and the admin invites again.
	if err := s.deps.Mailer.SendInvite(ctx, email, orgName, token); err != nil {
		slog.ErrorContext(ctx, "invite delivery failed", "invite_id", inv.ID.String(), "err", err.Error())
	}
	return inv, nil
}

// ensureNotMember refuses inviting someone who already belongs to the org.
func (s *service) ensureNotMember(ctx context.Context, orgID uuid.UUID, email string) error {
	userID, err := s.deps.Users.UserIDByEmail(ctx, email)
	if apperr.IsCode(err, apperr.CodeUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = store.New(s.deps.Pool).GetMembership(ctx, store.GetMembershipParams{OrgID: orgID, UserID: userID})
	if db.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return apperr.New(apperr.CodeMemberAlreadyExists, nil)
}

// AcceptInvite: an unknown, already accepted or foreign (other email) token is invite.not_found
// (never revealing it exists); a known, expired one is invite.expired.
func (s *service) AcceptInvite(ctx context.Context, p auth.Principal, token string) (identity.MembershipSummary, error) {
	var out identity.MembershipSummary
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		inv, err := q.GetInviteByTokenHashForUpdate(ctx, inviteTokenHash(token))
		if err != nil {
			return notFoundAs(err, apperr.CodeInviteNotFound)
		}
		if err := s.checkInvitee(ctx, inv, p.UserID); err != nil {
			return err
		}
		if err := s.join(ctx, q, inv, p.UserID); err != nil {
			return err
		}
		org, err := q.GetOrganization(ctx, inv.OrgID)
		if err != nil {
			return err
		}
		out = identity.MembershipSummary{
			OrgID: org.ID, OrgName: org.LegalName, OrgKind: org.Kind, Role: auth.Role(inv.Role), KYCStatus: org.KycStatus,
		}
		return s.record(ctx, tx, p, inv.OrgID, "invite.accept", "invite", inv.ID, map[string]any{"role": inv.Role})
	})
	if err != nil {
		return identity.MembershipSummary{}, err
	}
	return out, nil
}

func (s *service) checkInvitee(ctx context.Context, inv store.Invite, userID uuid.UUID) error {
	if inv.AcceptedAt != nil {
		return apperr.New(apperr.CodeInviteNotFound, nil)
	}
	invitee, err := s.deps.Users.UserIDByEmail(ctx, inv.Email)
	if apperr.IsCode(err, apperr.CodeUserNotFound) || (err == nil && invitee != userID) {
		return apperr.New(apperr.CodeInviteNotFound, nil)
	}
	if err != nil {
		return err
	}
	if !inv.ExpiresAt.After(s.now()) {
		return apperr.New(apperr.CodeInviteExpired, nil)
	}
	return nil
}

func (s *service) join(ctx context.Context, q *store.Queries, inv store.Invite, userID uuid.UUID) error {
	now := s.now()
	n, err := q.MarkInviteAccepted(ctx, store.MarkInviteAcceptedParams{ID: inv.ID, AcceptedAt: &now, AcceptedBy: &userID})
	if err != nil {
		return err
	}
	if n != 1 {
		return apperr.New(apperr.CodeInviteNotFound, nil)
	}
	err = q.InsertMembership(ctx, store.InsertMembershipParams{OrgID: inv.OrgID, UserID: userID, Role: inv.Role, CreatedAt: now})
	if db.IsUniqueViolation(err, "") {
		return apperr.Wrap(apperr.CodeMemberAlreadyExists, err, nil)
	}
	return err
}

func (s *service) MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]identity.MembershipSummary, error) {
	rows, err := store.New(s.deps.Pool).ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]identity.MembershipSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, identity.MembershipSummary{
			OrgID: r.ID, OrgName: r.LegalName, OrgKind: r.Kind, Role: auth.Role(r.Role), KYCStatus: r.KycStatus,
		})
	}
	return out, nil
}

func (s *service) SetKYCStatus(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, status string) error {
	n, err := store.New(tx).SetOrganizationKYCStatus(ctx, store.SetOrganizationKYCStatusParams{ID: orgID, KycStatus: status, UpdatedAt: s.now()})
	if err != nil {
		return err
	}
	if n != 1 {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	return nil
}

func (s *service) OrgCUIT(ctx context.Context, orgID uuid.UUID) (string, error) {
	org, err := store.New(s.deps.Pool).GetOrganization(ctx, orgID)
	if err != nil {
		return "", notFoundAs(err, apperr.CodeOrgNotFound)
	}
	if org.Cuit == nil {
		return "", nil
	}
	return *org.Cuit, nil
}

// newInviteToken is 256 random bits, base64url (43 chars: the contract's token pattern).
func newInviteToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (it panics on a broken OS source)
	return base64.RawURLEncoding.EncodeToString(b)
}

// inviteTokenHash is how tokens are stored: only their SHA-256 (A12: hashed single-use link).
func inviteTokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// notFoundAs maps "no rows" to the feature's not-found code and passes other errors through.
func notFoundAs(err error, code apperr.Code) error {
	if db.IsNoRows(err) {
		return apperr.Wrap(code, err, nil)
	}
	return err
}

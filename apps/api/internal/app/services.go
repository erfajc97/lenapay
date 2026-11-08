package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/fees"
	"lenapay/apps/api/internal/identity"
	"lenapay/apps/api/internal/kyc"
	"lenapay/apps/api/internal/ledger"
	"lenapay/apps/api/internal/movements"
	"lenapay/apps/api/internal/notifications"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/payerview"
	"lenapay/apps/api/internal/payins"
	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/idempotency"
	"lenapay/apps/api/internal/platform/jobs"
	"lenapay/apps/api/internal/psp"
	"lenapay/apps/api/internal/reconciliation"
)

// Ports are the external dependencies (P2): fakes locally, real adapters later.
type Ports struct {
	PSP          psp.Provider
	KYC          kyc.Provider
	KYCDocuments kyc.DocumentStore
	ERP          erplink.Directory
	Sender       notifications.Sender
	Documents    payouts.DocumentStore
}

// Platform is the shared infrastructure every service gets.
type Platform struct {
	Pool   *pgxpool.Pool
	Clock  clock.Clock
	Jobs   jobs.Enqueuer
	Issuer *auth.Issuer
	Config config.Config
}

// Services holds every feature service, wired once for cmd/api and cmd/worker (P5:
// cross-feature calls go through these interfaces).
type Services struct {
	Identity       identity.Service
	Orgs           orgs.Service
	KYC            kyc.Service
	Ledger         ledger.Service
	Fees           fees.Service
	Accounts       accounts.Service
	Payers         payers.Service
	Payins         payins.Service
	Payouts        payouts.Service
	Movements      movements.Service
	Reconciliation reconciliation.Service
	ERPLink        erplink.Service
	Notifications  notifications.Service
	PayerView      payerview.Service
}

// NewServices builds every service in dependency order.
func NewServices(p Platform, ports Ports) *Services {
	auditLog := audit.NewLogger(p.Clock)
	idem := idempotency.NewStore(p.Clock, idempotency.DefaultTTL)
	s := &Services{}

	s.Ledger = ledger.NewService(ledger.Deps{Pool: p.Pool, Clock: p.Clock})
	s.Fees = fees.NewService(fees.Deps{Pool: p.Pool})
	s.Notifications = notifications.NewService(notifications.Deps{Pool: p.Pool, Clock: p.Clock, Jobs: p.Jobs, Sender: ports.Sender})

	// identity ↔ orgs: identity needs memberships, orgs needs users. The closure reads s.Orgs
	// at call time, after both exist.
	s.Identity = identity.NewService(identity.Deps{
		Pool: p.Pool, Clock: p.Clock, Issuer: p.Issuer, Codes: s.Notifications, Config: p.Config,
		Memberships: identity.MembershipsFunc(s.membershipsForUser), OnSignup: identity.SignupHookFunc(s.onSignup),
		Accounts: identity.AccountsFunc(s.accountArchived), Invites: identity.InviteAcceptorFunc(s.acceptERPInvites),
	})
	s.Orgs = orgs.NewService(orgs.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Users: s.Identity, Mailer: s.Notifications, Config: p.Config,
		KYC: orgs.KYCCasesFunc(s.openKYCCase), Names: s.Identity,
	})

	// accounts ↔ payers: accounts lists its own CVUs, which payers owns; the closure reads
	// s.Payers at call time, after both exist.
	s.Accounts = accounts.NewService(accounts.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Idempotency: idem, Ledger: s.Ledger,
		CVUs: accounts.ReceivingCVUsFunc(s.receivingCVUs),
	})
	s.Payers = payers.NewService(payers.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Idempotency: idem, Jobs: p.Jobs, PSP: ports.PSP, Accounts: s.Accounts,
		Orgs: s.Orgs, ShareBaseURL: p.Config.CVUShareBaseURL,
	})
	s.KYC = kyc.NewService(kyc.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Provider: ports.KYC, Documents: ports.KYCDocuments,
		Orgs: s.Orgs, OnApprove: []kyc.ApprovalHook{s.Accounts, s.Payers, payerViewHook{s}},
		Jobs: p.Jobs, AutoApproveAfter: p.Config.DevKYCAutoApproveDelay(),
	})
	s.Payins = payins.NewService(payins.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Jobs: p.Jobs, PSP: ports.PSP, Payers: s.Payers,
		Ledger: s.Ledger, Fees: s.Fees, Notifications: s.Notifications,
		WebhookSecret: p.Config.PSPWebhookSecret, Tolerance: p.Config.PSPWebhookTolerance,
		Accounts: s.Accounts, Members: s.Orgs,
	})
	s.Payouts = payouts.NewService(payouts.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Idempotency: idem, Jobs: p.Jobs, PSP: ports.PSP,
		Accounts: s.Accounts, Ledger: s.Ledger, Fees: s.Fees, Notifications: s.Notifications,
		Orgs: s.Orgs, Documents: ports.Documents, Location: p.Config.Location(),
		CVUs: s.Payers, Credits: s.Payins, Policies: s.Orgs,
	})
	s.Movements = movements.NewService(movements.Deps{Pool: p.Pool, Location: p.Config.Location()})
	s.Reconciliation = reconciliation.NewService(reconciliation.Deps{
		Pool: p.Pool, Clock: p.Clock, Jobs: p.Jobs, PSP: ports.PSP, Payins: s.Payins, Payouts: s.Payouts,
		CVUs: s.Payers, Location: p.Config.Location(),
	})
	s.ERPLink = erplink.NewService(erplink.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Jobs: p.Jobs, Directory: ports.ERP,
		Orgs: s.Orgs, Accounts: s.Accounts, Payers: s.Payers, Invites: s.Orgs,
	})
	s.PayerView = payerview.NewService(payerview.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, ERP: s.ERPLink, Payouts: s.Payouts,
	})
	return s
}

// NewServer builds the composite strict server over the services.
func NewServer(s *Services, issuer *auth.Issuer, health Pinger) *Server {
	return &Server{
		IdentityHandler:       identity.NewHandler(s.Identity, issuer.JWKS),
		OrgsHandler:           orgs.NewHandler(s.Orgs),
		KYCHandler:            kyc.NewHandler(s.KYC),
		AccountsHandler:       accounts.NewHandler(s.Accounts),
		PayersHandler:         payers.NewHandler(s.Payers),
		PayinsHandler:         payins.NewHandler(s.Payins),
		PayoutsHandler:        payouts.NewHandler(s.Payouts),
		MovementsHandler:      movements.NewHandler(s.Movements),
		ReconciliationHandler: reconciliation.NewHandler(s.Reconciliation),
		ERPLinkHandler:        erplink.NewHandler(s.ERPLink),
		NotificationsHandler:  notifications.NewHandler(s.Notifications),
		PayerViewHandler:      payerview.NewHandler(s.PayerView),
		Health:                health,
	}
}

// membershipsForUser forwards to orgs (see the identity ↔ orgs note in NewServices).
func (s *Services) membershipsForUser(ctx context.Context, userID uuid.UUID) ([]identity.MembershipSummary, error) {
	return s.Orgs.MembershipsForUser(ctx, userID)
}

// onSignup gives a new user their personal org and personal account (personal mode), in the
// signup transaction; the personal CVUs follow on the person KYC approval (payers hook).
func (s *Services) onSignup(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) error {
	orgID, created, err := s.Orgs.CreatePersonalForUser(ctx, tx, userID, email)
	if err != nil || !created {
		return err
	}
	return s.Accounts.OpenOrgAccount(ctx, tx, orgID, string(orgs.KindPersonal))
}

// accountArchived forwards to accounts (identity is built before accounts): a preferred default
// account must exist in that org and not be archived.
func (s *Services) accountArchived(ctx context.Context, orgID, accountID uuid.UUID) (bool, error) {
	a, err := s.Accounts.Get(ctx, orgID, accountID)
	if err != nil {
		return false, err
	}
	return a.Archived, nil
}

// acceptERPInvites forwards to orgs (see the identity ↔ orgs note in NewServices).
func (s *Services) acceptERPInvites(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) (int, error) {
	return s.Orgs.AcceptERPInvites(ctx, tx, userID, email)
}

// receivingCVUs forwards to payers (see the accounts ↔ payers note in NewServices).
func (s *Services) receivingCVUs(ctx context.Context, orgID uuid.UUID) ([]accounts.ReceivingCVU, error) {
	return s.Payers.ReceivingCVUs(ctx, orgID)
}

// openKYCCase forwards to kyc (orgs is built before kyc, which needs orgs.Service; the closure
// reads s.KYC at call time, after both exist).
func (s *Services) openKYCCase(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	return s.KYC.OpenCase(ctx, tx, orgID, orgKind)
}

// payerViewHook forwards KYC approvals to payerview, which is built after kyc (it needs payouts
// and erplink); it reads s.PayerView at call time, after every service exists.
type payerViewHook struct{ s *Services }

// OnKYCApproved implements kyc.ApprovalHook.
func (h payerViewHook) OnKYCApproved(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	return h.s.PayerView.OnKYCApproved(ctx, tx, orgID, orgKind)
}

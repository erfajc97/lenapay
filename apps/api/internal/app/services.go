package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/accounts"
	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/fees"
	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/kyc"
	"github.com/erfajc97/lenapay/apps/api/internal/ledger"
	"github.com/erfajc97/lenapay/apps/api/internal/movements"
	"github.com/erfajc97/lenapay/apps/api/internal/notifications"
	"github.com/erfajc97/lenapay/apps/api/internal/orgs"
	"github.com/erfajc97/lenapay/apps/api/internal/payers"
	"github.com/erfajc97/lenapay/apps/api/internal/payins"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/audit"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/idempotency"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/jobs"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/reconciliation"
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
		Memberships: identity.MembershipsFunc(s.membershipsForUser),
	})
	s.Orgs = orgs.NewService(orgs.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Users: s.Identity, Mailer: s.Notifications, Config: p.Config,
		KYC: orgs.KYCCasesFunc(s.openKYCCase),
	})

	s.Accounts = accounts.NewService(accounts.Deps{Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Idempotency: idem, Ledger: s.Ledger})
	s.Payers = payers.NewService(payers.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Idempotency: idem, Jobs: p.Jobs, PSP: ports.PSP, Accounts: s.Accounts,
		Orgs: s.Orgs, ShareBaseURL: p.Config.CVUShareBaseURL,
	})
	s.KYC = kyc.NewService(kyc.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Provider: ports.KYC, Documents: ports.KYCDocuments,
		Orgs: s.Orgs, OnApprove: []kyc.ApprovalHook{s.Accounts, s.Payers},
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
	})
	s.Movements = movements.NewService(movements.Deps{Pool: p.Pool, Location: p.Config.Location()})
	s.Reconciliation = reconciliation.NewService(reconciliation.Deps{
		Pool: p.Pool, Clock: p.Clock, Jobs: p.Jobs, PSP: ports.PSP, Payins: s.Payins, Payouts: s.Payouts,
		CVUs: s.Payers, Location: p.Config.Location(),
	})
	s.ERPLink = erplink.NewService(erplink.Deps{
		Pool: p.Pool, Clock: p.Clock, Audit: auditLog, Jobs: p.Jobs, Directory: ports.ERP,
		Orgs: s.Orgs, Accounts: s.Accounts, Payers: s.Payers,
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
		Health:                health,
	}
}

// membershipsForUser forwards to orgs (see the identity ↔ orgs note in NewServices).
func (s *Services) membershipsForUser(ctx context.Context, userID uuid.UUID) ([]identity.MembershipSummary, error) {
	return s.Orgs.MembershipsForUser(ctx, userID)
}

// openKYCCase forwards to kyc (orgs is built before kyc, which needs orgs.Service; the closure
// reads s.KYC at call time, after both exist).
func (s *Services) openKYCCase(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, orgKind string) error {
	return s.KYC.OpenCase(ctx, tx, orgID, orgKind)
}

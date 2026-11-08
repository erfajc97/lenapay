// Package app wires the features into one HTTP API: the composite strict Server, the access
// table and the router. It is the only package that imports every feature handler.
package app

import (
	"context"

	"lenapay/apps/api/internal/accounts"
	"lenapay/apps/api/internal/api"
	"lenapay/apps/api/internal/erplink"
	"lenapay/apps/api/internal/identity"
	"lenapay/apps/api/internal/kyc"
	"lenapay/apps/api/internal/movements"
	"lenapay/apps/api/internal/notifications"
	"lenapay/apps/api/internal/orgs"
	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/payerview"
	"lenapay/apps/api/internal/payins"
	"lenapay/apps/api/internal/payouts"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/reconciliation"
)

// Feature handlers are aliased so each embedded field gets a distinct name (they are all
// called Handler in their own package).
type (
	IdentityHandler       = identity.Handler
	OrgsHandler           = orgs.Handler
	KYCHandler            = kyc.Handler
	AccountsHandler       = accounts.Handler
	PayersHandler         = payers.Handler
	PayinsHandler         = payins.Handler
	PayoutsHandler        = payouts.Handler
	MovementsHandler      = movements.Handler
	ReconciliationHandler = reconciliation.Handler
	ERPLinkHandler        = erplink.Handler
	NotificationsHandler  = notifications.Handler
	PayerViewHandler      = payerview.Handler
)

// Server is the strict server: one embedded handler per feature, each implementing only its
// own operations, so lanes edit only their package. A missing operation is a compile error.
type Server struct {
	*IdentityHandler
	*OrgsHandler
	*KYCHandler
	*AccountsHandler
	*PayersHandler
	*PayinsHandler
	*PayoutsHandler
	*MovementsHandler
	*ReconciliationHandler
	*ERPLinkHandler
	*NotificationsHandler
	*PayerViewHandler

	// Health pings the database for GET /healthz (*pgxpool.Pool in cmd/api).
	Health Pinger
}

// Pinger checks a dependency is up.
type Pinger interface {
	Ping(ctx context.Context) error
}

var _ api.StrictServerInterface = (*Server)(nil)

// GetHealth answers 200 when the database answers (docker healthcheck, load balancer).
func (s *Server) GetHealth(ctx context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	if err := s.Health.Ping(ctx); err != nil {
		return nil, apperr.Wrap(apperr.CodeProviderUnavailable, err, map[string]any{"dependency": "postgres"})
	}
	return api.GetHealth200JSONResponse{Status: api.Ok}, nil
}

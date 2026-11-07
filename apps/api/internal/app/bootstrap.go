package app

import (
	"context"
	"errors"
	"log/slog"

	erpfake "github.com/erfajc97/lenapay/apps/api/internal/erplink/provider/fake"
	kycfake "github.com/erfajc97/lenapay/apps/api/internal/kyc/provider/fake"
	notifyfake "github.com/erfajc97/lenapay/apps/api/internal/notifications/provider/fake"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
	"github.com/erfajc97/lenapay/apps/api/internal/psp/provider/bind"
	pspfake "github.com/erfajc97/lenapay/apps/api/internal/psp/provider/fake"
)

// NewIssuer builds the local identity provider. Without JWT_PRIVATE_KEY (development and test
// only; config refuses it in production) it generates an ephemeral key: tokens die on restart.
func NewIssuer(ctx context.Context, cfg config.Config, clk clock.Clock) (*auth.Issuer, error) {
	seed := cfg.JWTSeed()
	if seed == nil {
		if cfg.IsProduction() {
			return nil, errors.New("JWT_PRIVATE_KEY is required in production")
		}
		slog.WarnContext(ctx, "JWT_PRIVATE_KEY not set: using an ephemeral signing key (tokens die on restart)")
		seed = auth.GenerateSeed()
	}
	return auth.NewIssuer(auth.IssuerConfig{
		Issuer: cfg.JWTIssuer, Seed: seed, AccessTTL: cfg.AccessTTL, StepUpTTL: cfg.StepUpTTL, Clock: clk,
	})
}

// FakePorts are the in-process fakes of every external dependency (P2), except the PSP: with
// PSP_MODE=http (the default) it is the BIND HTTP adapter at PSP_BASE_URL (apps/bind-mock
// locally, the same adapter for the real BIND later); PSP_MODE=fake keeps it in process.
func FakePorts(cfg config.Config) Ports {
	return Ports{
		PSP:          newPSP(cfg),
		KYC:          kycfake.NewProvider(cfg.KYCFakeRejectedDNIs),
		KYCDocuments: kycfake.NewDocumentStore(),
		ERP:          erpfake.NewDirectory(cfg.ERPFixturesDir),
		Sender:       notifyfake.NewSender(cfg.DevInboxEnabled),
	}
}

// newPSP picks the PSP adapter by PSP_MODE (config validated it; production is always http).
func newPSP(cfg config.Config) psp.Provider {
	if cfg.PSPMode == config.PSPModeFake {
		return pspfake.New()
	}
	return bind.New(bind.Options{BaseURL: cfg.PSPBaseURL, Timeout: cfg.PSPTimeout})
}

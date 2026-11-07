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

// FakePorts are the in-process fakes of every external dependency (P2). Lane C swaps the PSP
// fake for the bind-mock HTTP adapter (cfg.PSPBaseURL) here.
func FakePorts(cfg config.Config) Ports {
	return Ports{
		PSP:          pspfake.New(),
		KYC:          kycfake.NewProvider(cfg.KYCFakeRejectedDNIs),
		KYCDocuments: kycfake.NewDocumentStore(),
		ERP:          erpfake.NewDirectory(cfg.ERPFixturesDir),
		Sender:       notifyfake.NewSender(cfg.DevInboxEnabled),
	}
}

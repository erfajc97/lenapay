// Command seed loads the demo dataset (E9) into a running local stack, idempotently, through
// the real services: users, the company org approved by KYC, project accounts, payers with
// CVUs at the PSP, payins delivered by the bind-mock's signed webhooks (the worker credits
// them through the ledger), payouts with approvals, an internal transfer, a reconciliation run
// and a personal wallet. It needs postgres, bind-mock and the worker up (`docker compose up`).
// Run it again any time: every step checks what already exists.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // the business time zone must load in minimal containers

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/erfajc97/lenapay/apps/api/internal/app"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	applog "github.com/erfajc97/lenapay/apps/api/internal/platform/log"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load(nil)
	if err != nil {
		return err
	}
	logger := applog.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(logger)
	if cfg.IsProduction() {
		return errors.New("seed refuses to run in production")
	}
	// The demo users are verified with the fixed development code (the same one the API
	// uses when DEV_FIXED_EMAIL_CODE is set; the seed sets it for its own process otherwise).
	if cfg.DevFixedEmailCode == "" {
		cfg.DevFixedEmailCode = "123456"
	}
	pool, err := db.NewPool(ctx, cfg.DatabaseURL.Reveal(), 4)
	if err != nil {
		return err
	}
	defer pool.Close()

	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Logger: logger})
	if err != nil {
		return err
	}
	clk := clock.Real{}
	issuer, err := app.NewIssuer(ctx, cfg, clk)
	if err != nil {
		return err
	}
	services := app.NewServices(app.Platform{Pool: pool, Clock: clk, Jobs: jobs, Issuer: issuer, Config: cfg}, app.FakePorts(cfg))
	if err := services.Ledger.EnsureSystemAccounts(ctx); err != nil {
		return err
	}
	if err := services.Fees.EnsureDefaultPlan(ctx); err != nil {
		return err
	}
	sd := &seeder{cfg: cfg, s: services, http: &http.Client{Timeout: 15 * time.Second}}
	if err := sd.run(ctx); err != nil {
		return err
	}
	slog.InfoContext(ctx, "seed complete", "demo", demoEmail, "approver", approverEmail, "persona", personaEmail)
	return nil
}

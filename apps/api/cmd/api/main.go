// Command api serves the HTTP API: wiring only (config → pool → migrations → River insert-only
// client → adapters → services → router). Jobs run in cmd/worker; this process only inserts them.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // the business time zone must load in minimal containers

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"golang.org/x/sync/errgroup"

	"lenapay/apps/api/internal/app"
	"lenapay/apps/api/internal/payins"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/httpx"
	applog "lenapay/apps/api/internal/platform/log"
	platformrealtime "lenapay/apps/api/internal/platform/realtime"
	"lenapay/apps/api/internal/realtime"
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

	cfg, err := config.Load(nil) // fail fast: a bad env never serves a request
	if err != nil {
		return err
	}
	logger := applog.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(logger)

	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, cfg.DatabaseURL.Reveal()); err != nil {
			return err
		}
	}
	pool, err := db.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Insert-only River client: no queues, no workers in the API process (AG13).
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
	hub := platformrealtime.NewHub(pool)

	handler, err := app.NewRouter(app.Deps{
		Server:             app.NewServer(services, issuer, pool),
		Guard:              auth.NewGuard(app.AccessRules(), issuer, auth.NewPGDirectory(pool), cfg.InternalAPIKey),
		Webhook:            payins.NewWebhookHandler(services.Payins),
		Events:             realtime.NewStreamHandler(hub),
		CORSOrigins:        cfg.CORSOrigins,
		TrustedProxies:     cfg.TrustedProxies,
		RateLimitPerMinute: cfg.RateLimitPerMinute,
		BodyLimitBytes:     cfg.BodyLimitBytes,
		UploadLimitBytes:   cfg.UploadLimitBytes,
		RequestTimeout:     cfg.RequestTimeout,
	})
	if err != nil {
		return err
	}
	srv := httpx.NewServer(cfg.HTTPAddr, handler, logger)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		slog.InfoContext(gctx, "listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error { return hub.Run(gctx) }) // returns nil at shutdown and ends open streams
	g.Go(func() error {
		<-gctx.Done() // SIGTERM, or another goroutine failed
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(gctx), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx) // stops accepting, waits for in-flight requests
	})
	return g.Wait() // then the deferred pool.Close()
}

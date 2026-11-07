// Command worker runs the River workers and periodic jobs (payin processing, payout dispatch
// and sweep, reconciliation, ERP import, notification delivery). Wiring only.
package main

import (
	"context"
	"log/slog"
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

	pool, err := db.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	clk := clock.Real{}
	issuer, err := app.NewIssuer(ctx, cfg, clk)
	if err != nil {
		return err
	}
	// Services enqueue follow-up jobs through an insert-only client; the working client below
	// needs the workers (which need the services) at construction.
	inserter, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Logger: logger})
	if err != nil {
		return err
	}
	services := app.NewServices(app.Platform{Pool: pool, Clock: clk, Jobs: inserter, Issuer: issuer, Config: cfg}, app.FakePorts(cfg))

	workers := river.NewWorkers()
	if app.RegisterWorkers(workers, services) == 0 {
		slog.InfoContext(ctx, "worker idle: no River workers registered yet")
		<-ctx.Done()
		return nil
	}
	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Logger:       logger,
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 20}},
		Workers:      workers,
		PeriodicJobs: app.PeriodicJobs(),
		JobTimeout:   time.Minute,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	slog.InfoContext(ctx, "worker started")
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	return client.Stop(stopCtx) // finishes running jobs, then returns
}

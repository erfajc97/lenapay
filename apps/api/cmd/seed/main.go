// Command seed loads demo data into a migrated local database (E9: a demo org linked to the
// ERP fixture, payers and payins). E0 ships the wiring only: it checks the database answers
// and reports that there is nothing to seed yet; E9 adds the steps through the feature services.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
	slog.SetDefault(applog.New(os.Stdout, cfg.LogLevel))
	if cfg.IsProduction() {
		return errors.New("seed refuses to run in production")
	}
	pool, err := db.NewPool(ctx, cfg.DatabaseURL.Reveal(), 2)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.InfoContext(ctx, "seed: database reachable; no seed steps yet (backlog E9)")
	return nil
}

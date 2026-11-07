// Command migrate applies every pending migration (River's tables included): the deploy step
// before rolling out new code. `make migrate`, and the compose api with MIGRATE_ON_START.
package main

import (
	"context"
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
	if err := db.Migrate(ctx, cfg.DatabaseURL.Reveal()); err != nil {
		return err
	}
	slog.InfoContext(ctx, "migrations applied")
	return nil
}

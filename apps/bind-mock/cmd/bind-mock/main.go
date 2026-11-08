// Command bind-mock is the local fake of the BIND PSP (never deployed): wiring and graceful
// shutdown only.
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
	_ "time/tzdata" // the bank's time zone must load in minimal containers

	"github.com/erfajc97/lenapay/apps/bind-mock/internal/bank"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/config"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/httpapi"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/simulator"
	"github.com/erfajc97/lenapay/apps/bind-mock/internal/simulator/provider/webhook"
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

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	now := func() time.Time { return time.Now().UTC() }
	b, err := bank.New(bank.Options{
		StateFile: cfg.StateFile, Location: cfg.TimeZone, PendingDelay: cfg.PendingDelay,
		InitialBalanceMinor: cfg.InitialBalanceMinor, Now: now,
	})
	if err != nil {
		return err
	}
	sim := simulator.New(ctx, b, webhook.NewSender(cfg.WebhookURL, []byte(cfg.WebhookSecret.Reveal()), cfg.WebhookTimeout, now), now)
	if cfg.AutoPayinEvery > 0 {
		sim.StartAuto(ctx, cfg.AutoPayinEvery)
	}
	defer sim.Wait() // runs last: the on-us webhooks in flight finish before exit
	defer sim.StopAuto()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(httpapi.Deps{Bank: b, Simulator: sim, TimeoutDelay: cfg.TimeoutDelay}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.TimeoutDelay + 30*time.Second, // a scripted timeout must still answer
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	errCh := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "bind-mock listening", "addr", cfg.HTTPAddr, "state_file", cfg.StateFile)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

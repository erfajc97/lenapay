// Package config is the only reader of the bind-mock environment: parsed once at boot, fail
// fast, errors name keys and never values.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Secret never prints its value.
type Secret string

func (Secret) String() string   { return "[REDACTED]" }
func (Secret) GoString() string { return "[REDACTED]" }

// Reveal is the only way to read the value.
func (s Secret) Reveal() string { return string(s) }

// Config of the fake PSP.
type Config struct {
	HTTPAddr string
	// WebhookURL is where signed transfer webhooks are POSTed (the API's
	// /webhooks/psp/transfers).
	WebhookURL string
	// WebhookSecret signs webhooks (HMAC-SHA256); it must equal the API's PSP_WEBHOOK_SECRET.
	WebhookSecret  Secret
	WebhookTimeout time.Duration
	// StateFile persists the fake bank between restarts ("" keeps it in memory only).
	StateFile string
	// TimeZone is the bank's business zone: statements are per local day (BIND is Argentine).
	TimeZone *time.Location
	// PendingDelay is how long an IN_PROGRESS outgoing transfer waits before it completes.
	PendingDelay time.Duration
	// TimeoutDelay is how long a "timeout" outcome holds the response (longer than the API's
	// PSP_TIMEOUT, so the caller gives up first).
	TimeoutDelay time.Duration
	// InitialBalanceMinor seeds the master account per currency (minor units).
	InitialBalanceMinor map[string]int64
	// AutoPayinEvery starts random demo payins at boot when > 0 (also /_mock/auto-payins).
	AutoPayinEvery time.Duration
}

// Load parses the environment through getenv (os.Getenv in main, a map in tests).
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	duration := func(key, def string) time.Duration {
		d, err := time.ParseDuration(withDefault(getenv(key), def))
		if err != nil || d < 0 {
			errs = append(errs, fmt.Errorf("%s must be a non-negative duration", key))
		}
		return d
	}
	minor := func(key, def string) int64 {
		n, err := strconv.ParseInt(withDefault(getenv(key), def), 10, 64)
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("%s must be a non-negative integer of minor units", key))
		}
		return n
	}
	cfg := Config{
		HTTPAddr:       withDefault(getenv("BINDMOCK_HTTP_ADDR"), ":8090"),
		WebhookURL:     withDefault(getenv("BINDMOCK_WEBHOOK_URL"), "http://localhost:8080/webhooks/psp/transfers"),
		WebhookSecret:  Secret(withDefault(getenv("PSP_WEBHOOK_SECRET"), "dev-only-psp-webhook-secret")),
		WebhookTimeout: duration("BINDMOCK_WEBHOOK_TIMEOUT", "5s"),
		StateFile:      getenv("BINDMOCK_STATE_FILE"),
		PendingDelay:   duration("BINDMOCK_PENDING_DELAY", "20s"),
		TimeoutDelay:   duration("BINDMOCK_TIMEOUT_DELAY", "30s"),
		InitialBalanceMinor: map[string]int64{
			"ARS": minor("BINDMOCK_INITIAL_BALANCE_ARS_MINOR", "5000000000"), // ARS 50.000.000,00
			"USD": minor("BINDMOCK_INITIAL_BALANCE_USD_MINOR", "10000000"),   // USD 100.000,00
		},
		AutoPayinEvery: duration("BINDMOCK_AUTO_PAYINS_EVERY", "0s"),
	}
	loc, err := time.LoadLocation(withDefault(getenv("BINDMOCK_TIME_ZONE"), "America/Argentina/Buenos_Aires"))
	if err != nil {
		errs = append(errs, errors.New("BINDMOCK_TIME_ZONE must be an IANA time zone"))
	}
	cfg.TimeZone = loc
	if u, err := url.Parse(cfg.WebhookURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, errors.New("BINDMOCK_WEBHOOK_URL must be an absolute http(s) URL"))
	}
	if len(cfg.WebhookSecret.Reveal()) < 16 {
		errs = append(errs, errors.New("PSP_WEBHOOK_SECRET must be at least 16 characters"))
	}
	if cfg.AutoPayinEvery != 0 && cfg.AutoPayinEvery < time.Second {
		errs = append(errs, errors.New("BINDMOCK_AUTO_PAYINS_EVERY must be 0 (off) or at least 1s"))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

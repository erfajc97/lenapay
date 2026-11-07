// Package config is the only reader of the bind-mock environment: parsed once at boot, fail
// fast, errors name keys and never values.
package config

import (
	"errors"
	"fmt"
	"net/url"
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
}

// Load parses the environment through getenv (os.Getenv in main, a map in tests).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:       withDefault(getenv("BINDMOCK_HTTP_ADDR"), ":8090"),
		WebhookURL:     withDefault(getenv("BINDMOCK_WEBHOOK_URL"), "http://localhost:8080/webhooks/psp/transfers"),
		WebhookSecret:  Secret(withDefault(getenv("PSP_WEBHOOK_SECRET"), "dev-only-psp-webhook-secret")),
		WebhookTimeout: 5 * time.Second,
	}
	var errs []error
	if u, err := url.Parse(cfg.WebhookURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, errors.New("BINDMOCK_WEBHOOK_URL must be an absolute http(s) URL"))
	}
	if len(cfg.WebhookSecret.Reveal()) < 16 {
		errs = append(errs, errors.New("PSP_WEBHOOK_SECRET must be at least 16 characters"))
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

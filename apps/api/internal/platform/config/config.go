// Package config is the ONLY reader of the environment (A8, AG14). It parses every variable
// once, at boot, into a typed Config and fails fast, naming the keys (never the values) that
// are wrong. Production refuses missing secrets and development defaults alike.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/caarlos0/env/v11"
)

// Secret never prints its value: not in logs, not in %v, not in error messages.
type Secret string

func (Secret) String() string               { return "[REDACTED]" }
func (Secret) GoString() string             { return "[REDACTED]" }
func (Secret) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }
func (Secret) MarshalText() ([]byte, error) { return []byte("[REDACTED]"), nil }

// Reveal is the only way to read the value: grep for it to find every use of a secret.
func (s Secret) Reveal() string { return string(s) }

// Environments.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Development-only defaults (placeholders, not secrets). validate rejects them in production.
//
//nolint:gosec // G101: these are the published dev placeholders production refuses
const (
	devInternalAPIKey   = "dev-only-internal-api-key"
	devPSPWebhookSecret = "dev-only-psp-webhook-secret"
)

// Config is the typed configuration shared by cmd/api, cmd/worker, cmd/migrate and cmd/seed.
// Lanes APPEND fields (with .env.example and docs) — never rename or reorder (G14).
type Config struct {
	Env      string     `env:"APP_ENV,required"`
	HTTPAddr string     `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"INFO"`

	DatabaseURL Secret `env:"DATABASE_URL,required,notEmpty"`
	DBMaxConns  int32  `env:"DB_MAX_CONNS" envDefault:"10"`
	// MigrateOnStart lets cmd/api apply migrations at boot (local and compose); production runs
	// cmd/migrate as a deploy step instead.
	MigrateOnStart bool `env:"MIGRATE_ON_START" envDefault:"false"`

	RequestTimeout     time.Duration `env:"REQUEST_TIMEOUT" envDefault:"15s"`
	ShutdownTimeout    time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"20s"`
	BodyLimitBytes     int64         `env:"BODY_LIMIT_BYTES" envDefault:"1048576"`
	UploadLimitBytes   int64         `env:"UPLOAD_LIMIT_BYTES" envDefault:"10485760"`
	RateLimitPerMinute int           `env:"RATE_LIMIT_PER_MINUTE" envDefault:"300"`
	CORSOrigins        []string      `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:8081,http://localhost:19006"`
	TrustedProxies     []string      `env:"TRUSTED_PROXY_CIDRS" envSeparator:","`
	// BusinessTimeZone is the zone of business dates (A6), e.g. movements "from/to".
	BusinessTimeZone string `env:"BUSINESS_TIME_ZONE" envDefault:"America/Argentina/Buenos_Aires"`

	// JWT: the local identity provider (R-ID-4). An empty key in development means "generate
	// an ephemeral key at boot" (tokens die on restart); production requires it.
	JWTIssuer     string        `env:"JWT_ISSUER" envDefault:"http://localhost:8080"`
	JWTPrivateKey Secret        `env:"JWT_PRIVATE_KEY"`
	AccessTTL     time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTTL    time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
	StepUpTTL     time.Duration `env:"STEP_UP_TOKEN_TTL" envDefault:"5m"`

	InternalAPIKey Secret `env:"INTERNAL_API_KEY" envDefault:"dev-only-internal-api-key"`

	PSPBaseURL          string        `env:"PSP_BASE_URL" envDefault:"http://localhost:8090"`
	PSPWebhookSecret    Secret        `env:"PSP_WEBHOOK_SECRET" envDefault:"dev-only-psp-webhook-secret"`
	PSPTimeout          time.Duration `env:"PSP_TIMEOUT" envDefault:"10s"`
	PSPWebhookTolerance time.Duration `env:"PSP_WEBHOOK_TOLERANCE" envDefault:"5m"`

	PersonalModeEnabled bool     `env:"PERSONAL_MODE_ENABLED" envDefault:"false"`
	ERPFixturesDir      string   `env:"ERP_FIXTURES_DIR" envDefault:"fixtures/erp"`
	KYCFakeRejectedDNIs []string `env:"KYC_FAKE_REJECTED_DNIS" envSeparator:","`
	// DevInboxEnabled lets the fake notifier log emailed codes (R-ID-1, development only).
	DevInboxEnabled bool `env:"DEV_INBOX_ENABLED" envDefault:"false"`

	// DevFixedEmailCode makes every emailed verification code this 6-digit value (demo only;
	// empty = random codes). Production refuses it.
	DevFixedEmailCode string `env:"DEV_FIXED_EMAIL_CODE"`
	// DevKYCAutoApproveSeconds auto-approves a submitted company KYC case after this many
	// seconds through a River job (demo only; 0 = off). Ignored in production.
	DevKYCAutoApproveSeconds int `env:"DEV_KYC_AUTO_APPROVE_SECONDS" envDefault:"8"`

	// PSPMode picks the PSP adapter: "http" (BIND wire format: apps/bind-mock locally) or
	// "fake" (in-process, tests and offline demos). Production refuses fake.
	PSPMode string `env:"PSP_MODE" envDefault:"http"`
	// CVUShareBaseURL prefixes the share link of a CVU (R-CVU-2).
	CVUShareBaseURL string `env:"CVU_SHARE_BASE_URL" envDefault:"https://lenapay.app/pagar"`
	// DocumentsDir is the root of the local-disk document store (payout attachments) used
	// until an object-storage adapter exists ("" = <os temp>/lenapay-documents).
	DocumentsDir string `env:"DOCUMENTS_DIR"`
}

// DevKYCAutoApproveDelay is the dev auto-approval delay; 0 (off) in production.
func (c Config) DevKYCAutoApproveDelay() time.Duration {
	if c.IsProduction() || c.DevKYCAutoApproveSeconds <= 0 {
		return 0
	}
	return time.Duration(c.DevKYCAutoApproveSeconds) * time.Second
}

// IsProduction reports whether the process runs in production.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Location returns the business time zone (validated by Load).
func (c Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.BusinessTimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// JWTSeed returns the decoded Ed25519 seed, or nil when none is configured (validated by Load).
func (c Config) JWTSeed() []byte {
	seed, err := decodeSeed(c.JWTPrivateKey.Reveal())
	if err != nil {
		return nil
	}
	return seed
}

// Load parses environ (nil = the process environment) and validates the result.
func Load(environ map[string]string) (Config, error) {
	opts := env.Options{}
	if environ != nil {
		opts.Environment = environ
	}
	cfg, err := env.ParseAsWithOptions[Config](opts)
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error
	fail := func(key, reason string) { errs = append(errs, fmt.Errorf("%s %s", key, reason)) }

	if !slices.Contains([]string{EnvDevelopment, EnvTest, EnvProduction}, c.Env) {
		fail("APP_ENV", "must be development, test or production")
	}
	if c.BodyLimitBytes <= 0 || c.BodyLimitBytes > 1<<20 {
		fail("BODY_LIMIT_BYTES", "must be between 1 B and 1 MiB (A12)")
	}
	if c.UploadLimitBytes <= 0 || c.UploadLimitBytes > 10<<20 {
		fail("UPLOAD_LIMIT_BYTES", "must be between 1 B and 10 MiB")
	}
	if c.DBMaxConns < 1 {
		fail("DB_MAX_CONNS", "must be at least 1")
	}
	if c.RateLimitPerMinute < 1 {
		fail("RATE_LIMIT_PER_MINUTE", "must be at least 1")
	}
	for key, d := range map[string]time.Duration{
		"REQUEST_TIMEOUT": c.RequestTimeout, "SHUTDOWN_TIMEOUT": c.ShutdownTimeout, "ACCESS_TOKEN_TTL": c.AccessTTL,
		"REFRESH_TOKEN_TTL": c.RefreshTTL, "STEP_UP_TOKEN_TTL": c.StepUpTTL, "PSP_TIMEOUT": c.PSPTimeout,
		"PSP_WEBHOOK_TOLERANCE": c.PSPWebhookTolerance,
	} {
		if d <= 0 {
			fail(key, "must be a positive duration")
		}
	}
	if _, err := time.LoadLocation(c.BusinessTimeZone); err != nil {
		fail("BUSINESS_TIME_ZONE", "must be an IANA time zone")
	}
	if _, err := decodeSeed(c.JWTPrivateKey.Reveal()); err != nil {
		fail("JWT_PRIVATE_KEY", "must be a base64 32-byte Ed25519 seed")
	}
	if len(c.InternalAPIKey.Reveal()) < 16 {
		fail("INTERNAL_API_KEY", "must be at least 16 characters")
	}
	if len(c.PSPWebhookSecret.Reveal()) < 16 {
		fail("PSP_WEBHOOK_SECRET", "must be at least 16 characters")
	}
	if u, err := url.Parse(c.PSPBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fail("PSP_BASE_URL", "must be an absolute http(s) URL")
	}
	if c.DevFixedEmailCode != "" && !isSixDigits(c.DevFixedEmailCode) {
		fail("DEV_FIXED_EMAIL_CODE", "must be empty or 6 digits")
	}
	if c.DevKYCAutoApproveSeconds < 0 {
		fail("DEV_KYC_AUTO_APPROVE_SECONDS", "must be 0 or more")
	}
	if c.PSPMode != PSPModeHTTP && c.PSPMode != PSPModeFake {
		fail("PSP_MODE", "must be http or fake")
	}
	if u, err := url.Parse(c.CVUShareBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fail("CVU_SHARE_BASE_URL", "must be an absolute http(s) URL")
	}
	if c.IsProduction() {
		errs = append(errs, c.validateProduction()...)
	}
	return errors.Join(errs...)
}

// validateProduction refuses development values (AG14).
func (c Config) validateProduction() []error {
	var errs []error
	fail := func(key, reason string) { errs = append(errs, fmt.Errorf("%s %s", key, reason)) }
	if c.JWTPrivateKey.Reveal() == "" {
		fail("JWT_PRIVATE_KEY", "is required in production")
	}
	if c.InternalAPIKey.Reveal() == devInternalAPIKey {
		fail("INTERNAL_API_KEY", "must not use the development default in production")
	}
	if c.PSPWebhookSecret.Reveal() == devPSPWebhookSecret {
		fail("PSP_WEBHOOK_SECRET", "must not use the development default in production")
	}
	if c.DevInboxEnabled {
		fail("DEV_INBOX_ENABLED", "must be false in production")
	}
	if c.DevFixedEmailCode != "" {
		fail("DEV_FIXED_EMAIL_CODE", "must be empty in production")
	}
	if c.PSPMode != PSPModeHTTP {
		fail("PSP_MODE", "must be http in production")
	}
	if slices.Contains(c.CORSOrigins, "*") {
		fail("CORS_ORIGINS", "must not contain * in production")
	}
	if u, err := url.Parse(c.PSPBaseURL); err != nil || u.Scheme != "https" || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
		fail("PSP_BASE_URL", "must be a non-local https URL in production")
	}
	return errs
}

// decodeSeed accepts std or URL-safe base64; "" decodes to nil (no key configured).
func decodeSeed(raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		b, err = base64.RawURLEncoding.DecodeString(raw)
	}
	if err != nil {
		return nil, err
	}
	if len(b) != 32 {
		return nil, errors.New("seed must be 32 bytes")
	}
	return b, nil
}

// PSP adapters (PSP_MODE).
const (
	PSPModeHTTP = "http"
	PSPModeFake = "fake"
)

func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

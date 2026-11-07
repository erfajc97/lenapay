package config

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func validDevelopment() map[string]string {
	return map[string]string{
		"APP_ENV":      "development",
		"DATABASE_URL": "postgres://lenapay:lenapay@localhost:5433/lenapay?sslmode=disable",
	}
}

func validProduction() map[string]string {
	return map[string]string{
		"APP_ENV":            "production",
		"DATABASE_URL":       "postgres://app:prod-db-password@db:5432/lenapay",
		"JWT_PRIVATE_KEY":    base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"PSP_WEBHOOK_SECRET": "prod-webhook-secret-0123456789",
		"INTERNAL_API_KEY":   "prod-internal-key-0123456789",
		"PSP_BASE_URL":       "https://psp.example.com",
		"CORS_ORIGINS":       "https://app.lenapay.example",
	}
}

func TestDevelopmentDefaultsBoot(t *testing.T) {
	t.Parallel()
	cfg, err := Load(validDevelopment())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.AccessTTL != 15*time.Minute || cfg.StepUpTTL != 5*time.Minute ||
		cfg.PSPWebhookTolerance != 5*time.Minute || cfg.BodyLimitBytes != 1<<20 || cfg.PersonalModeEnabled {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.JWTSeed() != nil {
		t.Fatal("development without a key means an ephemeral key generated at boot")
	}
	if cfg.Location().String() != "America/Argentina/Buenos_Aires" {
		t.Fatalf("location %s", cfg.Location())
	}
}

func TestProductionBootsWithEverySecret(t *testing.T) {
	t.Parallel()
	cfg, err := Load(validProduction())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.IsProduction() || len(cfg.JWTSeed()) != 32 {
		t.Fatalf("cfg %+v", cfg)
	}
}

// A8/AG14: production refuses a missing secret AND a development value, naming the key only.
func TestProductionRefusesDevelopmentValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, key, value string
	}{
		{"missing database url", "DATABASE_URL", ""},
		{"missing jwt key", "JWT_PRIVATE_KEY", ""},
		{"dev webhook secret", "PSP_WEBHOOK_SECRET", devPSPWebhookSecret},
		{"dev internal key", "INTERNAL_API_KEY", devInternalAPIKey},
		{"dev inbox on", "DEV_INBOX_ENABLED", "true"},
		{"cors wildcard", "CORS_ORIGINS", "*"},
		{"local psp", "PSP_BASE_URL", "http://localhost:8090"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			environ := validProduction()
			if c.value == "" {
				delete(environ, c.key)
			} else {
				environ[c.key] = c.value
			}
			_, err := Load(environ)
			if err == nil || !strings.Contains(err.Error(), c.key) {
				t.Fatalf("err = %v, want it to name %s", err, c.key)
			}
			for _, secret := range []string{devPSPWebhookSecret, devInternalAPIKey, "prod-db-password", "localhost"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaks a value: %v", err)
				}
			}
		})
	}
}

func TestInvalidValuesAreAllReported(t *testing.T) {
	t.Parallel()
	environ := validDevelopment()
	for k, v := range map[string]string{
		"APP_ENV":            "staging",
		"JWT_PRIVATE_KEY":    base64.StdEncoding.EncodeToString(make([]byte, 31)),
		"PSP_BASE_URL":       "bind-mock:8090",
		"BUSINESS_TIME_ZONE": "Mars/Olympus",
		"INTERNAL_API_KEY":   "short",
		"BODY_LIMIT_BYTES":   fmt.Sprint(1<<20 + 1),
		"PSP_TIMEOUT":        "-1s",
	} {
		environ[k] = v
	}
	_, err := Load(environ)
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, key := range []string{
		"APP_ENV", "JWT_PRIVATE_KEY", "PSP_BASE_URL", "BUSINESS_TIME_ZONE",
		"INTERNAL_API_KEY", "BODY_LIMIT_BYTES", "PSP_TIMEOUT",
	} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not name %s: %v", key, err)
		}
	}
}

// Boundary pair (FT1) on the JSON body cap: 1 MiB passes, one byte more fails.
func TestBodyLimitBoundary(t *testing.T) {
	t.Parallel()
	environ := validDevelopment()
	environ["BODY_LIMIT_BYTES"] = fmt.Sprint(1 << 20)
	if _, err := Load(environ); err != nil {
		t.Fatalf("1 MiB rejected: %v", err)
	}
}

func TestSecretNeverPrints(t *testing.T) {
	t.Parallel()
	s := Secret("hunter2-hunter2")
	text, _ := s.MarshalText()
	for _, out := range []string{s.String(), s.GoString(), fmt.Sprintf("%v %s %+v", s, s, s), string(text), s.LogValue().String()} {
		if strings.Contains(out, "hunter2") {
			t.Fatalf("secret printed: %q", out)
		}
	}
	if s.Reveal() != "hunter2-hunter2" {
		t.Fatal("Reveal")
	}
}

func TestSeedEncodings(t *testing.T) {
	t.Parallel()
	seed := make([]byte, 32)
	seed[0] = 0xfb // forces a URL-unsafe char in std base64
	environ := validDevelopment()
	environ["JWT_PRIVATE_KEY"] = base64.RawURLEncoding.EncodeToString(seed)
	environ["CORS_ORIGINS"] = "http://a.test,http://b.test"
	cfg, err := Load(environ)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTSeed()[0] != 0xfb || strings.Join(cfg.CORSOrigins, "|") != "http://a.test|http://b.test" {
		t.Fatalf("cfg %+v", cfg)
	}
}

// PSP_MODE picks the adapter; production never runs on the in-process fake (P2).
func TestPSPModeAndShareURL(t *testing.T) {
	t.Parallel()
	cfg, err := Load(validDevelopment())
	if err != nil || cfg.PSPMode != PSPModeHTTP || cfg.CVUShareBaseURL == "" {
		t.Fatalf("defaults %q %q %v", cfg.PSPMode, cfg.CVUShareBaseURL, err)
	}
	dev := validDevelopment()
	dev["PSP_MODE"] = PSPModeFake
	if _, err := Load(dev); err != nil {
		t.Fatalf("fake in development: %v", err)
	}
	dev["PSP_MODE"], dev["CVU_SHARE_BASE_URL"] = "bind", "lenapay.app"
	if _, err := Load(dev); err == nil || !strings.Contains(err.Error(), "PSP_MODE") || !strings.Contains(err.Error(), "CVU_SHARE_BASE_URL") {
		t.Fatalf("err = %v", err)
	}
	prod := validProduction()
	prod["PSP_MODE"] = PSPModeFake
	if _, err := Load(prod); err == nil || !strings.Contains(err.Error(), "PSP_MODE must be http in production") {
		t.Fatalf("production with the fake PSP: %v", err)
	}
}

package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := Load(func(string) string { return "" })
	if err != nil || cfg.HTTPAddr != ":8090" || !strings.HasSuffix(cfg.WebhookURL, "/webhooks/psp/transfers") {
		t.Fatalf("cfg %+v err %v", cfg, err)
	}
}

func TestInvalidValuesNameTheKeyNotTheValue(t *testing.T) {
	t.Parallel()
	env := map[string]string{"BINDMOCK_WEBHOOK_URL": "api:8080", "PSP_WEBHOOK_SECRET": "short-secret"}
	_, err := Load(func(k string) string { return env[k] })
	if err == nil || !strings.Contains(err.Error(), "BINDMOCK_WEBHOOK_URL") || !strings.Contains(err.Error(), "PSP_WEBHOOK_SECRET") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "short-secret") || strings.Contains(fmt.Sprint(Secret("short-secret")), "short") {
		t.Fatal("secret leaked")
	}
}

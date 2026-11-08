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

func TestBehaviourKnobs(t *testing.T) {
	t.Parallel()
	cfg, err := Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TimeZone.String() != "America/Argentina/Buenos_Aires" || cfg.StateFile != "" ||
		cfg.InitialBalanceMinor["ARS"] != 5_000_000_000 || cfg.AutoPayinEvery != 0 {
		t.Fatalf("defaults %+v", cfg)
	}
	bad := map[string]string{
		"BINDMOCK_PENDING_DELAY":             "soon",
		"BINDMOCK_INITIAL_BALANCE_USD_MINOR": "-1",
		"BINDMOCK_AUTO_PAYINS_EVERY":         "500ms",
		"BINDMOCK_TIME_ZONE":                 "Mars/Olympus",
	}
	_, err = Load(func(k string) string { return bad[k] })
	for key := range bad {
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s not reported: %v", key, err)
		}
	}
	ok := map[string]string{"BINDMOCK_AUTO_PAYINS_EVERY": "1s"} // the smallest accepted interval
	if cfg, err := Load(func(k string) string { return ok[k] }); err != nil || cfg.AutoPayinEvery.Seconds() != 1 {
		t.Fatalf("1s rejected: %v", err)
	}
}

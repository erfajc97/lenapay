package config

import (
	"strings"
	"testing"
	"time"
)

// DEV_FIXED_EMAIL_CODE (R-ID-1 demo): 6 digits or empty, and never in production.
func TestDevFixedEmailCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		env     map[string]string
		value   string
		wantErr bool
	}{
		{"six digits in development", validDevelopment(), "123456", false},
		{"five digits", validDevelopment(), "12345", true},
		{"seven digits", validDevelopment(), "1234567", true},
		{"letters", validDevelopment(), "12345a", true},
		{"set in production", validProduction(), "123456", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.env["DEV_FIXED_EMAIL_CODE"] = tt.value
			_, err := Load(tt.env)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && (!strings.Contains(err.Error(), "DEV_FIXED_EMAIL_CODE") || strings.Contains(err.Error(), tt.value)) {
				t.Fatalf("the error must name the key, never the value: %v", err)
			}
		})
	}
}

// DEV_KYC_AUTO_APPROVE_SECONDS defaults to 8 in development and is always off in production.
func TestDevKYCAutoApproveDelay(t *testing.T) {
	t.Parallel()
	dev, err := Load(validDevelopment())
	if err != nil || dev.DevKYCAutoApproveDelay() != 8*time.Second {
		t.Fatalf("default delay %v (%v)", dev.DevKYCAutoApproveDelay(), err)
	}
	off := validDevelopment()
	off["DEV_KYC_AUTO_APPROVE_SECONDS"] = "0"
	if cfg, err := Load(off); err != nil || cfg.DevKYCAutoApproveDelay() != 0 {
		t.Fatalf("0 must turn it off: %v %v", cfg.DevKYCAutoApproveDelay(), err)
	}
	negative := validDevelopment()
	negative["DEV_KYC_AUTO_APPROVE_SECONDS"] = "-1"
	if _, err := Load(negative); err == nil || !strings.Contains(err.Error(), "DEV_KYC_AUTO_APPROVE_SECONDS") {
		t.Fatalf("negative delay: %v", err)
	}
	prod, err := Load(validProduction())
	if err != nil || prod.DevKYCAutoApproveDelay() != 0 {
		t.Fatalf("production delay %v (%v)", prod.DevKYCAutoApproveDelay(), err)
	}
}

package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestEveryCodeHasAStatusAndMessage(t *testing.T) {
	t.Parallel()
	codes := All()
	if len(codes) != len(catalog()) {
		t.Fatalf("All() has %d codes, catalog %d", len(codes), len(catalog()))
	}
	for _, c := range codes {
		if s := c.Status(); s < 400 || s > 599 {
			t.Errorf("%s: status %d", c, s)
		}
		if c.Message() == "" {
			t.Errorf("%s: empty message", c)
		}
		if area, reason, ok := strings.Cut(string(c), "."); !ok || area == "" || reason == "" {
			t.Errorf("%s: codes are area.reason", c)
		}
	}
}

func TestUnknownCodeIsInternal(t *testing.T) {
	t.Parallel()
	if Code("no.such_code").Status() != http.StatusInternalServerError {
		t.Fatal("unknown codes must map to 500")
	}
	if CodeNotImplemented.Status() != http.StatusNotImplemented || CodeValidation.Status() != http.StatusUnprocessableEntity {
		t.Fatal("stub and validation statuses changed")
	}
}

func TestWrapKeepsCauseAndCode(t *testing.T) {
	t.Parallel()
	cause := errors.New("pg: unique violation")
	err := fmt.Errorf("create payer: %w", Wrap(CodeConflict, cause, map[string]any{"field": "docNumber"}))
	if !IsCode(err, CodeConflict) || IsCode(err, CodeInternal) {
		t.Fatalf("IsCode wrong for %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatal("cause lost")
	}
	if CodeOf(errors.New("plain")) != "" || CodeOf(NotImplemented()) != CodeNotImplemented {
		t.Fatal("CodeOf")
	}
}

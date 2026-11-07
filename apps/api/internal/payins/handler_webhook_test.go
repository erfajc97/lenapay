package payins

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/httpx"
)

type webhookStub struct {
	Service
	got WebhookDelivery
	err error
}

func (s *webhookStub) ReceiveWebhook(_ context.Context, d WebhookDelivery) error {
	s.got = d
	return s.err
}

// The handler passes the RAW body and both headers untouched, and maps the outcome.
func TestWebhookHandler(t *testing.T) {
	t.Parallel()
	body := `{"eventId":"evt_1"}  ` // trailing spaces are part of what was signed
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"stored", nil, http.StatusAccepted},
		{"bad signature", apperr.New(apperr.CodeWebhookSignature, nil), http.StatusUnauthorized},
		{"database down: the PSP redelivers", errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stub := &webhookStub{err: tc.err}
			r := httptest.NewRequest(http.MethodPost, "/webhooks/psp/transfers", strings.NewReader(body))
			r.Header.Set(headerSignature, "sha256=abc")
			r.Header.Set(headerTimestamp, "1800000000")
			rec := httptest.NewRecorder()
			NewWebhookHandler(stub).ServeHTTP(rec, r)
			if rec.Code != tc.want || string(stub.got.RawBody) != body || stub.got.Signature != "sha256=abc" || stub.got.Timestamp != "1800000000" {
				t.Fatalf("status %d delivery %+v", rec.Code, stub.got)
			}
		})
	}
	// Over the body limit: 413 before the service sees anything.
	stub := &webhookStub{}
	h := httpx.BodyLimit(16, 16)(NewWebhookHandler(stub))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/webhooks/psp/transfers", strings.NewReader(strings.Repeat("x", 17))))
	if rec.Code != http.StatusRequestEntityTooLarge || stub.got.RawBody != nil {
		t.Fatalf("over the limit: %d", rec.Code)
	}
}

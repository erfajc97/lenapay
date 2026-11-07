package payers

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// stubService answers the handler with fixed data and records the input it got.
type stubService struct {
	Service
	gotCVU CreateCVUInput
	gotP   auth.Principal
}

func (s *stubService) CreateCVU(_ context.Context, p auth.Principal, _ uuid.UUID, in CreateCVUInput) (CVU, error) {
	s.gotCVU, s.gotP = in, p
	return sampleCVU(), nil
}

func (s *stubService) Get(context.Context, auth.Principal, uuid.UUID) (Payer, error) {
	email := "ana@example.com"
	return Payer{ID: uuid.New(), AccountID: uuid.New(), Name: "Ana", DocType: DocDNI, DocNumber: "30111222", Email: &email, CVUs: []CVU{sampleCVU()}, CreatedAt: time.Now()}, nil
}

func sampleCVU() CVU {
	return CVU{ID: uuid.New(), AccountID: uuid.New(), Currency: money.ARS, Number: "0000322800000001000012", Active: true, ShareLink: "https://l/x", QRPayload: "https://l/x?currency=ARS", CreatedAt: time.Now()}
}

func keys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return slices.Sorted(maps.Keys(m))
}

// A5/FT20: responses carry exactly the contract's keys (optional nulls omitted by codegen).
func TestResponsesAreAllowlists(t *testing.T) {
	t.Parallel()
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{UserID: uuid.New(), OrgID: uuid.New()})
	h := NewHandler(&stubService{})
	res, err := h.GetPayer(ctx, api.GetPayerRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	payer := res.(api.GetPayer200JSONResponse)
	if got := strings.Join(keys(t, payer), ","); got != "accountId,createdAt,cvus,docNumber,docType,email,id,name" {
		t.Fatalf("payer keys %s", got)
	}
	if got := strings.Join(keys(t, payer.Cvus[0]), ","); got != "accountId,createdAt,currency,cvu,id,share,status" {
		t.Fatalf("cvu keys %s", got)
	}
	if payer.Cvus[0].Status != api.Active || payer.Cvus[0].Share.QrPayload != "https://l/x?currency=ARS" {
		t.Fatalf("cvu %+v", payer.Cvus[0])
	}
}

func TestCreateCvuPassesTheIdempotencyKey(t *testing.T) {
	t.Parallel()
	stub := &stubService{}
	p := auth.Principal{UserID: uuid.New(), OrgID: uuid.New(), StepUp: true}
	alias := "torre.norte"
	_, err := NewHandler(stub).CreateCvu(auth.WithPrincipal(t.Context(), p), api.CreateCvuRequestObject{
		Params: api.CreateCvuParams{IdempotencyKey: "client-key-0000000001"}, Body: &api.CreateCvuRequest{Currency: "USD", Alias: &alias},
	})
	if err != nil || stub.gotCVU.IdempotencyKey != "client-key-0000000001" || stub.gotCVU.Currency != money.USD || *stub.gotCVU.Alias != alias || stub.gotP != p {
		t.Fatalf("got %+v %v", stub.gotCVU, err)
	}
	if _, err := NewHandler(stub).CreateCvu(t.Context(), api.CreateCvuRequestObject{}); !apperr.IsCode(err, apperr.CodeUnauthenticated) {
		t.Fatalf("no principal: %v", err)
	}
}

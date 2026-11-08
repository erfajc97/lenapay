//go:build bindmock

// Contract test of the adapter against the real apps/bind-mock (a separate Go module, so it
// cannot be started from here). Run it with the mock up:
//
//	cd apps/bind-mock && go run ./cmd/bind-mock            (or: docker compose up bind-mock)
//	cd apps/api && BINDMOCK_URL=http://localhost:8090 go test -tags=bindmock ./internal/psp/provider/bind/
package bind_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
	"lenapay/apps/api/internal/psp/provider/bind"
)

func TestAgainstBindMock(t *testing.T) {
	base := os.Getenv("BINDMOCK_URL")
	if base == "" {
		t.Skip("BINDMOCK_URL not set")
	}
	ctx := t.Context()
	c := bind.New(bind.Options{BaseURL: base, Timeout: 5 * time.Second})
	ref := uuid.NewString()

	first, err := c.CreateCVU(ctx, psp.CreateCVURequest{Reference: ref, Currency: money.ARS, HolderName: "Juan Pérez", HolderDoc: "20123456786"})
	if err != nil || len(first.CVU) != 22 {
		t.Fatalf("create: %+v %v", first, err)
	}
	again, err := c.CreateCVU(ctx, psp.CreateCVURequest{Reference: ref, Currency: money.ARS, HolderName: "Juan Pérez"})
	if err != nil || again != first {
		t.Fatalf("same reference must return the same CVU: %+v vs %+v (%v)", again, first, err)
	}
	alias := "c" + ref[:8]
	if err := c.UpdateAlias(ctx, first.ProviderRef, alias); err != nil {
		t.Fatalf("alias: %v", err)
	}
	other, err := c.CreateCVU(ctx, psp.CreateCVURequest{Reference: uuid.NewString(), Currency: money.USD, HolderName: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateAlias(ctx, other.ProviderRef, alias); !errors.Is(err, psp.ErrAliasTaken) {
		t.Fatalf("taken alias err %v", err)
	}
	if d, err := c.ResolveDestination(ctx, alias); err != nil || d.AccountNumber != first.CVU || d.HolderName != "Juan Pérez" {
		t.Fatalf("resolve own alias: %+v %v", d, err)
	}

	// A payer pays the CVU through the control endpoint; the adapter re-reads it.
	body, _ := json.Marshal(map[string]string{"cvu": first.CVU, "amount": "1234.56", "originName": "Ana Gómez", "originCuit": "27111111110"})
	resp, err := http.Post(base+"/_mock/incoming-transfer", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var incoming struct {
		Transfer struct {
			ID string `json:"id"`
		} `json:"transfer"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&incoming)
	resp.Body.Close()
	got, err := c.GetTransfer(ctx, incoming.Transfer.ID)
	if err != nil || got.CVU != first.CVU || got.Amount != money.New(123456, money.ARS) || got.OriginName != "Ana Gómez" {
		t.Fatalf("get transfer: %+v %v", got, err)
	}

	attempt := uuid.NewString()
	sent, err := c.SendTransfer(ctx, psp.SendTransferRequest{Reference: attempt, AccountNumber: first.CVU, Amount: money.New(100, money.ARS), Concept: "test"})
	if err != nil || sent.Status != psp.TransferSettled {
		t.Fatalf("send: %+v %v", sent, err)
	}
	if byRef, err := c.TransferByReference(ctx, attempt); err != nil || byRef.ProviderID != sent.ProviderID {
		t.Fatalf("by reference: %+v %v", byRef, err)
	}
	if _, err := c.TransferByReference(ctx, uuid.NewString()); !errors.Is(err, psp.ErrNotFound) {
		t.Fatalf("unknown reference err %v", err)
	}
	if _, err := c.SendTransfer(ctx, psp.SendTransferRequest{Reference: uuid.NewString(), AccountNumber: first.CVU, Amount: money.New(1099, money.ARS)}); !errors.Is(err, psp.ErrUncertain) {
		t.Fatalf(".99 must be uncertain: %v", err)
	}
	lines, err := c.Statement(ctx, money.ARS, time.Now().In(mustAR(t)))
	if err != nil || len(lines) == 0 {
		t.Fatalf("statement: %d %v", len(lines), err)
	}
	if err := c.DisableCVU(ctx, first.ProviderRef); err != nil {
		t.Fatal(err)
	}
	if err := c.DisableCVU(ctx, "caja-unknown"); err != nil {
		t.Fatalf("disable unknown must be a no-op: %v", err)
	}
}

func mustAR(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

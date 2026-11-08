package fake_test

import (
	"errors"
	"testing"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
	"lenapay/apps/api/internal/psp/provider/fake"
)

// The fake keeps the provider's semantics that callers rely on: idempotency by reference,
// unique aliases, pending until completed, scripted failures.
func TestFakeSemantics(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	p := fake.New()
	first, err := p.CreateCVU(ctx, psp.CreateCVURequest{Reference: "r1", Currency: money.ARS, HolderName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := p.CreateCVU(ctx, psp.CreateCVURequest{Reference: "r1", Currency: money.ARS}); again != first || len(p.CVUs()) != 1 {
		t.Fatal("same reference must return the same CVU")
	}
	if _, err := p.CreateCVU(ctx, psp.CreateCVURequest{Reference: "r2", Alias: &first.Alias}); !errors.Is(err, psp.ErrAliasTaken) || !errors.Is(err, psp.ErrRejected) {
		t.Fatalf("alias err %v", err)
	}
	p.FailNext(psp.ErrUncertain)
	if err := p.UpdateAlias(ctx, first.ProviderRef, "nuevo.alias"); !errors.Is(err, psp.ErrUncertain) {
		t.Fatalf("scripted err %v", err)
	}
	if err := p.DisableCVU(ctx, first.ProviderRef); err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateAlias(ctx, first.ProviderRef, "nuevo.alias"); !errors.Is(err, psp.ErrRejected) {
		t.Fatalf("disabled err %v", err)
	}
	res, _ := p.SendTransfer(ctx, psp.SendTransferRequest{Reference: "a1", Amount: money.New(100, money.ARS)})
	if res.Status != psp.TransferPending {
		t.Fatalf("send %+v", res)
	}
	if err := p.Complete("a1", psp.TransferSettled); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.SendTransfer(ctx, psp.SendTransferRequest{Reference: "a1"}); got.ProviderID != res.ProviderID || got.Status != psp.TransferSettled {
		t.Fatalf("resend %+v", got)
	}
	if _, err := p.TransferByReference(ctx, "nope"); !errors.Is(err, psp.ErrNotFound) {
		t.Fatal("unknown reference")
	}
	if _, err := p.GetTransfer(ctx, "trf-x"); !errors.Is(err, psp.ErrNotFound) {
		t.Fatal("unknown incoming")
	}
	if _, err := p.ResolveDestination(ctx, "noexiste.x"); !errors.Is(err, psp.ErrNotFound) {
		t.Fatal("noexiste alias")
	}
	if p.Calls() != 11 {
		t.Fatalf("calls %d", p.Calls())
	}
}

package accounts

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

type fakeService struct {
	Service
	transfer TransferInput
	account  Account
}

func (f *fakeService) Summary(context.Context, auth.Principal) (Summary, error) {
	return Summary{Totals: f.account.Balances, Accounts: []Account{f.account}}, nil
}

func (f *fakeService) List(context.Context, auth.Principal, bool) ([]Account, error) {
	return []Account{f.account}, nil
}

func (f *fakeService) Transfer(_ context.Context, _ auth.Principal, in TransferInput) (Transfer, error) {
	f.transfer = in
	return Transfer{ID: ids.New(), FromAccountID: in.FromAccountID, ToAccountID: in.ToAccountID, Amount: in.Amount, CreatedAt: time.Unix(0, 0).UTC()}, nil
}

func keys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return slices.Sorted(maps.Keys(m))
}

// Responses carry exactly the contract's keys (A5, FT20) and the transfer passes the
// Idempotency-Key header and the concept through to the service.
func TestHandlerMapping(t *testing.T) {
	t.Parallel()
	bal := CurrencyBalance{Posted: money.New(10, money.ARS), Held: money.New(4, money.ARS), Available: money.New(6, money.ARS)}
	svc := &fakeService{account: Account{ID: ids.New(), Kind: KindProject, Name: "Torre", Balances: []CurrencyBalance{bal}, CreatedAt: time.Unix(0, 0).UTC()}}
	h := NewHandler(svc)
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{UserID: ids.New(), OrgID: ids.New(), StepUp: true})

	list, err := h.ListAccounts(ctx, api.ListAccountsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	acc := list.(api.ListAccounts200JSONResponse).Items[0]
	if got, want := keys(t, acc), []string{"archived", "balances", "createdAt", "id", "kind", "name"}; !slices.Equal(got, want) {
		t.Fatalf("account keys = %v, want %v", got, want)
	}
	if got, want := keys(t, acc.Balances[0]), []string{"available", "currency", "held", "posted"}; !slices.Equal(got, want) {
		t.Fatalf("balance keys = %v, want %v", got, want)
	}
	if acc.Balances[0].Available.Minor != 6 || acc.Balances[0].Currency != "ARS" {
		t.Fatalf("balance = %+v", acc.Balances[0])
	}

	sum, err := h.GetBalances(ctx, api.GetBalancesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keys(t, sum), []string{"accounts", "totals"}; !slices.Equal(got, want) {
		t.Fatalf("balances keys = %v, want %v", got, want)
	}
	if got, want := keys(t, sum.(api.GetBalances200JSONResponse).Accounts[0]), []string{"accountId", "balances", "kind", "name"}; !slices.Equal(got, want) {
		t.Fatalf("account balances keys = %v, want %v", got, want)
	}

	concept := "Aporte"
	from, to := ids.New(), ids.New()
	res, err := h.CreateInternalTransfer(ctx, api.CreateInternalTransferRequestObject{
		Params: api.CreateInternalTransferParams{IdempotencyKey: "handler-key-000000001"},
		Body:   &api.InternalTransferRequest{FromAccountId: from, ToAccountId: to, Amount: api.Money{Minor: 250, Currency: "USD"}, Concept: &concept},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := TransferInput{FromAccountID: from, ToAccountID: to, Amount: money.New(250, money.USD), Concept: "Aporte", IdempotencyKey: "handler-key-000000001"}
	if svc.transfer != want {
		t.Fatalf("service got %+v, want %+v", svc.transfer, want)
	}
	if got, want := keys(t, res), []string{"amount", "createdAt", "fromAccountId", "id", "toAccountId"}; !slices.Equal(got, want) {
		t.Fatalf("transfer keys = %v, want %v", got, want)
	}
}

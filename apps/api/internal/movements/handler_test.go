package movements

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

type fakeService struct {
	got   Filter
	items []Movement
}

func (f *fakeService) Get(_ context.Context, _ auth.Principal, id uuid.UUID) (Movement, error) {
	for _, m := range f.items {
		if m.ID == id {
			return m, nil
		}
	}
	return Movement{}, apperr.New(apperr.CodeMovementNotFound, nil)
}

func (f *fakeService) List(_ context.Context, _ auth.Principal, filter Filter) (page.Page[Movement], error) {
	f.got = filter
	next := "next"
	return page.Page[Movement]{Items: f.items, NextCursor: &next}, nil
}

// The handler passes every filter through and answers exactly the contract's keys (FT20).
func TestListMovementsHandler(t *testing.T) {
	t.Parallel()
	name, fee := "Comprador", money.New(5, money.ARS)
	svc := &fakeService{items: []Movement{
		{
			ID: ids.New(), Type: TypePayin, Status: StatusCompleted, Direction: DirectionIn, AccountID: ids.New(),
			Amount: money.New(100, money.ARS), Fee: &fee, Counterparty: Counterparty{Name: &name}, RefID: ids.New(), CreatedAt: time.Unix(0, 0).UTC(),
		},
		{
			ID: ids.New(), Type: TypeInternalTransfer, Status: StatusCompleted, Direction: DirectionOut, AccountID: ids.New(),
			Amount: money.New(7, money.USD), RefID: ids.New(), CreatedAt: time.Unix(0, 0).UTC(),
		},
	}}
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{UserID: ids.New(), OrgID: ids.New()})
	account, typ, status, currency, limit := ids.New(), api.MovementType("payout"), api.MovementStatus("failed"), api.Currency("USD"), 5
	res, err := NewHandler(svc).ListMovements(ctx, api.ListMovementsRequestObject{Params: api.ListMovementsParams{
		Account: &account, Type: &typ, Status: &status, Currency: &currency, Limit: &limit,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if *svc.got.AccountID != account || *svc.got.Type != TypePayout || *svc.got.Status != StatusFailed ||
		*svc.got.Currency != money.USD || svc.got.Page.Limit != 5 {
		t.Fatalf("filter = %+v", svc.got)
	}
	body, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items      []map[string]any `json:"items"`
		NextCursor *string          `json:"nextCursor"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	// Nullable members are always present (null), never omitted.
	full := []string{"accountId", "amount", "concept", "counterparty", "createdAt", "direction", "fee", "id", "refId", "status", "type"}
	for i := range decoded.Items {
		if got := slices.Sorted(maps.Keys(decoded.Items[i])); !slices.Equal(got, full) {
			t.Fatalf("item %d keys = %v, want %v", i, got, full)
		}
	}
	if decoded.Items[1]["fee"] != nil || decoded.Items[1]["concept"] != nil {
		t.Fatalf("bare item = %v", decoded.Items[1])
	}
	if cp, ok := decoded.Items[1]["counterparty"].(map[string]any); !ok || len(cp) != 3 || cp["name"] != nil {
		t.Fatalf("bare counterparty = %v", decoded.Items[1]["counterparty"])
	}
	if decoded.NextCursor == nil || *decoded.NextCursor != "next" {
		t.Fatalf("nextCursor = %v", decoded.NextCursor)
	}
}

// GetMovement returns the one movement, and the service's not-found code passes through.
func TestGetMovementHandler(t *testing.T) {
	t.Parallel()
	m := Movement{
		ID: ids.New(), Type: TypePayout, Status: StatusPending, Direction: DirectionOut, AccountID: ids.New(),
		Amount: money.New(100, money.ARS), RefID: ids.New(), CreatedAt: time.Unix(0, 0).UTC(),
	}
	h := NewHandler(&fakeService{items: []Movement{m}})
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{UserID: ids.New(), OrgID: ids.New()})
	res, err := h.GetMovement(ctx, api.GetMovementRequestObject{MovementId: m.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.(api.GetMovement200JSONResponse); got.Id != m.ID || got.RefId != m.RefID || got.Type != api.MovementType("payout") {
		t.Fatalf("movement = %+v", got)
	}
	_, err = h.GetMovement(ctx, api.GetMovementRequestObject{MovementId: ids.New()})
	if !apperr.IsCode(err, apperr.CodeMovementNotFound) {
		t.Fatalf("err = %v", err)
	}
}

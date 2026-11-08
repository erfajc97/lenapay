package accounts

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
)

// Handler implements the accounts operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). The org comes from the
// principal the guard resolved for /orgs/{orgId} (T3), never from the body.
type Handler struct {
	svc Service
}

// NewHandler returns the accounts handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ListAccounts implements api.StrictServerInterface.
func (h *Handler) ListAccounts(ctx context.Context, req api.ListAccountsRequestObject) (api.ListAccountsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := h.svc.List(ctx, p, req.Params.IncludeArchived != nil && *req.Params.IncludeArchived)
	if err != nil {
		return nil, err
	}
	items := make([]api.Account, 0, len(accounts))
	for _, a := range accounts {
		items = append(items, toAPIAccount(a))
	}
	return api.ListAccounts200JSONResponse{Items: items}, nil
}

// CreateAccount implements api.StrictServerInterface.
func (h *Handler) CreateAccount(ctx context.Context, req api.CreateAccountRequestObject) (api.CreateAccountResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := h.svc.Create(ctx, p, CreateInput{Name: req.Body.Name})
	if err != nil {
		return nil, err
	}
	return api.CreateAccount201JSONResponse(toAPIAccount(acc)), nil
}

// UpdateAccount implements api.StrictServerInterface.
func (h *Handler) UpdateAccount(ctx context.Context, req api.UpdateAccountRequestObject) (api.UpdateAccountResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	acc, err := h.svc.Update(ctx, p, req.AccountId, UpdateInput{Name: req.Body.Name, Archived: req.Body.Archived})
	if err != nil {
		return nil, err
	}
	return api.UpdateAccount200JSONResponse(toAPIAccount(acc)), nil
}

// GetBalances implements api.StrictServerInterface.
func (h *Handler) GetBalances(ctx context.Context, _ api.GetBalancesRequestObject) (api.GetBalancesResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sum, err := h.svc.Summary(ctx, p)
	if err != nil {
		return nil, err
	}
	out := api.GetBalances200JSONResponse{
		Totals:   toAPIBalances(sum.Totals),
		Accounts: make([]api.AccountBalances, 0, len(sum.Accounts)),
	}
	for _, a := range sum.Accounts {
		out.Accounts = append(out.Accounts, api.AccountBalances{
			AccountId: a.ID, Kind: api.AccountKind(a.Kind), Name: a.Name, Balances: toAPIBalances(a.Balances),
		})
	}
	return out, nil
}

// CreateInternalTransfer implements api.StrictServerInterface.
func (h *Handler) CreateInternalTransfer(ctx context.Context, req api.CreateInternalTransferRequestObject) (api.CreateInternalTransferResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	in := TransferInput{
		FromAccountID:  req.Body.FromAccountId,
		ToAccountID:    req.Body.ToAccountId,
		Amount:         money.New(req.Body.Amount.Minor, money.Currency(req.Body.Amount.Currency)),
		IdempotencyKey: req.Params.IdempotencyKey,
	}
	if req.Body.Concept != nil {
		in.Concept = *req.Body.Concept
	}
	t, err := h.svc.Transfer(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.CreateInternalTransfer201JSONResponse{
		Id: t.ID, FromAccountId: t.FromAccountID, ToAccountId: t.ToAccountID,
		Amount: toAPIMoney(t.Amount), Concept: t.Concept, CreatedAt: t.CreatedAt,
	}, nil
}

func toAPIAccount(a Account) api.Account {
	cvus := make([]api.AccountCvu, 0, len(a.CVUs))
	for _, c := range a.CVUs {
		cvus = append(cvus, ToAPICVU(c))
	}
	return api.Account{
		Id: a.ID, Kind: api.AccountKind(a.Kind), Name: a.Name, Archived: a.Archived,
		ErpProjectRef: a.ERPProjectRef, Balances: toAPIBalances(a.Balances), Cvus: cvus, CreatedAt: a.CreatedAt,
	}
}

// ToAPICVU maps an account's own CVU to the contract (also the updateAccountCvu answer).
func ToAPICVU(c ReceivingCVU) api.AccountCvu {
	status := api.Disabled
	if c.Active {
		status = api.Active
	}
	return api.AccountCvu{
		Id: c.ID, Currency: api.Currency(c.Currency), Cvu: c.Number, Alias: c.Alias, Status: status,
		Share: api.CvuShare{Link: c.ShareLink, QrPayload: c.QRPayload},
	}
}

func toAPIBalances(in []CurrencyBalance) []api.CurrencyBalance {
	out := make([]api.CurrencyBalance, 0, len(in))
	for _, b := range in {
		out = append(out, api.CurrencyBalance{
			Currency: api.Currency(b.Posted.Currency),
			Posted:   toAPIMoney(b.Posted), Held: toAPIMoney(b.Held), Available: toAPIMoney(b.Available),
		})
	}
	return out
}

func toAPIMoney(m money.Amount) api.Money {
	return api.Money{Minor: m.Minor, Currency: api.Currency(m.Currency)}
}

package payins

import (
	"context"

	"lenapay/apps/api/internal/api"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
)

// Handler implements the payins operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the payins handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// GetPayin implements api.StrictServerInterface.
func (h *Handler) GetPayin(ctx context.Context, req api.GetPayinRequestObject) (api.GetPayinResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	payin, err := h.svc.Get(ctx, p, req.PayinId)
	if err != nil {
		return nil, err
	}
	return api.GetPayin200JSONResponse{
		Id: payin.ID, AccountId: payin.AccountID, PayerId: payin.PayerID, CvuId: payin.CVUID,
		Gross: toMoney(payin.Gross), Fee: toMoney(payin.Fee), Net: toMoney(payin.Net),
		Origin:             api.Counterparty{Name: payin.Origin.Name, Cuit: payin.Origin.CUIT, Cbu: payin.Origin.CBU},
		ProviderTransferId: payin.ProviderTransferID, Status: api.PayinStatus(payin.Status),
		CreatedAt: payin.CreatedAt, CreditedAt: payin.CreditedAt,
	}, nil
}

func toMoney(a money.Amount) api.Money {
	return api.Money{Minor: a.Minor, Currency: api.Currency(a.Currency)}
}

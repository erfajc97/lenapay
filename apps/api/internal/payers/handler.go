package payers

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Handler implements the payers operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the payers handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ListPayers implements api.StrictServerInterface.
func (h *Handler) ListPayers(ctx context.Context, req api.ListPayersRequestObject) (api.ListPayersResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pageReq, err := page.ParseRequest(req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	result, err := h.svc.List(ctx, p, ListInput{AccountID: req.Params.AccountId, Query: req.Params.Q, Page: pageReq})
	if err != nil {
		return nil, err
	}
	items := make([]api.Payer, 0, len(result.Items))
	for _, payer := range result.Items {
		items = append(items, toAPIPayer(payer))
	}
	return api.ListPayers200JSONResponse{Items: items, NextCursor: result.NextCursor}, nil
}

// CreatePayer implements api.StrictServerInterface.
func (h *Handler) CreatePayer(ctx context.Context, req api.CreatePayerRequestObject) (api.CreatePayerResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	payer, err := h.svc.Create(ctx, p, CreateInput{
		AccountID: req.Body.AccountId, Name: req.Body.Name, DocType: DocType(req.Body.DocType),
		DocNumber: req.Body.DocNumber, Email: (*string)(req.Body.Email),
	})
	if err != nil {
		return nil, err
	}
	return api.CreatePayer201JSONResponse(toAPIPayer(payer)), nil
}

// GetPayer implements api.StrictServerInterface.
func (h *Handler) GetPayer(ctx context.Context, req api.GetPayerRequestObject) (api.GetPayerResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	payer, err := h.svc.Get(ctx, p, req.PayerId)
	if err != nil {
		return nil, err
	}
	return api.GetPayer200JSONResponse(toAPIPayer(payer)), nil
}

// UpdatePayer implements api.StrictServerInterface.
func (h *Handler) UpdatePayer(ctx context.Context, req api.UpdatePayerRequestObject) (api.UpdatePayerResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	payer, err := h.svc.Update(ctx, p, req.PayerId, UpdateInput{Name: req.Body.Name, Email: (*string)(req.Body.Email)})
	if err != nil {
		return nil, err
	}
	return api.UpdatePayer200JSONResponse(toAPIPayer(payer)), nil
}

// CreateCvu implements api.StrictServerInterface.
func (h *Handler) CreateCvu(ctx context.Context, req api.CreateCvuRequestObject) (api.CreateCvuResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	cvu, err := h.svc.CreateCVU(ctx, p, req.PayerId, CreateCVUInput{
		Currency: money.Currency(req.Body.Currency), Alias: req.Body.Alias, IdempotencyKey: req.Params.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return api.CreateCvu201JSONResponse(toAPICVU(cvu)), nil
}

// UpdateCvu implements api.StrictServerInterface.
func (h *Handler) UpdateCvu(ctx context.Context, req api.UpdateCvuRequestObject) (api.UpdateCvuResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	cvu, err := h.svc.UpdateCVUAlias(ctx, p, req.CvuId, req.Body.Alias)
	if err != nil {
		return nil, err
	}
	return api.UpdateCvu200JSONResponse(toAPICVU(cvu)), nil
}

// DisableCvu implements api.StrictServerInterface.
func (h *Handler) DisableCvu(ctx context.Context, req api.DisableCvuRequestObject) (api.DisableCvuResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DisableCVU(ctx, p, req.CvuId); err != nil {
		return nil, err
	}
	return api.DisableCvu204Response{}, nil
}

// toAPIPayer builds the contract's Payer field by field (A5).
func toAPIPayer(p Payer) api.Payer {
	cvus := make([]api.Cvu, 0, len(p.CVUs))
	for _, c := range p.CVUs {
		cvus = append(cvus, toAPICVU(c))
	}
	return api.Payer{
		Id: p.ID, AccountId: p.AccountID, Name: p.Name, DocType: api.DocType(p.DocType), DocNumber: p.DocNumber,
		Email: (*openapi_types.Email)(p.Email), ErpRef: p.ERPRef, Cvus: cvus, CreatedAt: p.CreatedAt,
	}
}

func toAPICVU(c CVU) api.Cvu {
	status := api.Disabled
	if c.Active {
		status = api.Active
	}
	return api.Cvu{
		Id: c.ID, PayerId: c.PayerID, AccountId: c.AccountID, Currency: api.Currency(c.Currency), Cvu: c.Number,
		Alias: c.Alias, Status: status, Share: api.CvuShare{Link: c.ShareLink, QrPayload: c.QRPayload}, CreatedAt: c.CreatedAt,
	}
}

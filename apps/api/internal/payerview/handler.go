package payerview

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/erplink"
	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Handler implements the "Mis pagos" operations of api.StrictServerInterface (HTTP only:
// generated request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the payer view handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ListMyPaymentCompanies implements api.StrictServerInterface.
func (h *Handler) ListMyPaymentCompanies(ctx context.Context, req api.ListMyPaymentCompaniesRequestObject) (api.ListMyPaymentCompaniesResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	includeHidden := req.Params.IncludeHidden != nil && *req.Params.IncludeHidden
	list, err := h.svc.ListCompanies(ctx, p, includeHidden)
	if err != nil {
		return nil, err
	}
	items := make([]api.MyPaymentCompany, 0, len(list))
	for _, c := range list {
		items = append(items, companyToAPI(c))
	}
	return api.ListMyPaymentCompanies200JSONResponse{Items: items}, nil
}

// UpdateMyPaymentCompany implements api.StrictServerInterface.
func (h *Handler) UpdateMyPaymentCompany(ctx context.Context, req api.UpdateMyPaymentCompanyRequestObject) (api.UpdateMyPaymentCompanyResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.svc.SetHidden(ctx, p, req.LinkId, req.Body.Hidden)
	if err != nil {
		return nil, err
	}
	return api.UpdateMyPaymentCompany200JSONResponse(companyToAPI(c)), nil
}

// ListMyPaymentCompanyPayins implements api.StrictServerInterface.
func (h *Handler) ListMyPaymentCompanyPayins(ctx context.Context, req api.ListMyPaymentCompanyPayinsRequestObject) (api.ListMyPaymentCompanyPayinsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pageReq, err := page.ParseRequest(req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	pg, err := h.svc.ListPayins(ctx, p, req.LinkId, pageReq)
	if err != nil {
		return nil, err
	}
	items := make([]api.MyPaymentPayin, 0, len(pg.Items))
	for _, pi := range pg.Items {
		items = append(items, api.MyPaymentPayin{
			Id: pi.ID, Cvu: pi.CVU, Amount: moneyToAPI(pi.Amount), Status: api.PayinStatus(pi.Status),
			CreatedAt: pi.CreatedAt, CreditedAt: pi.CreditedAt,
		})
	}
	return api.ListMyPaymentCompanyPayins200JSONResponse{Items: items, NextCursor: pg.NextCursor}, nil
}

// ListMyPaymentCompanyInstallments implements api.StrictServerInterface.
func (h *Handler) ListMyPaymentCompanyInstallments(ctx context.Context, req api.ListMyPaymentCompanyInstallmentsRequestObject) (api.ListMyPaymentCompanyInstallmentsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	plan, err := h.svc.Installments(ctx, p, req.LinkId)
	if err != nil {
		return nil, err
	}
	items := make([]api.MyPaymentInstallment, 0, len(plan.Items))
	for _, in := range plan.Items {
		items = append(items, installmentToAPI(in))
	}
	return api.ListMyPaymentCompanyInstallments200JSONResponse{ErpLinked: plan.ERPLinked, Items: items}, nil
}

// PayMyPaymentCompany implements api.StrictServerInterface.
func (h *Handler) PayMyPaymentCompany(ctx context.Context, req api.PayMyPaymentCompanyRequestObject) (api.PayMyPaymentCompanyResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Pay(ctx, p, req.LinkId, PayInput{
		Amount:            money.New(req.Body.Amount.Minor, money.Currency(req.Body.Amount.Currency)),
		InstallmentNumber: req.Body.InstallmentNumber, Concept: req.Body.Concept, IdempotencyKey: req.Params.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return api.PayMyPaymentCompany201JSONResponse(payouts.PayoutToAPI(po)), nil
}

// companyToAPI builds the contract's MyPaymentCompany field by field (A5: the allowlist).
func companyToAPI(c Company) api.MyPaymentCompany {
	cvus := make([]api.MyPaymentCvu, 0, len(c.CVUs))
	for _, cv := range c.CVUs {
		cvus = append(cvus, api.MyPaymentCvu{Currency: api.Currency(cv.Currency), Cvu: cv.Number, Alias: cv.Alias})
	}
	totals := make([]api.Money, 0, len(c.TotalPaid))
	for _, t := range c.TotalPaid {
		totals = append(totals, moneyToAPI(t))
	}
	return api.MyPaymentCompany{
		LinkId: c.LinkID, CompanyName: c.CompanyName, ProjectName: c.ProjectName, Cvus: cvus, TotalPaid: totals,
		LastPaymentAt: c.LastPaymentAt, ErpLinked: c.ERPLinked, Hidden: c.Hidden,
	}
}

func installmentToAPI(in erplink.Installment) api.MyPaymentInstallment {
	out := api.MyPaymentInstallment{
		Number: in.Number, DueDate: openapi_types.Date{Time: in.DueDate}, Amount: moneyToAPI(in.Amount),
		Status: api.InstallmentStatus(in.Status),
	}
	if in.AdjustmentIndex != nil {
		idx := api.MyPaymentInstallmentAdjustmentIndex(*in.AdjustmentIndex)
		out.AdjustmentIndex = &idx
	}
	return out
}

func moneyToAPI(a money.Amount) api.Money {
	return api.Money{Minor: a.Minor, Currency: api.Currency(a.Currency)}
}

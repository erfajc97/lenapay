package reconciliation

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Handler implements the reconciliation operations of api.StrictServerInterface (HTTP only:
// generated request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the reconciliation handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ListReconciliationRuns implements api.StrictServerInterface.
func (h *Handler) ListReconciliationRuns(ctx context.Context, req api.ListReconciliationRunsRequestObject) (api.ListReconciliationRunsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pr, err := page.ParseRequest(req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	var currency *money.Currency
	if req.Params.Currency != nil {
		c := money.Currency(*req.Params.Currency)
		currency = &c
	}
	pg, err := h.svc.ListRuns(ctx, p, currency, pr)
	if err != nil {
		return nil, err
	}
	items := make([]api.ReconciliationRun, 0, len(pg.Items))
	for _, r := range pg.Items {
		items = append(items, runToAPI(r))
	}
	return api.ListReconciliationRuns200JSONResponse{Items: items, NextCursor: pg.NextCursor}, nil
}

// RunReconciliation implements api.StrictServerInterface (internal ops run over the whole
// master account).
func (h *Handler) RunReconciliation(ctx context.Context, req api.RunReconciliationRequestObject) (api.RunReconciliationResponseObject, error) {
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	run, err := h.svc.RequestRun(ctx, RunInput{Currency: money.Currency(req.Body.Currency), Day: req.Body.Day.Time})
	if err != nil {
		return nil, err
	}
	return api.RunReconciliation202JSONResponse(runToAPI(run)), nil
}

func moneyToAPI(a money.Amount) api.Money {
	return api.Money{Minor: a.Minor, Currency: api.Currency(a.Currency)}
}

func runToAPI(r Run) api.ReconciliationRun {
	return api.ReconciliationRun{
		Id: r.ID, Currency: api.Currency(r.Currency), Day: openapi_types.Date{Time: r.Day},
		Status: api.ReconciliationRunStatus(r.Status), MatchedCount: r.MatchedCount,
		MissingInLedgerCount: r.MissingInLedgerCount, MissingAtPspCount: r.MissingAtPSPCount,
		PspTotal: moneyToAPI(r.PSPTotal), LedgerTotal: moneyToAPI(r.LedgerTotal), Difference: moneyToAPI(r.Difference),
		CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt,
	}
}

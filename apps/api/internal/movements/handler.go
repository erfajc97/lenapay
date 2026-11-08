package movements

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
)

// Handler implements the movements operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the movements handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ListMovements implements api.StrictServerInterface.
func (h *Handler) ListMovements(ctx context.Context, req api.ListMovementsRequestObject) (api.ListMovementsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pg, err := page.ParseRequest(req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	f := Filter{AccountID: req.Params.Account, Page: pg}
	if req.Params.Type != nil {
		t := Type(*req.Params.Type)
		f.Type = &t
	}
	if req.Params.Status != nil {
		s := Status(*req.Params.Status)
		f.Status = &s
	}
	if req.Params.Currency != nil {
		c := money.Currency(*req.Params.Currency)
		f.Currency = &c
	}
	if req.Params.From != nil {
		f.From = &req.Params.From.Time
	}
	if req.Params.To != nil {
		f.To = &req.Params.To.Time
	}
	result, err := h.svc.List(ctx, p, f)
	if err != nil {
		return nil, err
	}
	items := make([]api.Movement, 0, len(result.Items))
	for _, m := range result.Items {
		items = append(items, toAPIMovement(m))
	}
	return api.ListMovements200JSONResponse{Items: items, NextCursor: result.NextCursor}, nil
}

// GetMovement implements api.StrictServerInterface.
func (h *Handler) GetMovement(ctx context.Context, req api.GetMovementRequestObject) (api.GetMovementResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.Get(ctx, p, req.MovementId)
	if err != nil {
		return nil, err
	}
	return api.GetMovement200JSONResponse(toAPIMovement(m)), nil
}

func toAPIMovement(m Movement) api.Movement {
	out := api.Movement{
		Id: m.ID, Type: api.MovementType(m.Type), Status: api.MovementStatus(m.Status),
		Direction: api.MovementDirection(m.Direction), AccountId: m.AccountID,
		Amount:  api.Money{Minor: m.Amount.Minor, Currency: api.Currency(m.Amount.Currency)},
		Concept: m.Concept, RefId: m.RefID, Rail: api.Rail(m.Rail), CreatedAt: m.CreatedAt,
		Counterparty: api.Counterparty{Name: m.Counterparty.Name, Cuit: m.Counterparty.CUIT, Cbu: m.Counterparty.CBU},
	}
	if m.Fee != nil {
		out.Fee = &api.Money{Minor: m.Fee.Minor, Currency: api.Currency(m.Fee.Currency)}
	}
	return out
}

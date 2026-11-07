package payouts

import (
	"bytes"
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/money"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/page"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Handler implements the payouts operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). The org comes from the
// guard's principal (T3).
type Handler struct {
	svc Service
}

// NewHandler returns the payouts handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ResolveDestination implements api.StrictServerInterface.
func (h *Handler) ResolveDestination(ctx context.Context, req api.ResolveDestinationRequestObject) (api.ResolveDestinationResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	d, err := h.svc.ResolveDestination(ctx, p, req.Params.To)
	if err != nil {
		return nil, err
	}
	return api.ResolveDestination200JSONResponse(destinationToAPI(d)), nil
}

// QuotePayout implements api.StrictServerInterface.
func (h *Handler) QuotePayout(ctx context.Context, req api.QuotePayoutRequestObject) (api.QuotePayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	q, err := h.svc.Quote(ctx, p, QuoteInput{AccountID: req.Body.AccountId, Amount: moneyFromAPI(req.Body.Amount), To: req.Body.To})
	if err != nil {
		return nil, err
	}
	return api.QuotePayout200JSONResponse{
		Amount: moneyToAPI(q.Amount), Fee: moneyToAPI(q.Fee), Total: moneyToAPI(q.Total),
		Destination: destinationToAPI(q.Destination), AvailableAfter: moneyToAPI(q.AvailableAfter),
	}, nil
}

// CreatePayout implements api.StrictServerInterface.
func (h *Handler) CreatePayout(ctx context.Context, req api.CreatePayoutRequestObject) (api.CreatePayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	po, err := h.svc.Create(ctx, p, CreateInput{
		AccountID: req.Body.AccountId, Amount: moneyFromAPI(req.Body.Amount), To: req.Body.To,
		Concept: req.Body.Concept, IdempotencyKey: req.Params.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return api.CreatePayout201JSONResponse(payoutToAPI(po)), nil
}

// GetPayout implements api.StrictServerInterface.
func (h *Handler) GetPayout(ctx context.Context, req api.GetPayoutRequestObject) (api.GetPayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Get(ctx, p, req.PayoutId)
	if err != nil {
		return nil, err
	}
	return api.GetPayout200JSONResponse(payoutToAPI(po)), nil
}

// ApprovePayout implements api.StrictServerInterface.
func (h *Handler) ApprovePayout(ctx context.Context, req api.ApprovePayoutRequestObject) (api.ApprovePayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Approve(ctx, p, req.PayoutId, req.Params.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return api.ApprovePayout200JSONResponse(payoutToAPI(po)), nil
}

// RejectPayout implements api.StrictServerInterface.
func (h *Handler) RejectPayout(ctx context.Context, req api.RejectPayoutRequestObject) (api.RejectPayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	po, err := h.svc.Reject(ctx, p, req.PayoutId, req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return api.RejectPayout200JSONResponse(payoutToAPI(po)), nil
}

// CancelPayout implements api.StrictServerInterface.
func (h *Handler) CancelPayout(ctx context.Context, req api.CancelPayoutRequestObject) (api.CancelPayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Cancel(ctx, p, req.PayoutId)
	if err != nil {
		return nil, err
	}
	return api.CancelPayout200JSONResponse(payoutToAPI(po)), nil
}

// GetPayoutProof implements api.StrictServerInterface.
func (h *Handler) GetPayoutProof(ctx context.Context, req api.GetPayoutProofRequestObject) (api.GetPayoutProofResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pdf, err := h.svc.Proof(ctx, p, req.PayoutId)
	if err != nil {
		return nil, err
	}
	return api.GetPayoutProof200ApplicationpdfResponse{Body: bytes.NewReader(pdf), ContentLength: int64(len(pdf))}, nil
}

// ListApprovals implements api.StrictServerInterface.
func (h *Handler) ListApprovals(ctx context.Context, req api.ListApprovalsRequestObject) (api.ListApprovalsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pr, err := page.ParseRequest(req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	pg, err := h.svc.ListApprovals(ctx, p, pr)
	if err != nil {
		return nil, err
	}
	items := make([]api.Payout, 0, len(pg.Items))
	for _, po := range pg.Items {
		items = append(items, payoutToAPI(po))
	}
	return api.ListApprovals200JSONResponse{Items: items, NextCursor: pg.NextCursor}, nil
}

func moneyFromAPI(m api.Money) money.Amount { return money.New(m.Minor, money.Currency(m.Currency)) }

func moneyToAPI(a money.Amount) api.Money {
	return api.Money{Minor: a.Minor, Currency: api.Currency(a.Currency)}
}

func destinationToAPI(d psp.Destination) api.Destination {
	out := api.Destination{
		Type: api.DestinationType(d.Type), Value: d.Value, AccountNumber: d.AccountNumber,
		HolderName: d.HolderName, HolderCuit: d.HolderCUIT,
	}
	if d.BankName != "" {
		bank := d.BankName
		out.BankName = &bank
	}
	return out
}

func payoutToAPI(po Payout) api.Payout {
	total, err := po.Total()
	if err != nil { // amount + fee was summed when the hold was taken; it cannot overflow now
		total = po.Amount
	}
	events := make([]api.PayoutEvent, 0, len(po.Events))
	for _, e := range po.Events {
		ev := api.PayoutEvent{ToStatus: api.PayoutStatus(e.ToStatus), ActorId: e.ActorID, Reason: e.Reason, CreatedAt: e.CreatedAt}
		if e.FromStatus != nil {
			from := api.PayoutStatus(*e.FromStatus)
			ev.FromStatus = &from
		}
		events = append(events, ev)
	}
	return api.Payout{
		Id: po.ID, AccountId: po.AccountID, Amount: moneyToAPI(po.Amount), Fee: moneyToAPI(po.Fee), Total: moneyToAPI(total),
		Destination: destinationToAPI(po.Destination), Concept: po.Concept, Status: api.PayoutStatus(po.Status),
		CreatedBy: po.CreatedBy, ApprovedBy: po.ApprovedBy, RejectReason: po.RejectReason, FailureCode: po.FailureCode,
		Events: events, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt, SettledAt: po.SettledAt,
	}
}

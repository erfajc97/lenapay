package payouts

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the payouts operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the payouts handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// ResolveDestination implements api.StrictServerInterface.
func (h *Handler) ResolveDestination(_ context.Context, _ api.ResolveDestinationRequestObject) (api.ResolveDestinationResponseObject, error) {
	return nil, h.notImplemented()
}

// QuotePayout implements api.StrictServerInterface.
func (h *Handler) QuotePayout(_ context.Context, _ api.QuotePayoutRequestObject) (api.QuotePayoutResponseObject, error) {
	return nil, h.notImplemented()
}

// CreatePayout implements api.StrictServerInterface.
func (h *Handler) CreatePayout(_ context.Context, _ api.CreatePayoutRequestObject) (api.CreatePayoutResponseObject, error) {
	return nil, h.notImplemented()
}

// GetPayout implements api.StrictServerInterface.
func (h *Handler) GetPayout(_ context.Context, _ api.GetPayoutRequestObject) (api.GetPayoutResponseObject, error) {
	return nil, h.notImplemented()
}

// ApprovePayout implements api.StrictServerInterface.
func (h *Handler) ApprovePayout(_ context.Context, _ api.ApprovePayoutRequestObject) (api.ApprovePayoutResponseObject, error) {
	return nil, h.notImplemented()
}

// RejectPayout implements api.StrictServerInterface.
func (h *Handler) RejectPayout(_ context.Context, _ api.RejectPayoutRequestObject) (api.RejectPayoutResponseObject, error) {
	return nil, h.notImplemented()
}

// CancelPayout implements api.StrictServerInterface.
func (h *Handler) CancelPayout(_ context.Context, _ api.CancelPayoutRequestObject) (api.CancelPayoutResponseObject, error) {
	return nil, h.notImplemented()
}

// GetPayoutProof implements api.StrictServerInterface.
func (h *Handler) GetPayoutProof(_ context.Context, _ api.GetPayoutProofRequestObject) (api.GetPayoutProofResponseObject, error) {
	return nil, h.notImplemented()
}

// ListApprovals implements api.StrictServerInterface.
func (h *Handler) ListApprovals(_ context.Context, _ api.ListApprovalsRequestObject) (api.ListApprovalsResponseObject, error) {
	return nil, h.notImplemented()
}

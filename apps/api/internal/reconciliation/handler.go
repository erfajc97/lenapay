package reconciliation

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the reconciliation operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the reconciliation handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// ListReconciliationRuns implements api.StrictServerInterface.
func (h *Handler) ListReconciliationRuns(_ context.Context, _ api.ListReconciliationRunsRequestObject) (api.ListReconciliationRunsResponseObject, error) {
	return nil, h.notImplemented()
}

// RunReconciliation implements api.StrictServerInterface.
func (h *Handler) RunReconciliation(_ context.Context, _ api.RunReconciliationRequestObject) (api.RunReconciliationResponseObject, error) {
	return nil, h.notImplemented()
}

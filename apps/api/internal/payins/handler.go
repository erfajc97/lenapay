package payins

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the payins operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the payins handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// GetPayin implements api.StrictServerInterface.
func (h *Handler) GetPayin(_ context.Context, _ api.GetPayinRequestObject) (api.GetPayinResponseObject, error) {
	return nil, h.notImplemented()
}

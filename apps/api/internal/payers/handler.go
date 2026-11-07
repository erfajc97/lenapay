package payers

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the payers operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the payers handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// ListPayers implements api.StrictServerInterface.
func (h *Handler) ListPayers(_ context.Context, _ api.ListPayersRequestObject) (api.ListPayersResponseObject, error) {
	return nil, h.notImplemented()
}

// CreatePayer implements api.StrictServerInterface.
func (h *Handler) CreatePayer(_ context.Context, _ api.CreatePayerRequestObject) (api.CreatePayerResponseObject, error) {
	return nil, h.notImplemented()
}

// GetPayer implements api.StrictServerInterface.
func (h *Handler) GetPayer(_ context.Context, _ api.GetPayerRequestObject) (api.GetPayerResponseObject, error) {
	return nil, h.notImplemented()
}

// UpdatePayer implements api.StrictServerInterface.
func (h *Handler) UpdatePayer(_ context.Context, _ api.UpdatePayerRequestObject) (api.UpdatePayerResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateCvu implements api.StrictServerInterface.
func (h *Handler) CreateCvu(_ context.Context, _ api.CreateCvuRequestObject) (api.CreateCvuResponseObject, error) {
	return nil, h.notImplemented()
}

// UpdateCvu implements api.StrictServerInterface.
func (h *Handler) UpdateCvu(_ context.Context, _ api.UpdateCvuRequestObject) (api.UpdateCvuResponseObject, error) {
	return nil, h.notImplemented()
}

// DisableCvu implements api.StrictServerInterface.
func (h *Handler) DisableCvu(_ context.Context, _ api.DisableCvuRequestObject) (api.DisableCvuResponseObject, error) {
	return nil, h.notImplemented()
}

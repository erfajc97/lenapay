package kyc

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the kyc operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the kyc handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// GetKyc implements api.StrictServerInterface.
func (h *Handler) GetKyc(_ context.Context, _ api.GetKycRequestObject) (api.GetKycResponseObject, error) {
	return nil, h.notImplemented()
}

// UpdateKyc implements api.StrictServerInterface.
func (h *Handler) UpdateKyc(_ context.Context, _ api.UpdateKycRequestObject) (api.UpdateKycResponseObject, error) {
	return nil, h.notImplemented()
}

// UploadKycDocument implements api.StrictServerInterface.
func (h *Handler) UploadKycDocument(_ context.Context, _ api.UploadKycDocumentRequestObject) (api.UploadKycDocumentResponseObject, error) {
	return nil, h.notImplemented()
}

// SubmitKyc implements api.StrictServerInterface.
func (h *Handler) SubmitKyc(_ context.Context, _ api.SubmitKycRequestObject) (api.SubmitKycResponseObject, error) {
	return nil, h.notImplemented()
}

// DecideKyc implements api.StrictServerInterface.
func (h *Handler) DecideKyc(_ context.Context, _ api.DecideKycRequestObject) (api.DecideKycResponseObject, error) {
	return nil, h.notImplemented()
}

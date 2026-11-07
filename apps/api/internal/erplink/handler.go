package erplink

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the erplink operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the erplink handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// GetErpLinkCandidate implements api.StrictServerInterface.
func (h *Handler) GetErpLinkCandidate(_ context.Context, _ api.GetErpLinkCandidateRequestObject) (api.GetErpLinkCandidateResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateErpLink implements api.StrictServerInterface.
func (h *Handler) CreateErpLink(_ context.Context, _ api.CreateErpLinkRequestObject) (api.CreateErpLinkResponseObject, error) {
	return nil, h.notImplemented()
}

// GetErpLink implements api.StrictServerInterface.
func (h *Handler) GetErpLink(_ context.Context, _ api.GetErpLinkRequestObject) (api.GetErpLinkResponseObject, error) {
	return nil, h.notImplemented()
}

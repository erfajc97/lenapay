package notifications

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// Handler implements the notifications operations of api.StrictServerInterface (HTTP only:
// generated request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the notifications handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// RegisterPushToken implements api.StrictServerInterface.
func (h *Handler) RegisterPushToken(ctx context.Context, req api.RegisterPushTokenRequestObject) (api.RegisterPushTokenResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	platform := PlatformUnknown
	if req.Body.Platform != nil {
		platform = Platform(*req.Body.Platform)
	}
	if err := h.svc.RegisterPushToken(ctx, p, req.Body.Token, platform); err != nil {
		return nil, err
	}
	return api.RegisterPushToken204Response{}, nil
}

package orgs

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the orgs operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the orgs handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// ListOrgs implements api.StrictServerInterface.
func (h *Handler) ListOrgs(_ context.Context, _ api.ListOrgsRequestObject) (api.ListOrgsResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateOrg implements api.StrictServerInterface.
func (h *Handler) CreateOrg(_ context.Context, _ api.CreateOrgRequestObject) (api.CreateOrgResponseObject, error) {
	return nil, h.notImplemented()
}

// GetOrg implements api.StrictServerInterface.
func (h *Handler) GetOrg(_ context.Context, _ api.GetOrgRequestObject) (api.GetOrgResponseObject, error) {
	return nil, h.notImplemented()
}

// ListMembers implements api.StrictServerInterface.
func (h *Handler) ListMembers(_ context.Context, _ api.ListMembersRequestObject) (api.ListMembersResponseObject, error) {
	return nil, h.notImplemented()
}

// AddMember implements api.StrictServerInterface.
func (h *Handler) AddMember(_ context.Context, _ api.AddMemberRequestObject) (api.AddMemberResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateInvite implements api.StrictServerInterface.
func (h *Handler) CreateInvite(_ context.Context, _ api.CreateInviteRequestObject) (api.CreateInviteResponseObject, error) {
	return nil, h.notImplemented()
}

// AcceptInvite implements api.StrictServerInterface.
func (h *Handler) AcceptInvite(_ context.Context, _ api.AcceptInviteRequestObject) (api.AcceptInviteResponseObject, error) {
	return nil, h.notImplemented()
}

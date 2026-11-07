package orgs

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/identity"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// Handler implements the orgs operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). The org of /orgs/{orgId}
// routes comes from the guard's principal (T3), never from the path value itself.
type Handler struct {
	svc Service
}

// NewHandler returns the orgs handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ListOrgs implements api.StrictServerInterface.
func (h *Handler) ListOrgs(ctx context.Context, _ api.ListOrgsRequestObject) (api.ListOrgsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := h.svc.List(ctx, p)
	if err != nil {
		return nil, err
	}
	out := api.OrganizationList{Items: make([]api.Organization, 0, len(orgs))}
	for _, o := range orgs {
		out.Items = append(out.Items, organizationToAPI(o))
	}
	return api.ListOrgs200JSONResponse(out), nil
}

// CreateOrg implements api.StrictServerInterface.
func (h *Handler) CreateOrg(ctx context.Context, req api.CreateOrgRequestObject) (api.CreateOrgResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.Create(ctx, p, CreateInput{Kind: Kind(req.Body.Kind), LegalName: req.Body.LegalName, CUIT: req.Body.Cuit})
	if err != nil {
		return nil, err
	}
	return api.CreateOrg201JSONResponse(organizationToAPI(o)), nil
}

// GetOrg implements api.StrictServerInterface.
func (h *Handler) GetOrg(ctx context.Context, _ api.GetOrgRequestObject) (api.GetOrgResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.Get(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetOrg200JSONResponse(organizationToAPI(o)), nil
}

func organizationToAPI(o Organization) api.Organization {
	return api.Organization{
		Id: o.ID, Kind: api.OrgKind(o.Kind), LegalName: o.LegalName, Cuit: o.CUIT,
		KycStatus: api.KycStatus(o.KYCStatus), Role: api.Role(o.Role), CreatedAt: o.CreatedAt,
	}
}

// ListMembers implements api.StrictServerInterface.
func (h *Handler) ListMembers(ctx context.Context, _ api.ListMembersRequestObject) (api.ListMembersResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	members, err := h.svc.ListMembers(ctx, p)
	if err != nil {
		return nil, err
	}
	out := api.MemberList{Items: make([]api.Member, 0, len(members))}
	for _, m := range members {
		out.Items = append(out.Items, memberToAPI(m))
	}
	return api.ListMembers200JSONResponse(out), nil
}

// AddMember implements api.StrictServerInterface.
func (h *Handler) AddMember(ctx context.Context, req api.AddMemberRequestObject) (api.AddMemberResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.AddMember(ctx, p, MemberInput{Email: string(req.Body.Email), Role: auth.Role(req.Body.Role)})
	if err != nil {
		return nil, err
	}
	return api.AddMember201JSONResponse(memberToAPI(m)), nil
}

// UpdateMember implements api.StrictServerInterface.
func (h *Handler) UpdateMember(ctx context.Context, req api.UpdateMemberRequestObject) (api.UpdateMemberResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.UpdateMemberRole(ctx, p, req.UserId, auth.Role(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return api.UpdateMember200JSONResponse(memberToAPI(m)), nil
}

// RemoveMember implements api.StrictServerInterface.
func (h *Handler) RemoveMember(ctx context.Context, req api.RemoveMemberRequestObject) (api.RemoveMemberResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RemoveMember(ctx, p, req.UserId); err != nil {
		return nil, err
	}
	return api.RemoveMember204Response{}, nil
}

func memberToAPI(m Member) api.Member {
	return api.Member{UserId: m.UserID, Email: openapi_types.Email(m.Email), Role: api.Role(m.Role), JoinedAt: m.JoinedAt}
}

// CreateInvite implements api.StrictServerInterface.
func (h *Handler) CreateInvite(ctx context.Context, req api.CreateInviteRequestObject) (api.CreateInviteResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	inv, err := h.svc.CreateInvite(ctx, p, MemberInput{Email: string(req.Body.Email), Role: auth.Role(req.Body.Role)})
	if err != nil {
		return nil, err
	}
	return api.CreateInvite201JSONResponse{
		Id: inv.ID, Email: openapi_types.Email(inv.Email), Role: api.Role(inv.Role), ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}, nil
}

// AcceptInvite implements api.StrictServerInterface.
func (h *Handler) AcceptInvite(ctx context.Context, req api.AcceptInviteRequestObject) (api.AcceptInviteResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.AcceptInvite(ctx, p, req.Token)
	if err != nil {
		return nil, err
	}
	return api.AcceptInvite200JSONResponse(identity.MembershipToAPI(m)), nil
}

package erplink

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// Handler implements the erplink operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc Service
}

// NewHandler returns the erplink handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// GetErpLinkCandidate implements api.StrictServerInterface.
func (h *Handler) GetErpLinkCandidate(ctx context.Context, _ api.GetErpLinkCandidateRequestObject) (api.GetErpLinkCandidateResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.svc.Candidate(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetErpLinkCandidate200JSONResponse{Found: c.Found, ErpOrgId: c.ERPOrgID, DisplayName: c.DisplayName}, nil
}

// CreateErpLink implements api.StrictServerInterface.
func (h *Handler) CreateErpLink(ctx context.Context, req api.CreateErpLinkRequestObject) (api.CreateErpLinkResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	l, err := h.svc.Link(ctx, p, Credentials{Username: req.Body.Username, Password: config.Secret(req.Body.Password)})
	if err != nil {
		return nil, err
	}
	return api.CreateErpLink201JSONResponse(linkToAPI(l)), nil
}

// GetErpLink implements api.StrictServerInterface.
func (h *Handler) GetErpLink(ctx context.Context, _ api.GetErpLinkRequestObject) (api.GetErpLinkResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	l, err := h.svc.Get(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetErpLink200JSONResponse(linkToAPI(l)), nil
}

func linkToAPI(l Link) api.ErpLink {
	return api.ErpLink{
		Id: l.ID, ErpOrgId: l.ERPOrgID, DisplayName: l.DisplayName, LinkedBy: l.LinkedBy, LinkedAt: l.LinkedAt,
		ImportStatus: api.ErpImportStatus(l.ImportStatus), ImportedAt: l.ImportedAt,
	}
}

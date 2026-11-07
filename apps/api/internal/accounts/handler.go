package accounts

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
)

// Handler implements the accounts operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc Service
}

// NewHandler returns the accounts handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// ListAccounts implements api.StrictServerInterface.
func (h *Handler) ListAccounts(_ context.Context, _ api.ListAccountsRequestObject) (api.ListAccountsResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateAccount implements api.StrictServerInterface.
func (h *Handler) CreateAccount(_ context.Context, _ api.CreateAccountRequestObject) (api.CreateAccountResponseObject, error) {
	return nil, h.notImplemented()
}

// UpdateAccount implements api.StrictServerInterface.
func (h *Handler) UpdateAccount(_ context.Context, _ api.UpdateAccountRequestObject) (api.UpdateAccountResponseObject, error) {
	return nil, h.notImplemented()
}

// GetBalances implements api.StrictServerInterface.
func (h *Handler) GetBalances(_ context.Context, _ api.GetBalancesRequestObject) (api.GetBalancesResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateInternalTransfer implements api.StrictServerInterface.
func (h *Handler) CreateInternalTransfer(_ context.Context, _ api.CreateInternalTransferRequestObject) (api.CreateInternalTransferResponseObject, error) {
	return nil, h.notImplemented()
}

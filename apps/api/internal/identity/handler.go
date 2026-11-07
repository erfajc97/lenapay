package identity

import (
	"context"

	"github.com/erfajc97/lenapay/apps/api/internal/api"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

// Handler implements the identity operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). Every method answers
// 501 internal.not_implemented until its lane replaces the stub.
type Handler struct {
	svc  Service
	jwks func() []auth.JWK
}

// NewHandler returns the identity handler; jwks publishes the local IdP keys (auth.Issuer.JWKS).
func NewHandler(svc Service, jwks func() []auth.JWK) *Handler { return &Handler{svc: svc, jwks: jwks} }

// notImplemented is the E0 stub answer; each lane deletes it with its last stub.
func (h *Handler) notImplemented() error {
	_ = h.svc
	return apperr.NotImplemented()
}

// Signup implements api.StrictServerInterface.
func (h *Handler) Signup(_ context.Context, _ api.SignupRequestObject) (api.SignupResponseObject, error) {
	return nil, h.notImplemented()
}

// VerifyEmail implements api.StrictServerInterface.
func (h *Handler) VerifyEmail(_ context.Context, _ api.VerifyEmailRequestObject) (api.VerifyEmailResponseObject, error) {
	return nil, h.notImplemented()
}

// Login implements api.StrictServerInterface.
func (h *Handler) Login(_ context.Context, _ api.LoginRequestObject) (api.LoginResponseObject, error) {
	return nil, h.notImplemented()
}

// RefreshToken implements api.StrictServerInterface.
func (h *Handler) RefreshToken(_ context.Context, _ api.RefreshTokenRequestObject) (api.RefreshTokenResponseObject, error) {
	return nil, h.notImplemented()
}

// Logout implements api.StrictServerInterface.
func (h *Handler) Logout(_ context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	return nil, h.notImplemented()
}

// CreateStepUpChallenge implements api.StrictServerInterface.
func (h *Handler) CreateStepUpChallenge(_ context.Context, _ api.CreateStepUpChallengeRequestObject) (api.CreateStepUpChallengeResponseObject, error) {
	return nil, h.notImplemented()
}

// VerifyStepUp implements api.StrictServerInterface.
func (h *Handler) VerifyStepUp(_ context.Context, _ api.VerifyStepUpRequestObject) (api.VerifyStepUpResponseObject, error) {
	return nil, h.notImplemented()
}

// RegisterDevice implements api.StrictServerInterface.
func (h *Handler) RegisterDevice(_ context.Context, _ api.RegisterDeviceRequestObject) (api.RegisterDeviceResponseObject, error) {
	return nil, h.notImplemented()
}

// GetMe implements api.StrictServerInterface.
func (h *Handler) GetMe(_ context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	return nil, h.notImplemented()
}

// GetJwks publishes the public keys of the local identity provider (R-ID-4).
func (h *Handler) GetJwks(_ context.Context, _ api.GetJwksRequestObject) (api.GetJwksResponseObject, error) {
	keys := h.jwks()
	out := api.Jwks{Keys: make([]api.Jwk, 0, len(keys))}
	for _, k := range keys {
		out.Keys = append(out.Keys, api.Jwk{
			Kty: api.JwkKty(k.Kty), Crv: api.JwkCrv(k.Crv), X: k.X, Kid: k.Kid, Alg: api.JwkAlg(k.Alg), Use: api.JwkUse(k.Use),
		})
	}
	return api.GetJwks200JSONResponse(out), nil
}

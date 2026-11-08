package identity

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"lenapay/apps/api/internal/api"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/config"
)

// Handler implements the identity operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1).
type Handler struct {
	svc  Service
	jwks func() []auth.JWK
}

// NewHandler returns the identity handler; jwks publishes the local IdP keys (auth.Issuer.JWKS).
func NewHandler(svc Service, jwks func() []auth.JWK) *Handler { return &Handler{svc: svc, jwks: jwks} }

// Signup implements api.StrictServerInterface.
func (h *Handler) Signup(ctx context.Context, req api.SignupRequestObject) (api.SignupResponseObject, error) {
	id, err := h.svc.Signup(ctx, SignupInput{Email: string(req.Body.Email), Password: config.Secret(req.Body.Password), FullName: req.Body.FullName})
	if err != nil {
		return nil, err
	}
	return api.Signup201JSONResponse{UserId: id}, nil
}

// VerifyEmail implements api.StrictServerInterface.
func (h *Handler) VerifyEmail(ctx context.Context, req api.VerifyEmailRequestObject) (api.VerifyEmailResponseObject, error) {
	if err := h.svc.VerifyEmail(ctx, string(req.Body.Email), req.Body.Code); err != nil {
		return nil, err
	}
	return api.VerifyEmail204Response{}, nil
}

// ResendEmailCode implements api.StrictServerInterface.
func (h *Handler) ResendEmailCode(ctx context.Context, req api.ResendEmailCodeRequestObject) (api.ResendEmailCodeResponseObject, error) {
	if err := h.svc.ResendEmailCode(ctx, string(req.Body.Email)); err != nil {
		return nil, err
	}
	return api.ResendEmailCode202Response{}, nil
}

// Login implements api.StrictServerInterface.
func (h *Handler) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	t, err := h.svc.Login(ctx, LoginInput{
		Email: string(req.Body.Email), Password: config.Secret(req.Body.Password),
		DeviceID: req.Body.DeviceId, TOTPCode: req.Body.TotpCode,
	})
	if err != nil {
		return nil, err
	}
	return api.Login200JSONResponse(tokenPair(t)), nil
}

// RefreshToken implements api.StrictServerInterface.
func (h *Handler) RefreshToken(ctx context.Context, req api.RefreshTokenRequestObject) (api.RefreshTokenResponseObject, error) {
	t, err := h.svc.Refresh(ctx, req.Body.RefreshToken)
	if err != nil {
		return nil, err
	}
	return api.RefreshToken200JSONResponse(tokenPair(t)), nil
}

func tokenPair(t Tokens) api.TokenPair {
	return api.TokenPair{
		AccessToken: t.AccessToken, AccessTokenExpiresAt: t.AccessExpiresAt,
		RefreshToken: t.RefreshToken, RefreshTokenExpiresAt: t.RefreshExpiresAt, TokenType: api.Bearer,
	}
}

// Logout implements api.StrictServerInterface.
func (h *Handler) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.Logout(ctx, p); err != nil {
		return nil, err
	}
	return api.Logout204Response{}, nil
}

// CreateStepUpChallenge implements api.StrictServerInterface.
func (h *Handler) CreateStepUpChallenge(ctx context.Context, req api.CreateStepUpChallengeRequestObject) (api.CreateStepUpChallengeResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var deviceID *uuid.UUID
	if req.Body != nil {
		deviceID = req.Body.DeviceId
	}
	ch, err := h.svc.CreateStepUpChallenge(ctx, p, deviceID)
	if err != nil {
		return nil, err
	}
	return api.CreateStepUpChallenge201JSONResponse{
		ChallengeId: ch.ID, Nonce: base64.RawURLEncoding.EncodeToString(ch.Nonce), ExpiresAt: ch.ExpiresAt,
	}, nil
}

// VerifyStepUp implements api.StrictServerInterface.
func (h *Handler) VerifyStepUp(ctx context.Context, req api.VerifyStepUpRequestObject) (api.VerifyStepUpResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	in := StepUpInput{ChallengeID: req.Body.ChallengeId, Method: StepUpMethod(req.Body.Method), DeviceID: req.Body.DeviceId}
	if req.Body.Signature != nil {
		if in.Signature, err = decodeBase64URL(*req.Body.Signature, "/signature"); err != nil {
			return nil, err
		}
	}
	if req.Body.Password != nil {
		in.Password = config.Secret(*req.Body.Password)
	}
	t, err := h.svc.VerifyStepUp(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.VerifyStepUp200JSONResponse{StepUpToken: t.Token, ExpiresAt: t.ExpiresAt}, nil
}

// RegisterDevice implements api.StrictServerInterface.
func (h *Handler) RegisterDevice(ctx context.Context, req api.RegisterDeviceRequestObject) (api.RegisterDeviceResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	key, err := decodeBase64URL(req.Body.PublicKey, "/publicKey")
	if err != nil {
		return nil, err
	}
	d, err := h.svc.RegisterDevice(ctx, p, DeviceInput{PublicKey: key, Name: req.Body.Name})
	if err != nil {
		return nil, err
	}
	return api.RegisterDevice201JSONResponse{Id: d.ID, Name: d.Name, CreatedAt: d.CreatedAt}, nil
}

// decodeBase64URL accepts base64url with or without padding (clients differ).
func decodeBase64URL(s, field string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeValidation, err, map[string]any{"field": field})
	}
	return b, nil
}

// GetMe implements api.StrictServerInterface.
func (h *Handler) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	me, err := h.svc.Me(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetMe200JSONResponse(meToAPI(me)), nil
}

// UpdateMe implements api.StrictServerInterface.
func (h *Handler) UpdateMe(ctx context.Context, req api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	fullName, err := patchOf(req.Body.FullName, keep)
	if err != nil {
		return nil, err
	}
	me, err := h.svc.UpdateProfile(ctx, p, fullName)
	if err != nil {
		return nil, err
	}
	return api.UpdateMe200JSONResponse(meToAPI(me)), nil
}

// GetMyPreferences implements api.StrictServerInterface.
func (h *Handler) GetMyPreferences(ctx context.Context, _ api.GetMyPreferencesRequestObject) (api.GetMyPreferencesResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	prefs, err := h.svc.Preferences(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetMyPreferences200JSONResponse(preferencesToAPI(prefs)), nil
}

// UpdateMyPreferences implements api.StrictServerInterface.
func (h *Handler) UpdateMyPreferences(ctx context.Context, req api.UpdateMyPreferencesRequestObject) (api.UpdateMyPreferencesResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var in PreferencesPatch
	if in.DefaultOrgID, err = patchOf(req.Body.DefaultOrgId, parseUUID("defaultOrgId")); err != nil {
		return nil, err
	}
	if in.DefaultAccountID, err = patchOf(req.Body.DefaultAccountId, parseUUID("defaultAccountId")); err != nil {
		return nil, err
	}
	if in.Locale, err = patchOf(req.Body.Locale, keep); err != nil {
		return nil, err
	}
	if in.Theme, err = patchOf(req.Body.Theme, keep); err != nil {
		return nil, err
	}
	prefs, err := h.svc.UpdatePreferences(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.UpdateMyPreferences200JSONResponse(preferencesToAPI(prefs)), nil
}

// patchOf maps a nullable request field to a Patch: absent keeps, null clears, a value is parsed.
func patchOf[T any](f nullable.Nullable[string], parse func(string) (T, error)) (Patch[T], error) {
	if !f.IsSpecified() {
		return Patch[T]{}, nil
	}
	if f.IsNull() {
		return Patch[T]{Set: true}, nil
	}
	raw, err := f.Get()
	if err != nil {
		return Patch[T]{}, err
	}
	v, err := parse(raw)
	if err != nil {
		return Patch[T]{}, err
	}
	return Patch[T]{Set: true, Value: &v}, nil
}

func keep(s string) (string, error) { return s, nil }

// parseUUID parses a uuid field the validator already checked (a miss is validation.failed).
func parseUUID(field string) func(string) (uuid.UUID, error) {
	return func(s string) (uuid.UUID, error) {
		id, err := uuid.Parse(s)
		if err != nil {
			return uuid.Nil, apperr.Wrap(apperr.CodeValidation, err, map[string]any{"field": field})
		}
		return id, nil
	}
}

func meToAPI(me Me) api.Me {
	prefs := preferencesToAPI(me.Preferences)
	out := api.Me{
		User: api.User{
			Id: me.User.ID, Email: openapi_types.Email(me.User.Email), FullName: me.User.FullName,
			EmailVerified: me.User.EmailVerified, MfaEnabled: me.User.MFAEnabled, CreatedAt: me.User.CreatedAt,
		},
		Memberships: make([]api.Membership, 0, len(me.Memberships)),
		Preferences: &prefs,
	}
	for _, m := range me.Memberships {
		out.Memberships = append(out.Memberships, MembershipToAPI(m))
	}
	return out
}

func preferencesToAPI(p Preferences) api.UserPreferences {
	return api.UserPreferences{
		DefaultOrgId: p.DefaultOrgID, DefaultAccountId: p.DefaultAccountID,
		Locale: (*api.UserPreferencesLocale)(p.Locale), Theme: (*api.UserPreferencesTheme)(p.Theme), UpdatedAt: p.UpdatedAt,
	}
}

// MembershipToAPI maps a membership to the contract (shared with orgs' acceptInvite, G12).
func MembershipToAPI(m MembershipSummary) api.Membership {
	return api.Membership{
		OrgId: m.OrgID, OrgName: m.OrgName, OrgKind: api.OrgKind(m.OrgKind),
		Role: api.MemberRole(m.Role), KycStatus: api.KycStatus(m.KYCStatus),
	}
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

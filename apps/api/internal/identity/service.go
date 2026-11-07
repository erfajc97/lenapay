package identity

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/clock"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// Service is the identity API used by the handler and by other features.
type Service interface {
	// Signup creates an unverified user and emails a 6-digit code (R-ID-1).
	Signup(ctx context.Context, in SignupInput) (uuid.UUID, error)
	// VerifyEmail consumes the code (attempts are capped).
	VerifyEmail(ctx context.Context, email, code string) error
	// Login opens a session and returns tokens (R-ID-2; TOTP when enabled).
	Login(ctx context.Context, in LoginInput) (Tokens, error)
	// Refresh rotates the refresh token; reusing a consumed one revokes the session.
	Refresh(ctx context.Context, refreshToken string) (Tokens, error)
	// Logout revokes the caller's session.
	Logout(ctx context.Context, p auth.Principal) error
	// CreateStepUpChallenge issues a nonce for the device key or the password fallback (R-ID-3).
	CreateStepUpChallenge(ctx context.Context, p auth.Principal, deviceID *uuid.UUID) (StepUpChallenge, error)
	// VerifyStepUp checks the answer and issues a step-up token bound to the session.
	VerifyStepUp(ctx context.Context, p auth.Principal, in StepUpInput) (StepUpToken, error)
	// RegisterDevice stores a device public key for biometric step-up.
	RegisterDevice(ctx context.Context, p auth.Principal, in DeviceInput) (Device, error)
	// Me returns the caller and their memberships.
	Me(ctx context.Context, p auth.Principal) (Me, error)

	// UserIDByEmail finds a registered user (orgs: add member, accept invite); user.not_found otherwise.
	UserIDByEmail(ctx context.Context, email string) (uuid.UUID, error)
}

// Deps are the collaborators of the identity service.
type Deps struct {
	Pool        *pgxpool.Pool
	Clock       clock.Clock
	Issuer      *auth.Issuer
	Memberships Memberships
	Codes       CodeSender
	Config      config.Config
}

type service struct {
	deps Deps
}

// NewService returns the identity service.
func NewService(deps Deps) Service { return &service{deps: deps} }

// notImplemented is the E0 stub answer; lane A deletes it with its last stub.
func (s *service) notImplemented() error {
	_ = s.deps
	return apperr.NotImplemented()
}

func (s *service) Signup(context.Context, SignupInput) (uuid.UUID, error) {
	return uuid.Nil, s.notImplemented()
}

func (s *service) VerifyEmail(context.Context, string, string) error { return s.notImplemented() }

func (s *service) Login(context.Context, LoginInput) (Tokens, error) {
	return Tokens{}, s.notImplemented()
}

func (s *service) Refresh(context.Context, string) (Tokens, error) {
	return Tokens{}, s.notImplemented()
}

func (s *service) Logout(context.Context, auth.Principal) error { return s.notImplemented() }

func (s *service) CreateStepUpChallenge(context.Context, auth.Principal, *uuid.UUID) (StepUpChallenge, error) {
	return StepUpChallenge{}, s.notImplemented()
}

func (s *service) VerifyStepUp(context.Context, auth.Principal, StepUpInput) (StepUpToken, error) {
	return StepUpToken{}, s.notImplemented()
}

func (s *service) RegisterDevice(context.Context, auth.Principal, DeviceInput) (Device, error) {
	return Device{}, s.notImplemented()
}

func (s *service) Me(context.Context, auth.Principal) (Me, error) {
	return Me{}, s.notImplemented()
}

func (s *service) UserIDByEmail(context.Context, string) (uuid.UUID, error) {
	return uuid.Nil, s.notImplemented()
}

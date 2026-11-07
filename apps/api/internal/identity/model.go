package identity

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// SignupInput is a new user's credentials.
type SignupInput struct {
	Email    string
	Password config.Secret
}

// LoginInput is a login attempt.
type LoginInput struct {
	Email    string
	Password config.Secret
	DeviceID *uuid.UUID
	TOTPCode *string
}

// Tokens is a short access token plus a rotating refresh token (R-ID-2).
type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// StepUpChallenge is the nonce the device key signs (or the password answers).
type StepUpChallenge struct {
	ID        uuid.UUID
	Nonce     []byte
	ExpiresAt time.Time
}

// StepUpMethod says how a challenge is answered.
type StepUpMethod string

// Step-up methods (contract: StepUpVerifyRequest.method).
const (
	StepUpDeviceSignature StepUpMethod = "device_signature"
	StepUpPassword        StepUpMethod = "password"
)

// StepUpInput answers a challenge.
type StepUpInput struct {
	ChallengeID uuid.UUID
	Method      StepUpMethod
	DeviceID    *uuid.UUID
	Signature   []byte
	Password    config.Secret
}

// StepUpToken is the 5-minute token money routes require (auth.Issuer.IssueStepUp).
type StepUpToken struct {
	Token     string
	ExpiresAt time.Time
}

// DeviceInput registers a device public key (Ed25519, 32 bytes).
type DeviceInput struct {
	PublicKey []byte
	Name      string
}

// Device is a registered device.
type Device struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

// User is the public view of a user.
type User struct {
	ID            uuid.UUID
	Email         string
	EmailVerified bool
	MFAEnabled    bool
	CreatedAt     time.Time
}

// MembershipSummary is one org of the user, as /me shows it (filled by orgs through the
// Memberships port).
type MembershipSummary struct {
	OrgID     uuid.UUID
	OrgName   string
	OrgKind   string
	Role      auth.Role
	KYCStatus string
}

// Me is the current user and their memberships.
type Me struct {
	User        User
	Memberships []MembershipSummary
}

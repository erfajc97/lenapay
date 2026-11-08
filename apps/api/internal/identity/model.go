package identity

import (
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/config"
)

// SignupInput is a new user's credentials and optional display name.
type SignupInput struct {
	Email    string
	Password config.Secret
	FullName *string
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
	FullName      *string
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

// Me is the current user, their memberships and preferences.
type Me struct {
	User        User
	Memberships []MembershipSummary
	Preferences Preferences
}

// Caps of the profile and preferences (contract UpdateMeRequest, UserPreferences; DB CHECKs).
const maxFullName = 120

// validLocale and validTheme mirror the DB CHECKs user_preferences.locale / theme (the
// contract enums reject anything else first).
func validLocale(s string) bool { return s == "es-AR" || s == "en-US" }

func validTheme(s string) bool { return s == "system" || s == "light" || s == "dark" }

// Preferences are a user's app defaults; every field is nil until set.
type Preferences struct {
	DefaultOrgID     *uuid.UUID
	DefaultAccountID *uuid.UUID
	Locale           *string
	Theme            *string
	UpdatedAt        *time.Time
}

// Patch is one field of a partial update (A4): Set false keeps the stored value; Set with a
// nil Value clears it (JSON null).
type Patch[T any] struct {
	Set   bool
	Value *T
}

// apply returns the merged value of a field.
func (p Patch[T]) apply(stored *T) *T {
	if !p.Set {
		return stored
	}
	return p.Value
}

// PreferencesPatch is a PATCH /me/preferences body.
type PreferencesPatch struct {
	DefaultOrgID     Patch[uuid.UUID]
	DefaultAccountID Patch[uuid.UUID]
	Locale           Patch[string]
	Theme            Patch[string]
}

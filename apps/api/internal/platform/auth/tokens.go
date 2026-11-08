package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/ids"
)

// Token uses. A token of one use is never accepted as another (a step-up token is not an
// access token and vice versa).
const (
	useAccess = "access"
	useStepUp = "step_up"
	amrStepUp = "stepup"
)

// IssuerConfig configures the local identity provider.
type IssuerConfig struct {
	Issuer string
	// Seed is the Ed25519 private key seed (32 bytes).
	Seed      []byte
	AccessTTL time.Duration
	StepUpTTL time.Duration
	Clock     clock.Clock
}

// Issuer signs and verifies EdDSA JWTs. Cognito can replace it later behind the same JWKS
// contract (architecture §8).
type Issuer struct {
	issuer    string
	key       ed25519.PrivateKey
	kid       string
	accessTTL time.Duration
	stepUpTTL time.Duration
	clock     clock.Clock
}

// Claims are the claims Lenapay tokens carry.
type Claims struct {
	jwt.RegisteredClaims
	SessionID uuid.UUID `json:"sid"`
	Use       string    `json:"use"`
	AMR       []string  `json:"amr,omitempty"`
}

// UserID returns the subject as a UUID.
func (c Claims) UserID() (uuid.UUID, error) { return uuid.Parse(c.Subject) }

// GenerateSeed returns a random Ed25519 seed (development boots without JWT_PRIVATE_KEY).
func GenerateSeed() []byte {
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed) // crypto/rand.Read never fails (it panics on a broken OS source)
	return seed
}

// NewIssuer builds the issuer; the key id is derived from the public key.
func NewIssuer(cfg IssuerConfig) (*Issuer, error) {
	if len(cfg.Seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("auth: Ed25519 seed must be %d bytes", ed25519.SeedSize)
	}
	if cfg.AccessTTL <= 0 || cfg.StepUpTTL <= 0 || cfg.Clock == nil || cfg.Issuer == "" {
		return nil, errors.New("auth: issuer config incomplete")
	}
	key := ed25519.NewKeyFromSeed(cfg.Seed)
	sum := sha256.Sum256(key.Public().(ed25519.PublicKey))
	return &Issuer{
		issuer:    cfg.Issuer,
		key:       key,
		kid:       base64.RawURLEncoding.EncodeToString(sum[:8]),
		accessTTL: cfg.AccessTTL,
		stepUpTTL: cfg.StepUpTTL,
		clock:     cfg.Clock,
	}, nil
}

// IssueAccess signs a short access token for a user session.
func (i *Issuer) IssueAccess(userID, sessionID uuid.UUID) (string, time.Time, error) {
	return i.sign(userID, sessionID, useAccess, []string{"pwd"}, i.accessTTL)
}

// IssueStepUp signs a step-up token bound to the session (R-ID-3, 5 minutes).
func (i *Issuer) IssueStepUp(userID, sessionID uuid.UUID) (string, time.Time, error) {
	return i.sign(userID, sessionID, useStepUp, []string{amrStepUp}, i.stepUpTTL)
}

func (i *Issuer) sign(userID, sessionID uuid.UUID, use string, amr []string, ttl time.Duration) (string, time.Time, error) {
	now := i.clock.Now()
	exp := now.Add(ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{i.issuer},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        ids.New().String(),
		},
		SessionID: sessionID,
		Use:       use,
		AMR:       amr,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = i.kid
	signed, err := tok.SignedString(i.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// VerifyAccess verifies an access token and returns its claims.
func (i *Issuer) VerifyAccess(raw string) (Claims, error) {
	return i.verify(raw, useAccess)
}

// VerifyStepUp verifies a step-up token for the given user session.
func (i *Issuer) VerifyStepUp(raw string, userID, sessionID uuid.UUID) error {
	c, err := i.verify(raw, useStepUp)
	if err != nil {
		return err
	}
	if c.Subject != userID.String() || c.SessionID != sessionID || !slices.Contains(c.AMR, amrStepUp) {
		return errors.New("auth: step-up token bound to another session")
	}
	return nil
}

func (i *Issuer) verify(raw, use string) (Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(raw, &c,
		func(t *jwt.Token) (any, error) { return i.key.Public(), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithAudience(i.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(i.clock.Now),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("auth: invalid token: %w", err)
	}
	if c.Use != use {
		return Claims{}, fmt.Errorf("auth: token use %q, want %q", c.Use, use)
	}
	if c.SessionID == uuid.Nil {
		return Claims{}, errors.New("auth: token without session")
	}
	if _, err := c.UserID(); err != nil {
		return Claims{}, fmt.Errorf("auth: token subject: %w", err)
	}
	return c, nil
}

// JWK is one public key of the JWKS document (OKP / Ed25519).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// JWKS returns the public keys clients and other services verify tokens with.
func (i *Issuer) JWKS() []JWK {
	return []JWK{{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(i.key.Public().(ed25519.PublicKey)),
		Kid: i.kid,
		Alg: jwt.SigningMethodEdDSA.Alg(),
		Use: "sig",
	}}
}

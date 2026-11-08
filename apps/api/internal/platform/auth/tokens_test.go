package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/ids"
)

func testIssuer(t *testing.T, c clock.Clock, seedByte byte) *Issuer {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = seedByte
	iss, err := NewIssuer(IssuerConfig{
		Issuer: "http://lenapay.test", Seed: seed, Clock: c,
		AccessTTL: 15 * time.Minute, StepUpTTL: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return iss
}

func TestAccessTokenRoundTripAndExpiry(t *testing.T) {
	fake := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	iss := testIssuer(t, fake, 1)
	user, session := ids.New(), ids.New()

	tok, exp, err := iss.IssueAccess(user, session)
	if err != nil || !exp.Equal(fake.Now().Add(15*time.Minute)) {
		t.Fatalf("IssueAccess exp=%v err=%v", exp, err)
	}
	claims, err := iss.VerifyAccess(tok)
	if err != nil || claims.SessionID != session || claims.Subject != user.String() {
		t.Fatalf("VerifyAccess = %+v, %v", claims, err)
	}

	// Boundary pair (FT1): valid one second before expiry, rejected at expiry.
	fake.Advance(15*time.Minute - time.Second)
	if _, err := iss.VerifyAccess(tok); err != nil {
		t.Fatalf("token must still be valid: %v", err)
	}
	fake.Advance(time.Second)
	if _, err := iss.VerifyAccess(tok); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestTokenUsesAreNotInterchangeable(t *testing.T) {
	iss := testIssuer(t, clock.Real{}, 1)
	user, session := ids.New(), ids.New()
	access, _, _ := iss.IssueAccess(user, session)
	stepUp, _, _ := iss.IssueStepUp(user, session)

	if _, err := iss.VerifyAccess(stepUp); err == nil {
		t.Fatal("a step-up token must not work as an access token")
	}
	if err := iss.VerifyStepUp(access, user, session); err == nil {
		t.Fatal("an access token must not work as a step-up token")
	}
	if err := iss.VerifyStepUp(stepUp, user, session); err != nil {
		t.Fatalf("valid step-up rejected: %v", err)
	}
	if err := iss.VerifyStepUp(stepUp, user, ids.New()); err == nil {
		t.Fatal("step-up bound to another session accepted")
	}
	if err := iss.VerifyStepUp(stepUp, ids.New(), session); err == nil {
		t.Fatal("step-up of another user accepted")
	}
}

func TestStepUpExpiresAfterFiveMinutes(t *testing.T) {
	fake := clock.NewFake(time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	iss := testIssuer(t, fake, 1)
	user, session := ids.New(), ids.New()
	tok, _, _ := iss.IssueStepUp(user, session)
	fake.Advance(5*time.Minute - time.Second)
	if err := iss.VerifyStepUp(tok, user, session); err != nil {
		t.Fatalf("step-up must be valid just before 5 minutes: %v", err)
	}
	fake.Advance(time.Second)
	if err := iss.VerifyStepUp(tok, user, session); err == nil {
		t.Fatal("step-up valid after 5 minutes")
	}
}

func TestForgedTokensRejected(t *testing.T) {
	iss := testIssuer(t, clock.Real{}, 1)
	other := testIssuer(t, clock.Real{}, 2)
	user, session := ids.New(), ids.New()

	foreign, _, _ := other.IssueAccess(user, session)
	if _, err := iss.VerifyAccess(foreign); err == nil {
		t.Fatal("token signed by another key accepted")
	}

	tok, _, _ := iss.IssueAccess(user, session)
	parts := strings.Split(tok, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	tampered := strings.Replace(string(payload), user.String(), ids.New().String(), 1)
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(tampered))
	if _, err := iss.VerifyAccess(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered token accepted")
	}

	// alg=none and HMAC tokens must never be accepted.
	none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		SessionID: session, Use: useAccess,
		RegisteredClaims: jwt.RegisteredClaims{Subject: user.String(), Issuer: "http://lenapay.test"},
	})
	unsigned, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := iss.VerifyAccess(unsigned); err == nil {
		t.Fatal("alg=none accepted")
	}
}

func TestJWKSVerifiesIssuedTokens(t *testing.T) {
	iss := testIssuer(t, clock.Real{}, 1)
	keys := iss.JWKS()
	if len(keys) != 1 || keys[0].Kty != "OKP" || keys[0].Crv != "Ed25519" || keys[0].Alg != "EdDSA" || keys[0].Use != "sig" {
		t.Fatalf("jwks %+v", keys)
	}
	pub, err := base64.RawURLEncoding.DecodeString(keys[0].X)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatalf("x = %q", keys[0].X)
	}
	tok, _, _ := iss.IssueAccess(ids.New(), ids.New())
	parsed, err := jwt.Parse(tok, func(*jwt.Token) (any, error) { return ed25519.PublicKey(pub), nil })
	if err != nil || parsed.Header["kid"] != keys[0].Kid {
		t.Fatalf("token not verifiable with the JWKS key: %v (kid %v)", err, parsed.Header["kid"])
	}
}

func TestNewIssuerValidates(t *testing.T) {
	if _, err := NewIssuer(IssuerConfig{Seed: make([]byte, 31)}); err == nil {
		t.Fatal("short seed accepted")
	}
	if _, err := NewIssuer(IssuerConfig{Seed: GenerateSeed()}); err == nil {
		t.Fatal("incomplete config accepted")
	}
}

func TestRoleOrdering(t *testing.T) {
	cases := []struct {
		have, min Role
		want      bool
	}{
		{RoleAdmin, RoleApprover, true},
		{RoleApprover, RoleApprover, true},
		{RoleOperator, RoleApprover, false},
		{RoleViewer, RoleViewer, true},
		{RoleViewer, RoleOperator, false},
		{RoleOwner, RoleAdmin, true},
		{RoleAdmin, RoleOwner, false},
		{Role("root"), RoleViewer, false},
	}
	for _, c := range cases {
		if got := c.have.AtLeast(c.min); got != c.want {
			t.Errorf("%s.AtLeast(%s) = %v", c.have, c.min, got)
		}
	}
	if _, ok := ParseRole("approver"); !ok {
		t.Error("approver must parse")
	}
	if _, ok := ParseRole("root"); ok {
		t.Error("unknown role parsed")
	}
	for role, want := range map[Role]bool{RoleViewer: true, RoleAdmin: true, RoleOwner: false, Role("root"): false} {
		if role.Grantable() != want {
			t.Errorf("%s.Grantable() = %v", role, !want)
		}
	}
}

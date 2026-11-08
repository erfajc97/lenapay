//go:build integration

package identity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/identity"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/db/dbtest"
	"lenapay/apps/api/internal/platform/ids"
)

var pg *dbtest.Postgres

func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }

// inbox is the CodeSender fake: it keeps the last code per email.
type inbox struct {
	mu    sync.Mutex
	codes map[string]string
	fail  error
}

func (i *inbox) SendEmailCode(_ context.Context, email, code string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.fail != nil {
		return i.fail
	}
	i.codes[email] = code
	return nil
}

func (i *inbox) code(t *testing.T, email string) string {
	t.Helper()
	i.mu.Lock()
	defer i.mu.Unlock()
	c, ok := i.codes[email]
	if !ok {
		t.Fatalf("no code sent to %s", email)
	}
	return c
}

type env struct {
	svc    identity.Service
	pool   *pgxpool.Pool
	clock  *clock.Fake
	inbox  *inbox
	issuer *auth.Issuer
}

var start = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func newEnv(t *testing.T, cfg config.Config) env {
	t.Helper()
	pool := pg.NewDatabase(t)
	clk := clock.NewFake(start)
	issuer, err := auth.NewIssuer(auth.IssuerConfig{
		Issuer: "http://lenapay.test", Seed: make([]byte, ed25519.SeedSize), Clock: clk,
		AccessTTL: 15 * time.Minute, StepUpTTL: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env == "" {
		cfg.Env = config.EnvTest
	}
	cfg.RefreshTTL = 720 * time.Hour
	box := &inbox{codes: map[string]string{}}
	svc := identity.NewService(identity.Deps{
		Pool: pool, Clock: clk, Issuer: issuer, Codes: box, Config: cfg,
		Memberships: identity.MembershipsFunc(func(context.Context, uuid.UUID) ([]identity.MembershipSummary, error) {
			return []identity.MembershipSummary{{OrgName: "Constructora Sur", Role: auth.RoleAdmin}}, nil
		}),
	})
	return env{svc: svc, pool: pool, clock: clk, inbox: box, issuer: issuer}
}

const password = "una-clave-larga-123"

// signupVerified creates a verified user and returns its id.
func (e env) signupVerified(t *testing.T, email string) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	id, err := e.svc.Signup(ctx, identity.SignupInput{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.VerifyEmail(ctx, email, e.inbox.code(t, email)); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e env) login(t *testing.T, email string) (identity.Tokens, auth.Principal) {
	t.Helper()
	tok, err := e.svc.Login(t.Context(), identity.LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	c, err := e.issuer.VerifyAccess(tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := c.UserID()
	return tok, auth.Principal{UserID: userID, SessionID: c.SessionID}
}

func wantCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if !apperr.IsCode(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

// The R-ID-1/R-ID-2 golden path, including refresh-token reuse: replaying a consumed refresh
// token (a stolen copy) ends the whole session, so the thief and the victim both lose it.
func TestSignupVerifyLoginRefreshReuseLogout(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	dir := auth.NewPGDirectory(e.pool)

	userID, err := e.svc.Signup(ctx, identity.SignupInput{Email: "Ana@Example.com", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Login(ctx, identity.LoginInput{Email: "ana@example.com", Password: password})
	wantCode(t, err, apperr.CodeEmailNotVerified)
	wantCode(t, e.svc.VerifyEmail(ctx, "ana@example.com", "000000"), apperr.CodeCodeInvalid)
	if err := e.svc.VerifyEmail(ctx, "ANA@example.com", e.inbox.code(t, "ana@example.com")); err != nil {
		t.Fatal(err)
	}
	wantCode(t, e.svc.VerifyEmail(ctx, "ana@example.com", e.inbox.code(t, "ana@example.com")), apperr.CodeCodeInvalid)

	first, p := e.login(t, "ana@example.com")
	if p.UserID != userID || first.AccessExpiresAt != start.Add(15*time.Minute) || first.RefreshExpiresAt != start.Add(720*time.Hour) {
		t.Fatalf("tokens %+v for %v", first, p)
	}
	second, err := e.svc.Refresh(ctx, first.RefreshToken)
	if err != nil || second.RefreshToken == first.RefreshToken {
		t.Fatalf("rotation: %v %+v", err, second)
	}
	if err := dir.ActiveSession(ctx, p.UserID, p.SessionID); err != nil {
		t.Fatalf("session must stay active after a rotation: %v", err)
	}

	_, err = e.svc.Refresh(ctx, first.RefreshToken) // reuse of the consumed token
	wantCode(t, err, apperr.CodeRefreshInvalid)
	if err := dir.ActiveSession(ctx, p.UserID, p.SessionID); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("reuse must revoke the session, got %v", err)
	}
	_, err = e.svc.Refresh(ctx, second.RefreshToken)
	wantCode(t, err, apperr.CodeRefreshInvalid)

	_, p2 := e.login(t, "ana@example.com")
	if err := e.svc.Logout(ctx, p2); err != nil {
		t.Fatal(err)
	}
	if err := dir.ActiveSession(ctx, p2.UserID, p2.SessionID); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("logout must revoke the session, got %v", err)
	}
}

func TestRefreshRejections(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	e.signupVerified(t, "beto@example.com")

	tok, _ := e.login(t, "beto@example.com")
	_, err := e.svc.Refresh(ctx, "not-a-token-at-all-0000")
	wantCode(t, err, apperr.CodeRefreshInvalid)

	e.clock.Advance(720*time.Hour - time.Second) // last instant the token is valid
	tok, err = e.svc.Refresh(ctx, tok.RefreshToken)
	if err != nil {
		t.Fatalf("refresh one second before expiry: %v", err)
	}
	e.clock.Advance(720 * time.Hour) // exactly at expiry: rejected
	_, err = e.svc.Refresh(ctx, tok.RefreshToken)
	wantCode(t, err, apperr.CodeRefreshInvalid)

	tok, _ = e.login(t, "beto@example.com")
	if _, err := e.pool.Exec(ctx, "UPDATE users SET status = 'disabled' WHERE email = 'beto@example.com'"); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Refresh(ctx, tok.RefreshToken)
	wantCode(t, err, apperr.CodeRefreshInvalid)
}

// The 5th wrong code burns it (6 digits must not be brute-forced); the code expires at 10 min.
func TestEmailCodeAttemptsAndExpiry(t *testing.T) {
	t.Parallel()
	t.Run("four wrong attempts still let the right code in", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		e := newEnv(t, config.Config{})
		if _, err := e.svc.Signup(ctx, identity.SignupInput{Email: "c@example.com", Password: password}); err != nil {
			t.Fatal(err)
		}
		for range 4 {
			wantCode(t, e.svc.VerifyEmail(ctx, "c@example.com", "000000"), apperr.CodeCodeInvalid)
		}
		if err := e.svc.VerifyEmail(ctx, "c@example.com", e.inbox.code(t, "c@example.com")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("the fifth wrong attempt burns the code", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		e := newEnv(t, config.Config{})
		if _, err := e.svc.Signup(ctx, identity.SignupInput{Email: "d@example.com", Password: password}); err != nil {
			t.Fatal(err)
		}
		for range 5 {
			wantCode(t, e.svc.VerifyEmail(ctx, "d@example.com", "000000"), apperr.CodeCodeInvalid)
		}
		wantCode(t, e.svc.VerifyEmail(ctx, "d@example.com", e.inbox.code(t, "d@example.com")), apperr.CodeCodeInvalid)
	})
	t.Run("valid until just before 10 minutes", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		e := newEnv(t, config.Config{})
		if _, err := e.svc.Signup(ctx, identity.SignupInput{Email: "f@example.com", Password: password}); err != nil {
			t.Fatal(err)
		}
		e.clock.Advance(10*time.Minute - time.Second)
		if err := e.svc.VerifyEmail(ctx, "f@example.com", e.inbox.code(t, "f@example.com")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("expired at 10 minutes", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		e := newEnv(t, config.Config{})
		if _, err := e.svc.Signup(ctx, identity.SignupInput{Email: "g@example.com", Password: password}); err != nil {
			t.Fatal(err)
		}
		e.clock.Advance(10 * time.Minute)
		wantCode(t, e.svc.VerifyEmail(ctx, "g@example.com", e.inbox.code(t, "g@example.com")), apperr.CodeCodeInvalid)
	})
	t.Run("unknown email", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		e := newEnv(t, config.Config{})
		wantCode(t, e.svc.VerifyEmail(ctx, "nobody@example.com", "123456"), apperr.CodeCodeInvalid)
	})
}

// Signing up again with the same credentials re-sends a code (the previous one dies); at most
// 5 codes per hour; another password or a verified email is auth.email_taken.
func TestSignupResendAndRateLimit(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	in := identity.SignupInput{Email: "h@example.com", Password: password}
	id, err := e.svc.Signup(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	old := e.inbox.code(t, "h@example.com")
	_, err = e.svc.Signup(ctx, identity.SignupInput{Email: "H@example.com", Password: "otra-clave-larga-9"})
	wantCode(t, err, apperr.CodeEmailTaken)

	for range 4 { // codes 2..5 within the hour
		again, err := e.svc.Signup(ctx, in)
		if err != nil || again != id {
			t.Fatalf("resend: %v (id %v want %v)", err, again, id)
		}
	}
	if latest := e.inbox.code(t, "h@example.com"); latest != old {
		wantCode(t, e.svc.VerifyEmail(ctx, "h@example.com", old), apperr.CodeCodeInvalid)
	}
	_, err = e.svc.Signup(ctx, in) // 6th code in the window
	wantCode(t, err, apperr.CodeRateLimited)

	e.clock.Advance(time.Hour + time.Second)
	if _, err := e.svc.Signup(ctx, in); err != nil {
		t.Fatalf("after the window: %v", err)
	}
	if err := e.svc.VerifyEmail(ctx, "h@example.com", e.inbox.code(t, "h@example.com")); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Signup(ctx, in)
	wantCode(t, err, apperr.CodeEmailTaken)
}

// ResendEmailCode re-issues the code of an unverified user (the old one dies), shares the
// 5-per-hour limit with signup, and is a silent no-op for unknown or verified emails.
func TestResendEmailCode(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	if err := e.svc.ResendEmailCode(ctx, "ghost@example.com"); err != nil {
		t.Fatalf("unknown email: %v", err)
	}
	if _, ok := e.inbox.codes["ghost@example.com"]; ok {
		t.Fatal("a code was sent to an unknown email")
	}
	in := identity.SignupInput{Email: "r@example.com", Password: password}
	if _, err := e.svc.Signup(ctx, in); err != nil {
		t.Fatal(err)
	}
	for range 4 { // codes 2..5 within the hour
		if err := e.svc.ResendEmailCode(ctx, " R@example.com "); err != nil {
			t.Fatalf("resend: %v", err)
		}
	}
	wantCode(t, e.svc.ResendEmailCode(ctx, "r@example.com"), apperr.CodeRateLimited)
	if err := e.svc.VerifyEmail(ctx, "r@example.com", e.inbox.code(t, "r@example.com")); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Hour + time.Second)
	delete(e.inbox.codes, "r@example.com")
	if err := e.svc.ResendEmailCode(ctx, "r@example.com"); err != nil {
		t.Fatalf("verified email: %v", err)
	}
	if _, ok := e.inbox.codes["r@example.com"]; ok {
		t.Fatal("a code was sent to a verified email")
	}
}

// A failed delivery does not undo the signup (it is logged; the user re-sends).
func TestSignupSurvivesDeliveryFailure(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	e.inbox.fail = errors.New("smtp down")
	if _, err := e.svc.Signup(t.Context(), identity.SignupInput{Email: "i@example.com", Password: password}); err != nil {
		t.Fatalf("signup must not fail on delivery: %v", err)
	}
	if _, err := e.svc.UserIDByEmail(t.Context(), "i@example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestDevFixedEmailCode(t *testing.T) {
	t.Parallel()
	dev := newEnv(t, config.Config{Env: config.EnvDevelopment, DevFixedEmailCode: "123456"})
	if _, err := dev.svc.Signup(t.Context(), identity.SignupInput{Email: "j@example.com", Password: password}); err != nil {
		t.Fatal(err)
	}
	if got := dev.inbox.code(t, "j@example.com"); got != "123456" {
		t.Fatalf("development fixed code: got %s", got)
	}
	prod := newEnv(t, config.Config{Env: config.EnvProduction, DevFixedEmailCode: "123456"})
	for i := range 3 { // a random code equals 123456 with p = 1e-6; three in a row never
		email := string(rune('k'+i)) + "@example.com"
		if _, err := prod.svc.Signup(t.Context(), identity.SignupInput{Email: email, Password: password}); err != nil {
			t.Fatal(err)
		}
		if prod.inbox.code(t, email) != "123456" {
			return
		}
	}
	t.Fatal("production must never use the fixed code")
}

func TestLoginRejections(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	e.signupVerified(t, "l@example.com")
	unknownDevice := ids.New()
	tests := []struct {
		name string
		in   identity.LoginInput
		want apperr.Code
	}{
		{"unknown email", identity.LoginInput{Email: "nobody@example.com", Password: password}, apperr.CodeInvalidCredentials},
		{"wrong password", identity.LoginInput{Email: "l@example.com", Password: password + "x"}, apperr.CodeInvalidCredentials},
		{"unknown device", identity.LoginInput{Email: "l@example.com", Password: password, DeviceID: &unknownDevice}, apperr.CodeDeviceNotFound},
	}
	for _, tt := range tests {
		_, err := e.svc.Login(ctx, tt.in)
		wantCode(t, err, tt.want)
	}
	if _, err := e.pool.Exec(ctx, "UPDATE users SET status = 'disabled' WHERE email = 'l@example.com'"); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.Login(ctx, identity.LoginInput{Email: "l@example.com", Password: password})
	wantCode(t, err, apperr.CodeInvalidCredentials)
	_, err = e.svc.UserIDByEmail(ctx, "l@example.com")
	wantCode(t, err, apperr.CodeUserNotFound)
}

func TestLoginWithTOTP(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	e.signupVerified(t, "m@example.com")
	// RFC 6238 SHA-1 test secret; at T=59 the 6-digit code is 287082.
	if _, err := e.pool.Exec(ctx, "UPDATE users SET totp_secret = $1, totp_enabled_at = now() WHERE email = 'm@example.com'",
		[]byte("12345678901234567890")); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(time.Unix(59, 0))
	_, err := e.svc.Login(ctx, identity.LoginInput{Email: "m@example.com", Password: password})
	wantCode(t, err, apperr.CodeMFARequired)
	wrong := "111111"
	_, err = e.svc.Login(ctx, identity.LoginInput{Email: "m@example.com", Password: password, TOTPCode: &wrong})
	wantCode(t, err, apperr.CodeInvalidCredentials)
	right := "287082"
	if _, err := e.svc.Login(ctx, identity.LoginInput{Email: "m@example.com", Password: password, TOTPCode: &right}); err != nil {
		t.Fatal(err)
	}
}

// R-ID-3: the device key signs the nonce; every challenge allows exactly one answer and is
// bound to the session that asked for it; the token is bound to that session too.
func TestStepUpWithDeviceKey(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	e.signupVerified(t, "n@example.com")
	_, p := e.login(t, "n@example.com")
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	dev, err := e.svc.RegisterDevice(ctx, p, identity.DeviceInput{PublicKey: pub, Name: " Pixel 9 "})
	if err != nil || dev.Name != "Pixel 9" {
		t.Fatalf("register: %v %+v", err, dev)
	}

	ch, err := e.svc.CreateStepUpChallenge(ctx, p, &dev.ID)
	if err != nil || len(ch.Nonce) != 32 || ch.ExpiresAt != start.Add(2*time.Minute) {
		t.Fatalf("challenge: %v %+v", err, ch)
	}
	answer := identity.StepUpInput{ChallengeID: ch.ID, Method: identity.StepUpDeviceSignature, DeviceID: &dev.ID, Signature: ed25519.Sign(priv, ch.Nonce)}
	tok, err := e.svc.VerifyStepUp(ctx, p, answer)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.issuer.VerifyStepUp(tok.Token, p.UserID, p.SessionID); err != nil || tok.ExpiresAt != start.Add(5*time.Minute) {
		t.Fatalf("step-up token: %v (expires %v)", err, tok.ExpiresAt)
	}
	if e.issuer.VerifyStepUp(tok.Token, p.UserID, ids.New()) == nil {
		t.Fatal("a step-up token must not work for another session")
	}
	_, err = e.svc.VerifyStepUp(ctx, p, answer) // replay
	wantCode(t, err, apperr.CodeChallengeInvalid)

	ch, _ = e.svc.CreateStepUpChallenge(ctx, p, &dev.ID)
	bad := answer
	bad.ChallengeID, bad.Signature = ch.ID, ed25519.Sign(priv, []byte("another nonce"))
	_, err = e.svc.VerifyStepUp(ctx, p, bad)
	wantCode(t, err, apperr.CodeChallengeInvalid)
	good := answer
	good.ChallengeID, good.Signature = ch.ID, ed25519.Sign(priv, ch.Nonce)
	_, err = e.svc.VerifyStepUp(ctx, p, good) // the wrong answer burned it
	wantCode(t, err, apperr.CodeChallengeInvalid)

	ch, _ = e.svc.CreateStepUpChallenge(ctx, p, &dev.ID)
	other := p
	other.SessionID = ids.New()
	good.ChallengeID = ch.ID
	good.Signature = ed25519.Sign(priv, ch.Nonce)
	_, err = e.svc.VerifyStepUp(ctx, other, good)
	wantCode(t, err, apperr.CodeChallengeInvalid)

	e.clock.Advance(2 * time.Minute) // expired exactly at its TTL
	_, err = e.svc.VerifyStepUp(ctx, p, good)
	wantCode(t, err, apperr.CodeChallengeInvalid)
}

func TestStepUpWithPassword(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	ctx := t.Context()
	e.signupVerified(t, "o@example.com")
	stranger := e.signupVerified(t, "p@example.com")
	_, p := e.login(t, "o@example.com")
	_, ps := e.login(t, "p@example.com")
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	strangerDevice, err := e.svc.RegisterDevice(ctx, ps, identity.DeviceInput{PublicKey: pub, Name: "otro"})
	if err != nil || ps.UserID != stranger {
		t.Fatal(err)
	}
	_, err = e.svc.CreateStepUpChallenge(ctx, p, &strangerDevice.ID)
	wantCode(t, err, apperr.CodeDeviceNotFound)

	ch, err := e.svc.CreateStepUpChallenge(ctx, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.VerifyStepUp(ctx, p, identity.StepUpInput{ChallengeID: ch.ID, Method: identity.StepUpPassword, Password: "wrong-password-1"})
	wantCode(t, err, apperr.CodeInvalidCredentials)
	ch, _ = e.svc.CreateStepUpChallenge(ctx, p, nil)
	_, err = e.svc.VerifyStepUp(ctx, p, identity.StepUpInput{ChallengeID: ch.ID, Method: identity.StepUpDeviceSignature, Signature: make([]byte, 64)})
	wantCode(t, err, apperr.CodeChallengeInvalid) // no device on a password challenge
	ch, _ = e.svc.CreateStepUpChallenge(ctx, p, nil)
	if _, err := e.svc.VerifyStepUp(ctx, p, identity.StepUpInput{ChallengeID: ch.ID, Method: identity.StepUpPassword, Password: password}); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.RegisterDevice(ctx, p, identity.DeviceInput{PublicKey: pub[:31], Name: "corto"})
	wantCode(t, err, apperr.CodeValidation)
	_, err = e.svc.RegisterDevice(ctx, p, identity.DeviceInput{PublicKey: pub, Name: "   "})
	wantCode(t, err, apperr.CodeValidation)
}

func TestMe(t *testing.T) {
	t.Parallel()
	e := newEnv(t, config.Config{})
	id := e.signupVerified(t, "q@example.com")
	_, p := e.login(t, "q@example.com")
	me, err := e.svc.Me(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if me.User.ID != id || me.User.Email != "q@example.com" || !me.User.EmailVerified || me.User.MFAEnabled ||
		len(me.Memberships) != 1 || me.Memberships[0].OrgName != "Constructora Sur" {
		t.Fatalf("me %+v", me)
	}
	_, err = e.svc.Me(t.Context(), auth.Principal{UserID: ids.New()})
	wantCode(t, err, apperr.CodeUnauthenticated)
	got, err := e.svc.UserIDByEmail(t.Context(), " Q@Example.com ")
	if err != nil || got != id {
		t.Fatalf("UserIDByEmail: %v %v", got, err)
	}
	_, err = e.svc.UserIDByEmail(t.Context(), "nobody@example.com")
	wantCode(t, err, apperr.CodeUserNotFound)
}

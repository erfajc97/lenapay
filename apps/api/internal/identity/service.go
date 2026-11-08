package identity

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lenapay/apps/api/internal/identity/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/clock"
	"lenapay/apps/api/internal/platform/config"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/ids"
)

// Service is the identity API used by the handler and by other features.
type Service interface {
	// Signup creates an unverified user and emails a 6-digit code (R-ID-1). Signing up again
	// with the same email and password while unverified re-sends a code (rate limited). A new
	// user runs Deps.OnSignup in the same transaction (the personal org of personal mode).
	Signup(ctx context.Context, in SignupInput) (uuid.UUID, error)
	// ResendEmailCode issues and emails a fresh code to an UNVERIFIED user (the previous one
	// stops working; rate limited). An unknown or verified email is a silent no-op, so the
	// answer never reveals whether an email is registered.
	ResendEmailCode(ctx context.Context, email string) error
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
	// Me returns the caller, their memberships and preferences.
	Me(ctx context.Context, p auth.Principal) (Me, error)
	// UpdateProfile sets or clears the caller's full name (PATCH /me).
	UpdateProfile(ctx context.Context, p auth.Principal, fullName Patch[string]) (Me, error)
	// Preferences returns the caller's preferences (a default org they no longer belong to reads
	// as unset).
	Preferences(ctx context.Context, p auth.Principal) (Preferences, error)
	// UpdatePreferences merges a partial update and validates the merged state (A4).
	UpdatePreferences(ctx context.Context, p auth.Principal, in PreferencesPatch) (Preferences, error)
	// FillFullNameIfEmpty names a user from a verified identity (person KYC approval) unless they
	// already named themselves, inside the approval transaction.
	FillFullNameIfEmpty(ctx context.Context, tx pgx.Tx, userID uuid.UUID, name string) error

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
	// OnSignup runs for each new user inside the signup transaction (nil: nothing).
	OnSignup SignupHook
	// Accounts checks a preferred default account.
	Accounts Accounts
	// Invites accepts the ERP import invites of a verified email (nil: nothing).
	Invites InviteAcceptor
}

// Limits of the email code and the step-up challenge (R-ID-1, R-ID-3).
const (
	emailCodeTTL         = 10 * time.Minute
	emailCodeMaxAttempts = 5
	emailCodesPerWindow  = 5
	emailCodeWindow      = time.Hour
	challengeTTL         = 2 * time.Minute
	purposeVerifyEmail   = "verify_email"
	statusActive         = "active"
)

type service struct {
	deps Deps
	// dummyHash is verified for unknown emails, so a login for a missing user costs the same
	// as one for an existing user (no user enumeration by timing).
	dummyHash string
}

// NewService returns the identity service.
func NewService(deps Deps) Service {
	return &service{deps: deps, dummyHash: hashPassword(newOpaqueToken())}
}

func (s *service) now() time.Time { return s.deps.Clock.Now() }

func (s *service) nowRef() *time.Time {
	t := s.now()
	return &t
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// commitThen runs fn in one transaction and, once it COMMITTED, returns the outcome fn
// reported. It is for rejections that must still persist a write (a burned challenge, a
// counted attempt, a session revoked on refresh-token reuse): returning the rejection from
// fn would roll that write back.
func (s *service) commitThen(ctx context.Context, fn func(q *store.Queries) (outcome error, err error)) error {
	return s.commitThenTx(ctx, func(_ pgx.Tx, q *store.Queries) (error, error) { return fn(q) })
}

// commitThenTx is commitThen for steps that also hand the transaction to another feature.
func (s *service) commitThenTx(ctx context.Context, fn func(tx pgx.Tx, q *store.Queries) (outcome error, err error)) error {
	var outcome error
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		outcome, err = fn(tx, store.New(tx))
		return err
	})
	if err != nil {
		return err
	}
	return outcome
}

func (s *service) Signup(ctx context.Context, in SignupInput) (uuid.UUID, error) {
	email := normalizeEmail(in.Email)
	fullName, err := normalizeFullName(in.FullName)
	if err != nil {
		return uuid.Nil, err
	}
	var userID uuid.UUID
	var code string
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		id, created, err := s.signupUser(ctx, q, newUser{email: email, password: in.Password.Reveal(), fullName: fullName})
		if err != nil {
			return err
		}
		userID = id
		if created && s.deps.OnSignup != nil {
			if err := s.deps.OnSignup.OnUserCreated(ctx, tx, id, email); err != nil {
				return err
			}
		}
		code, err = s.issueEmailCode(ctx, q, id)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	s.sendCode(ctx, email, code)
	return userID, nil
}

// newUser is a signup's normalized input.
type newUser struct {
	email, password string
	fullName        *string
}

// signupUser inserts a new user (created = true), or returns the existing UNVERIFIED user
// when the password matches (a re-sent code). Any other existing email is auth.email_taken.
func (s *service) signupUser(ctx context.Context, q *store.Queries, in newUser) (id uuid.UUID, created bool, err error) {
	existing, err := q.GetUserByEmail(ctx, in.email)
	switch {
	case err == nil:
		if existing.EmailVerifiedAt != nil || !verifyPassword(existing.PasswordHash, in.password) {
			return uuid.Nil, false, apperr.New(apperr.CodeEmailTaken, nil)
		}
		return existing.ID, false, nil
	case !db.IsNoRows(err):
		return uuid.Nil, false, err
	}
	id = ids.New()
	err = q.InsertUser(ctx, store.InsertUserParams{
		ID: id, Email: in.email, PasswordHash: hashPassword(in.password), FullName: in.fullName, CreatedAt: s.now(),
	})
	if db.IsUniqueViolation(err, "") {
		return uuid.Nil, false, apperr.Wrap(apperr.CodeEmailTaken, err, nil)
	}
	return id, err == nil, err
}

// issueEmailCode replaces the user's active codes with a new one; at most emailCodesPerWindow
// codes per window (R-ID-1 rate limit).
func (s *service) issueEmailCode(ctx context.Context, q *store.Queries, userID uuid.UUID) (string, error) {
	now := s.now()
	issued, err := q.CountEmailCodesSince(ctx, store.CountEmailCodesSinceParams{
		UserID: userID, Purpose: purposeVerifyEmail, Since: now.Add(-emailCodeWindow),
	})
	if err != nil {
		return "", err
	}
	if issued >= emailCodesPerWindow {
		return "", apperr.New(apperr.CodeRateLimited, nil)
	}
	if err := q.ConsumeActiveEmailCodes(ctx, store.ConsumeActiveEmailCodesParams{
		UserID: userID, Purpose: purposeVerifyEmail, ConsumedAt: &now,
	}); err != nil {
		return "", err
	}
	code := s.newCode()
	return code, q.InsertEmailCode(ctx, store.InsertEmailCodeParams{
		ID: ids.New(), UserID: userID, Purpose: purposeVerifyEmail, CodeHash: emailCodeHash(userID, code),
		ExpiresAt: now.Add(emailCodeTTL), CreatedAt: now,
	})
}

// newCode is random, except in development with DEV_FIXED_EMAIL_CODE (deterministic demo;
// config refuses it in production and this check keeps it off there regardless).
func (s *service) newCode() string {
	if fixed := s.deps.Config.DevFixedEmailCode; fixed != "" && !s.deps.Config.IsProduction() {
		return fixed
	}
	return newEmailCode()
}

// sendCode runs after commit (no network call inside a transaction, AG11). The account
// exists either way; a failed delivery is reported in the log and the user re-sends by
// signing up again with the same credentials.
func (s *service) sendCode(ctx context.Context, email, code string) {
	if err := s.deps.Codes.SendEmailCode(ctx, email, code); err != nil {
		slog.ErrorContext(ctx, "email code delivery failed", "code", string(apperr.CodeOf(err)), "err", err.Error())
	}
}

func (s *service) ResendEmailCode(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	var code string
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		user, err := q.GetUserByEmail(ctx, email)
		if db.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if user.EmailVerifiedAt != nil {
			return nil
		}
		code, err = s.issueEmailCode(ctx, q, user.ID)
		return err
	})
	if err != nil || code == "" {
		return err
	}
	s.sendCode(ctx, email, code)
	return nil
}

// VerifyEmail consumes the code and, in the same transaction, accepts the ERP import invites
// waiting for this email (they become memberships the moment the email is proven).
func (s *service) VerifyEmail(ctx context.Context, email, code string) error {
	return s.commitThenTx(ctx, func(tx pgx.Tx, q *store.Queries) (error, error) {
		user, err := q.GetUserByEmail(ctx, normalizeEmail(email))
		if db.IsNoRows(err) {
			return apperr.New(apperr.CodeCodeInvalid, nil), nil
		}
		if err != nil {
			return nil, err
		}
		if user.EmailVerifiedAt != nil {
			return apperr.New(apperr.CodeCodeInvalid, nil), nil
		}
		outcome, err := s.consumeEmailCode(ctx, q, user.ID, code)
		if outcome != nil || err != nil {
			return outcome, err
		}
		return nil, s.acceptInvites(ctx, tx, user.ID, user.Email)
	})
}

// acceptInvites turns the pending ERP import invites of a verified email into memberships.
func (s *service) acceptInvites(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) error {
	if s.deps.Invites == nil {
		return nil
	}
	n, err := s.deps.Invites.AcceptERPInvites(ctx, tx, userID, email)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.InfoContext(ctx, "erp invites accepted", "user_id", userID.String(), "count", n)
	}
	return nil
}

// consumeEmailCode checks the latest active code. A wrong code counts an attempt; the
// emailCodeMaxAttempts-th wrong attempt burns the code, so 6 digits cannot be brute-forced.
func (s *service) consumeEmailCode(ctx context.Context, q *store.Queries, userID uuid.UUID, code string) (error, error) {
	now := s.now()
	row, err := q.GetActiveEmailCodeForUpdate(ctx, store.GetActiveEmailCodeForUpdateParams{
		UserID: userID, Purpose: purposeVerifyEmail, Now: now,
	})
	if db.IsNoRows(err) {
		return apperr.New(apperr.CodeCodeInvalid, nil), nil
	}
	if err != nil {
		return nil, err
	}
	if !equalHash(row.CodeHash, emailCodeHash(userID, code)) {
		if row.Attempts+1 >= emailCodeMaxAttempts {
			_, err = q.ConsumeEmailCode(ctx, store.ConsumeEmailCodeParams{ID: row.ID, ConsumedAt: &now})
		} else {
			err = q.IncrementEmailCodeAttempts(ctx, row.ID)
		}
		return apperr.New(apperr.CodeCodeInvalid, nil), err
	}
	if n, err := q.ConsumeEmailCode(ctx, store.ConsumeEmailCodeParams{ID: row.ID, ConsumedAt: &now}); err != nil || n != 1 {
		return apperr.New(apperr.CodeCodeInvalid, nil), err
	}
	return nil, q.MarkEmailVerified(ctx, store.MarkEmailVerifiedParams{ID: userID, VerifiedAt: &now})
}

func (s *service) Login(ctx context.Context, in LoginInput) (Tokens, error) {
	q := store.New(s.deps.Pool)
	user, err := q.GetUserByEmail(ctx, normalizeEmail(in.Email))
	if db.IsNoRows(err) {
		verifyPassword(s.dummyHash, in.Password.Reveal())
		return Tokens{}, apperr.New(apperr.CodeInvalidCredentials, nil)
	}
	if err != nil {
		return Tokens{}, err
	}
	if !verifyPassword(user.PasswordHash, in.Password.Reveal()) || user.Status != statusActive {
		return Tokens{}, apperr.New(apperr.CodeInvalidCredentials, nil)
	}
	if user.EmailVerifiedAt == nil {
		return Tokens{}, apperr.New(apperr.CodeEmailNotVerified, nil)
	}
	if err := s.checkTOTP(user, in.TOTPCode); err != nil {
		return Tokens{}, err
	}
	if in.DeviceID != nil {
		if _, err := q.GetDevice(ctx, store.GetDeviceParams{ID: *in.DeviceID, UserID: user.ID}); err != nil {
			return Tokens{}, notFoundAs(err, apperr.CodeDeviceNotFound)
		}
	}
	s.acceptInvitesAtLogin(ctx, user.ID, user.Email)
	return s.openSession(ctx, user.ID, in.DeviceID)
}

// acceptInvitesAtLogin covers users who already existed when the ERP import invited them. A
// failure never blocks the login: it is logged and retried at the next login.
func (s *service) acceptInvitesAtLogin(ctx context.Context, userID uuid.UUID, email string) {
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return s.acceptInvites(ctx, tx, userID, email)
	})
	if err != nil {
		slog.ErrorContext(ctx, "accepting erp invites at login failed", "user_id", userID.String(), "err", err.Error())
	}
}

// checkTOTP enforces the second factor of users who enabled TOTP (R-ID-2).
func (s *service) checkTOTP(user store.User, code *string) error {
	if user.TotpEnabledAt == nil {
		return nil
	}
	if code == nil {
		return apperr.New(apperr.CodeMFARequired, nil)
	}
	if !verifyTOTP(user.TotpSecret, *code, s.now()) {
		return apperr.New(apperr.CodeInvalidCredentials, nil)
	}
	return nil
}

func (s *service) openSession(ctx context.Context, userID uuid.UUID, deviceID *uuid.UUID) (Tokens, error) {
	sessionID := ids.New()
	var refresh string
	var refreshExp time.Time
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := q.InsertSession(ctx, store.InsertSessionParams{ID: sessionID, UserID: userID, DeviceID: deviceID, CreatedAt: s.now()}); err != nil {
			return err
		}
		var err error
		refresh, refreshExp, _, err = s.insertRefreshToken(ctx, q, sessionID)
		return err
	})
	if err != nil {
		return Tokens{}, err
	}
	return s.tokens(userID, sessionID, refresh, refreshExp)
}

func (s *service) insertRefreshToken(ctx context.Context, q *store.Queries, sessionID uuid.UUID) (string, time.Time, uuid.UUID, error) {
	now := s.now()
	raw, id, exp := newOpaqueToken(), ids.New(), now.Add(s.deps.Config.RefreshTTL)
	err := q.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
		ID: id, SessionID: sessionID, TokenHash: tokenHash(raw), ExpiresAt: exp, CreatedAt: now,
	})
	return raw, exp, id, err
}

func (s *service) tokens(userID, sessionID uuid.UUID, refresh string, refreshExp time.Time) (Tokens, error) {
	access, accessExp, err := s.deps.Issuer.IssueAccess(userID, sessionID)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: refresh, RefreshExpiresAt: refreshExp}, nil
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	var userID, sessionID uuid.UUID
	var refresh string
	var refreshExp time.Time
	err := s.commitThen(ctx, func(q *store.Queries) (error, error) {
		rt, err := q.GetRefreshTokenByHashForUpdate(ctx, tokenHash(refreshToken))
		if db.IsNoRows(err) {
			return apperr.New(apperr.CodeRefreshInvalid, nil), nil
		}
		if err != nil {
			return nil, err
		}
		session, err := q.GetSessionForUpdate(ctx, rt.SessionID)
		if err != nil {
			return nil, err
		}
		if rt.UsedAt != nil {
			// Reuse of a consumed token means it leaked: end the whole session (R-ID-2).
			slog.WarnContext(ctx, "refresh token reuse: session revoked", "session_id", session.ID.String())
			return apperr.New(apperr.CodeRefreshInvalid, nil), q.RevokeSession(ctx, store.RevokeSessionParams{ID: session.ID, RevokedAt: s.nowRef()})
		}
		if outcome, err := s.checkRefreshable(ctx, q, rt, session); outcome != nil || err != nil {
			return outcome, err
		}
		userID, sessionID = session.UserID, session.ID
		return s.rotate(ctx, q, rt, &refresh, &refreshExp)
	})
	if err != nil {
		return Tokens{}, err
	}
	return s.tokens(userID, sessionID, refresh, refreshExp)
}

// checkRefreshable rejects an expired token, a revoked session and a disabled user.
func (s *service) checkRefreshable(ctx context.Context, q *store.Queries, rt store.RefreshToken, session store.Session) (error, error) {
	if session.RevokedAt != nil || !rt.ExpiresAt.After(s.now()) {
		return apperr.New(apperr.CodeRefreshInvalid, nil), nil
	}
	user, err := q.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != statusActive {
		return apperr.New(apperr.CodeRefreshInvalid, nil), nil
	}
	return nil, nil
}

// rotate issues the next token of the chain and consumes the presented one.
func (s *service) rotate(ctx context.Context, q *store.Queries, rt store.RefreshToken, refresh *string, refreshExp *time.Time) (error, error) {
	raw, exp, nextID, err := s.insertRefreshToken(ctx, q, rt.SessionID)
	if err != nil {
		return nil, err
	}
	n, err := q.MarkRefreshTokenUsed(ctx, store.MarkRefreshTokenUsedParams{ID: rt.ID, UsedAt: s.nowRef(), ReplacedBy: &nextID})
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return apperr.New(apperr.CodeRefreshInvalid, nil), nil
	}
	*refresh, *refreshExp = raw, exp
	return nil, q.TouchSession(ctx, store.TouchSessionParams{ID: rt.SessionID, LastSeenAt: s.now()})
}

func (s *service) Logout(ctx context.Context, p auth.Principal) error {
	return store.New(s.deps.Pool).RevokeSession(ctx, store.RevokeSessionParams{ID: p.SessionID, RevokedAt: s.nowRef()})
}

func (s *service) CreateStepUpChallenge(ctx context.Context, p auth.Principal, deviceID *uuid.UUID) (StepUpChallenge, error) {
	q := store.New(s.deps.Pool)
	if deviceID != nil {
		if _, err := q.GetDevice(ctx, store.GetDeviceParams{ID: *deviceID, UserID: p.UserID}); err != nil {
			return StepUpChallenge{}, notFoundAs(err, apperr.CodeDeviceNotFound)
		}
	}
	now := s.now()
	ch := StepUpChallenge{ID: ids.New(), Nonce: randomBytes(32), ExpiresAt: now.Add(challengeTTL)}
	err := q.InsertStepUpChallenge(ctx, store.InsertStepUpChallengeParams{
		ID: ch.ID, UserID: p.UserID, SessionID: p.SessionID, DeviceID: deviceID, Nonce: ch.Nonce,
		ExpiresAt: ch.ExpiresAt, CreatedAt: now,
	})
	if err != nil {
		return StepUpChallenge{}, err
	}
	return ch, nil
}

// VerifyStepUp burns the challenge on EVERY answer (right or wrong), so each challenge gives
// exactly one guess; a right answer gets a token bound to this session (R-ID-3).
func (s *service) VerifyStepUp(ctx context.Context, p auth.Principal, in StepUpInput) (StepUpToken, error) {
	err := s.commitThen(ctx, func(q *store.Queries) (error, error) {
		ch, err := q.GetStepUpChallengeForUpdate(ctx, store.GetStepUpChallengeForUpdateParams{
			ID: in.ChallengeID, UserID: p.UserID, SessionID: p.SessionID,
		})
		if db.IsNoRows(err) {
			return apperr.New(apperr.CodeChallengeInvalid, nil), nil
		}
		if err != nil {
			return nil, err
		}
		if ch.UsedAt != nil || !ch.ExpiresAt.After(s.now()) {
			return apperr.New(apperr.CodeChallengeInvalid, nil), nil
		}
		n, err := q.MarkStepUpChallengeUsed(ctx, store.MarkStepUpChallengeUsedParams{ID: ch.ID, UsedAt: s.nowRef()})
		if err != nil || n != 1 {
			return apperr.New(apperr.CodeChallengeInvalid, nil), err
		}
		return s.checkStepUpAnswer(ctx, q, p, ch, in)
	})
	if err != nil {
		return StepUpToken{}, err
	}
	token, exp, err := s.deps.Issuer.IssueStepUp(p.UserID, p.SessionID)
	if err != nil {
		return StepUpToken{}, err
	}
	return StepUpToken{Token: token, ExpiresAt: exp}, nil
}

func (s *service) checkStepUpAnswer(ctx context.Context, q *store.Queries, p auth.Principal, ch store.StepUpChallenge, in StepUpInput) (error, error) {
	switch in.Method {
	case StepUpDeviceSignature:
		if ch.DeviceID == nil || (in.DeviceID != nil && *in.DeviceID != *ch.DeviceID) {
			return apperr.New(apperr.CodeChallengeInvalid, nil), nil
		}
		device, err := q.GetDevice(ctx, store.GetDeviceParams{ID: *ch.DeviceID, UserID: p.UserID})
		if db.IsNoRows(err) {
			return apperr.New(apperr.CodeChallengeInvalid, nil), nil
		}
		if err != nil {
			return nil, err
		}
		if len(in.Signature) != ed25519.SignatureSize || !ed25519.Verify(device.PublicKey, ch.Nonce, in.Signature) {
			return apperr.New(apperr.CodeChallengeInvalid, nil), nil
		}
		return nil, nil
	case StepUpPassword:
		user, err := q.GetUserByID(ctx, p.UserID)
		if err != nil {
			return nil, err
		}
		if !verifyPassword(user.PasswordHash, in.Password.Reveal()) {
			return apperr.New(apperr.CodeInvalidCredentials, nil), nil
		}
		return nil, nil
	default:
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/method"}), nil
	}
}

func (s *service) RegisterDevice(ctx context.Context, p auth.Principal, in DeviceInput) (Device, error) {
	if len(in.PublicKey) != ed25519.PublicKeySize {
		return Device{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/publicKey"})
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Device{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/name"})
	}
	d := Device{ID: ids.New(), Name: name, CreatedAt: s.now()}
	err := store.New(s.deps.Pool).InsertDevice(ctx, store.InsertDeviceParams{
		ID: d.ID, UserID: p.UserID, PublicKey: in.PublicKey, Name: name, CreatedAt: d.CreatedAt,
	})
	if err != nil {
		return Device{}, err
	}
	return d, nil
}

func (s *service) Me(ctx context.Context, p auth.Principal) (Me, error) {
	q := store.New(s.deps.Pool)
	user, err := q.GetUserByID(ctx, p.UserID)
	if err != nil {
		return Me{}, notFoundAs(err, apperr.CodeUnauthenticated)
	}
	memberships, err := s.deps.Memberships.MembershipsForUser(ctx, p.UserID)
	if err != nil {
		return Me{}, err
	}
	prefs, err := s.storedPreferences(ctx, q, p.UserID, memberships)
	if err != nil {
		return Me{}, err
	}
	return Me{
		User: User{
			ID: user.ID, Email: user.Email, FullName: user.FullName, EmailVerified: user.EmailVerifiedAt != nil,
			MFAEnabled: user.TotpEnabledAt != nil, CreatedAt: user.CreatedAt,
		},
		Memberships: memberships,
		Preferences: prefs,
	}, nil
}

func (s *service) UserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	user, err := store.New(s.deps.Pool).GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return uuid.Nil, notFoundAs(err, apperr.CodeUserNotFound)
	}
	if user.Status != statusActive {
		return uuid.Nil, apperr.New(apperr.CodeUserNotFound, nil)
	}
	return user.ID, nil
}

// notFoundAs maps "no rows" to the feature's not-found code and passes other errors through.
func notFoundAs(err error, code apperr.Code) error {
	if db.IsNoRows(err) {
		return apperr.Wrap(code, err, nil)
	}
	return err
}

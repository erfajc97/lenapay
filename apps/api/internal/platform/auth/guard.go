package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// Headers and path parameters read by the guard.
const (
	HeaderStepUp      = "X-Step-Up-Token"
	HeaderInternalKey = "X-Internal-Api-Key"
	// OrgPathParam is the path parameter of every org route (contracts/openapi.yaml).
	OrgPathParam = "orgId"
)

// ErrNotFound is returned by a Directory when the session or membership does not exist.
var ErrNotFound = errors.New("auth: not found")

// Membership is the caller's role in an org plus the org's KYC status.
type Membership struct {
	Role      Role
	KYCStatus string
}

// Directory answers the two questions the guard asks on every authenticated request.
type Directory interface {
	// ActiveSession returns nil when the user is active and the session is not revoked.
	ActiveSession(ctx context.Context, userID, sessionID uuid.UUID) error
	// Membership returns the user's membership in org, or ErrNotFound.
	Membership(ctx context.Context, orgID, userID uuid.UUID) (Membership, error)
}

// Verifier checks tokens of the identity provider (*Issuer; Cognito later, R-ID-4).
type Verifier interface {
	VerifyAccess(raw string) (Claims, error)
	VerifyStepUp(raw string, userID, sessionID uuid.UUID) error
}

// Guard enforces the rules on every route: strict-server operations and hand-mounted ones.
type Guard struct {
	rules       Rules
	verifier    Verifier
	directory   Directory
	internalKey config.Secret
}

// NewGuard builds the guard.
func NewGuard(rules Rules, verifier Verifier, directory Directory, internalKey config.Secret) *Guard {
	return &Guard{rules: rules, verifier: verifier, directory: directory, internalKey: internalKey}
}

// Authorize applies the rule of operationID to the request (pathParams are the spec path
// parameters the router matched) and returns the context with the principal. An operation
// without a rule is denied (DEFAULT DENY).
func (g *Guard) Authorize(ctx context.Context, r *http.Request, operationID string, pathParams map[string]string) (context.Context, error) {
	rule, ok := g.rules[operationID]
	if !ok || rule.IsDeny() {
		slog.ErrorContext(ctx, "operation without an access rule: denied", "operation", operationID)
		return ctx, apperr.New(apperr.CodeForbidden, nil)
	}
	switch {
	case rule.IsInternal():
		got := []byte(r.Header.Get(HeaderInternalKey))
		want := []byte(g.internalKey.Reveal())
		if len(want) == 0 || subtle.ConstantTimeCompare(got, want) != 1 {
			return ctx, apperr.New(apperr.CodeUnauthenticated, nil)
		}
		return ctx, nil
	case !rule.NeedsToken():
		return ctx, nil
	}
	p, err := g.principal(ctx, r, rule, pathParams)
	if err != nil {
		return ctx, err
	}
	return WithPrincipal(ctx, p), nil
}

func (g *Guard) principal(ctx context.Context, r *http.Request, rule Rule, pathParams map[string]string) (Principal, error) {
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || raw == "" {
		return Principal{}, apperr.New(apperr.CodeUnauthenticated, nil)
	}
	claims, err := g.verifier.VerifyAccess(raw)
	if err != nil {
		return Principal{}, apperr.Wrap(apperr.CodeUnauthenticated, err, nil)
	}
	userID, err := claims.UserID()
	if err != nil {
		return Principal{}, apperr.Wrap(apperr.CodeUnauthenticated, err, nil)
	}
	if err := g.directory.ActiveSession(ctx, userID, claims.SessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return Principal{}, apperr.New(apperr.CodeUnauthenticated, nil)
		}
		return Principal{}, err
	}
	p := Principal{UserID: userID, SessionID: claims.SessionID}
	if rule.IsOrg() {
		if err := g.resolveOrg(ctx, rule, pathParams, &p); err != nil {
			return Principal{}, err
		}
	}
	if rule.NeedsStepUp() {
		token := r.Header.Get(HeaderStepUp)
		if token == "" || g.verifier.VerifyStepUp(token, p.UserID, p.SessionID) != nil {
			return Principal{}, apperr.New(apperr.CodeStepUpRequired, nil)
		}
		p.StepUp = true
	}
	return p, nil
}

// resolveOrg loads the membership. Not a member answers exactly like a missing org (404,
// T2); a member below the route's role gets 403.
func (g *Guard) resolveOrg(ctx context.Context, rule Rule, pathParams map[string]string, p *Principal) error {
	orgID, err := uuid.Parse(pathParams[OrgPathParam])
	if err != nil {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	m, err := g.directory.Membership(ctx, orgID, p.UserID)
	if errors.Is(err, ErrNotFound) {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if err != nil {
		return err
	}
	if !m.Role.AtLeast(rule.MinRole()) {
		return apperr.New(apperr.CodeForbidden, map[string]any{"requiredRole": string(rule.MinRole())})
	}
	if rule.NeedsKYC() && m.KYCStatus != KYCApproved {
		return apperr.New(apperr.CodeKYCNotApproved, nil)
	}
	p.OrgID, p.Role, p.KYCStatus = orgID, m.Role, m.KYCStatus
	return nil
}

// PGDirectory is the Directory backed by the users, sessions, memberships and organizations
// tables (read-only; the identity and orgs features own and write them).
type PGDirectory struct {
	pool *pgxpool.Pool
}

// NewPGDirectory returns the Postgres directory.
func NewPGDirectory(pool *pgxpool.Pool) *PGDirectory {
	return &PGDirectory{pool: pool}
}

// ActiveSession implements Directory.
func (d *PGDirectory) ActiveSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	var one int
	err := d.pool.QueryRow(ctx, `
		SELECT 1 FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.user_id = $2 AND s.revoked_at IS NULL AND u.status = 'active'`,
		sessionID, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("auth: load session: %w", err)
	}
	return nil
}

// Membership implements Directory.
func (d *PGDirectory) Membership(ctx context.Context, orgID, userID uuid.UUID) (Membership, error) {
	var role, kyc string
	err := d.pool.QueryRow(ctx, `
		SELECT m.role, o.kyc_status FROM memberships m JOIN organizations o ON o.id = m.org_id
		WHERE m.org_id = $1 AND m.user_id = $2`, orgID, userID).Scan(&role, &kyc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, fmt.Errorf("auth: load membership: %w", err)
	}
	return Membership{Role: Role(role), KYCStatus: kyc}, nil
}

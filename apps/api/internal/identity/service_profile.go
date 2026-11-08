package identity

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/identity/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/db"
)

// normalizeFullName trims a full name; nil stays nil, blank or longer than maxFullName runes is
// validation.failed (the contract caps it first; this keeps every door the same, G12).
func normalizeFullName(name *string) (*string, error) {
	if name == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*name)
	if n := utf8.RuneCountInString(trimmed); n < 1 || n > maxFullName {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "fullName"})
	}
	return &trimmed, nil
}

func (s *service) UpdateProfile(ctx context.Context, p auth.Principal, fullName Patch[string]) (Me, error) {
	if fullName.Set {
		name, err := normalizeFullName(fullName.Value)
		if err != nil {
			return Me{}, err
		}
		n, err := store.New(s.deps.Pool).SetUserFullName(ctx, store.SetUserFullNameParams{FullName: name, UpdatedAt: s.now(), ID: p.UserID})
		if err != nil {
			return Me{}, err
		}
		if n != 1 {
			return Me{}, apperr.New(apperr.CodeUnauthenticated, nil)
		}
	}
	return s.Me(ctx, p)
}

func (s *service) FillFullNameIfEmpty(ctx context.Context, tx pgx.Tx, userID uuid.UUID, name string) error {
	normalized, err := normalizeFullName(&name)
	if err != nil {
		return err
	}
	_, err = store.New(tx).FillUserFullNameIfEmpty(ctx, store.FillUserFullNameIfEmptyParams{FullName: normalized, UpdatedAt: s.now(), ID: userID})
	return err
}

func (s *service) Preferences(ctx context.Context, p auth.Principal) (Preferences, error) {
	memberships, err := s.deps.Memberships.MembershipsForUser(ctx, p.UserID)
	if err != nil {
		return Preferences{}, err
	}
	return s.storedPreferences(ctx, store.New(s.deps.Pool), p.UserID, memberships)
}

// storedPreferences reads the user's row (zero value when none). A default org the user no
// longer belongs to — and its account — reads as unset: preferences never point at an org the
// user cannot open (T2).
func (s *service) storedPreferences(ctx context.Context, q *store.Queries, userID uuid.UUID, memberships []MembershipSummary) (Preferences, error) {
	row, err := q.GetUserPreferences(ctx, userID)
	if db.IsNoRows(err) {
		return Preferences{}, nil
	}
	if err != nil {
		return Preferences{}, err
	}
	out := Preferences{
		DefaultOrgID: row.DefaultOrgID, DefaultAccountID: row.DefaultAccountID, Locale: row.Locale, Theme: row.Theme,
		UpdatedAt: &row.UpdatedAt,
	}
	if out.DefaultOrgID != nil && !isMember(memberships, *out.DefaultOrgID) {
		out.DefaultOrgID, out.DefaultAccountID = nil, nil
	}
	return out, nil
}

func isMember(memberships []MembershipSummary, orgID uuid.UUID) bool {
	return slices.ContainsFunc(memberships, func(m MembershipSummary) bool { return m.OrgID == orgID })
}

func (s *service) UpdatePreferences(ctx context.Context, p auth.Principal, in PreferencesPatch) (Preferences, error) {
	memberships, err := s.deps.Memberships.MembershipsForUser(ctx, p.UserID)
	if err != nil {
		return Preferences{}, err
	}
	q := store.New(s.deps.Pool)
	stored, err := s.storedPreferences(ctx, q, p.UserID, memberships)
	if err != nil {
		return Preferences{}, err
	}
	merged := Preferences{
		DefaultOrgID:     in.DefaultOrgID.apply(stored.DefaultOrgID),
		DefaultAccountID: in.DefaultAccountID.apply(stored.DefaultAccountID),
		Locale:           in.Locale.apply(stored.Locale),
		Theme:            in.Theme.apply(stored.Theme),
	}
	if err := s.validatePreferences(ctx, merged, memberships); err != nil {
		return Preferences{}, err
	}
	row, err := q.UpsertUserPreferences(ctx, store.UpsertUserPreferencesParams{
		UserID: p.UserID, DefaultOrgID: merged.DefaultOrgID, DefaultAccountID: merged.DefaultAccountID,
		Locale: merged.Locale, Theme: merged.Theme, UpdatedAt: s.now(),
	})
	if err != nil {
		return Preferences{}, err
	}
	return Preferences{
		DefaultOrgID: row.DefaultOrgID, DefaultAccountID: row.DefaultAccountID, Locale: row.Locale, Theme: row.Theme,
		UpdatedAt: &row.UpdatedAt,
	}, nil
}

// validatePreferences checks the MERGED state (A4): an account needs its org, the org must be
// one of the user's (org.not_found otherwise, like any foreign org, T2) and the account a live
// account of that org.
func (s *service) validatePreferences(ctx context.Context, merged Preferences, memberships []MembershipSummary) error {
	if merged.Locale != nil && !validLocale(*merged.Locale) {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "locale"})
	}
	if merged.Theme != nil && !validTheme(*merged.Theme) {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "theme"})
	}
	if merged.DefaultAccountID != nil && merged.DefaultOrgID == nil {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "defaultAccountId"})
	}
	if merged.DefaultOrgID == nil {
		return nil
	}
	if !isMember(memberships, *merged.DefaultOrgID) {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if merged.DefaultAccountID == nil {
		return nil
	}
	archived, err := s.deps.Accounts.AccountArchived(ctx, *merged.DefaultOrgID, *merged.DefaultAccountID)
	if err != nil {
		return err
	}
	if archived {
		return apperr.New(apperr.CodeAccountArchived, nil)
	}
	return nil
}

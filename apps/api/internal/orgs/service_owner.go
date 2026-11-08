package orgs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/orgs/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/db"
)

// TransferOwnership: the owner (step-up) hands the organization to another member and stays as
// admin. The caller's row is locked first and re-read, so two transfers at once cannot both
// succeed (the second sees the caller is no longer the owner). The DB keeps exactly one owner
// (unique partial index + deferred trigger), so the demotion runs before the promotion.
func (s *service) TransferOwnership(ctx context.Context, p auth.Principal, userID uuid.UUID) (Member, Member, error) {
	if err := requireOwner(p); err != nil {
		return Member{}, Member{}, err
	}
	if userID == p.UserID {
		return Member{}, Member{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/userId"})
	}
	var previous, owner Member
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		caller, err := q.GetMembershipForUpdate(ctx, store.GetMembershipForUpdateParams{OrgID: p.OrgID, UserID: p.UserID})
		if err != nil {
			return notFoundAs(err, apperr.CodeOrgNotFound)
		}
		if auth.Role(caller.Role) != auth.RoleOwner {
			return apperr.New(apperr.CodeForbidden, map[string]any{"requiredRole": string(auth.RoleOwner)})
		}
		target, err := q.GetMembershipForUpdate(ctx, store.GetMembershipForUpdateParams{OrgID: p.OrgID, UserID: userID})
		if err != nil {
			return notFoundAs(err, apperr.CodeMemberNotFound)
		}
		now := s.now()
		for _, change := range []struct {
			user uuid.UUID
			role auth.Role
		}{{p.UserID, auth.RoleAdmin}, {userID, auth.RoleOwner}} {
			if err := q.UpdateMembershipRole(ctx, store.UpdateMembershipRoleParams{
				Role: string(change.role), UpdatedAt: now, OrgID: p.OrgID, UserID: change.user,
			}); err != nil {
				return err
			}
		}
		if previous, err = getMember(ctx, q, p.OrgID, p.UserID); err != nil {
			return err
		}
		if owner, err = getMember(ctx, q, p.OrgID, userID); err != nil {
			return err
		}
		return s.record(ctx, tx, p, p.OrgID, "org.ownership_transfer", "user", userID, map[string]any{
			"from": p.UserID.String(), "to": userID.String(), "previousRoleOfNewOwner": target.Role,
		})
	})
	if err != nil {
		return Member{}, Member{}, err
	}
	return previous, owner, nil
}

// requireOwner is the service-side copy of the owner-only route rules (ownership, approval
// policy): the guard enforces them first.
func requireOwner(p auth.Principal) error {
	if !p.HasOrg() {
		return apperr.New(apperr.CodeOrgNotFound, nil)
	}
	if p.Role != auth.RoleOwner {
		return apperr.New(apperr.CodeForbidden, map[string]any{"requiredRole": string(auth.RoleOwner)})
	}
	if !p.StepUp {
		return apperr.New(apperr.CodeStepUpRequired, nil)
	}
	return nil
}

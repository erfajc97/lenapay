package orgs

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"lenapay/apps/api/internal/orgs/store"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/audit"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/db"
	"lenapay/apps/api/internal/platform/ids"
)

// capERPRole: an ERP user is never invited above the admin who linked the org, and never as
// owner (the owner is one person and only moves by transfer).
func capERPRole(mapped, linker auth.Role) auth.Role {
	ceiling := linker
	if ceiling == auth.RoleOwner {
		ceiling = auth.RoleAdmin
	}
	if !ceiling.AtLeast(mapped) {
		return ceiling
	}
	return mapped
}

// pendingInvite is an invite written in the transaction and emailed after commit.
type pendingInvite struct {
	email, token string
}

func (s *service) InviteERPUsers(ctx context.Context, in ERPInviteInput) (ERPInviteResult, error) {
	var res ERPInviteResult
	var toSend []pendingInvite
	var orgName string
	err := db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		res, toSend, orgName = ERPInviteResult{Invited: []ERPInvitee{}}, nil, ""
		q := store.New(tx)
		linker, err := q.GetMembership(ctx, store.GetMembershipParams{OrgID: in.OrgID, UserID: in.InvitedBy})
		if err != nil {
			return notFoundAs(err, apperr.CodeMemberNotFound)
		}
		org, err := q.GetOrganization(ctx, in.OrgID)
		if err != nil {
			return notFoundAs(err, apperr.CodeOrgNotFound)
		}
		orgName = org.LegalName
		for _, invitee := range in.Invitees {
			sent, member, err := s.inviteERPUser(ctx, tx, in, invitee, auth.Role(linker.Role))
			if err != nil {
				return err
			}
			switch {
			case member:
				res.AlreadyMembers++
			default:
				res.Invited = append(res.Invited, sent.invitee)
				if sent.token != "" {
					toSend = append(toSend, pendingInvite{email: sent.invitee.Email, token: sent.token})
				}
			}
		}
		return nil
	})
	if err != nil {
		return ERPInviteResult{}, err
	}
	// After commit, like any invite (AG11): the invite exists and is auto-accepted on email
	// verification even if the email never arrives.
	for _, p := range toSend {
		if err := s.deps.Mailer.SendInvite(ctx, p.email, orgName, p.token); err != nil {
			slog.ErrorContext(ctx, "erp invite delivery failed", "org_id", in.OrgID.String(), "err", err.Error())
		}
	}
	return res, nil
}

// erpInviteOutcome is one invitee's result: the (capped) invite, and the token when a new invite
// was written ("" when a pending one already existed: the import is idempotent).
type erpInviteOutcome struct {
	invitee ERPInvitee
	token   string
}

func (s *service) inviteERPUser(ctx context.Context, tx pgx.Tx, in ERPInviteInput, invitee ERPInvitee, linker auth.Role) (erpInviteOutcome, bool, error) {
	email := strings.ToLower(strings.TrimSpace(invitee.Email))
	role := capERPRole(invitee.Role, linker)
	if !role.Grantable() {
		return erpInviteOutcome{}, false, apperr.New(apperr.CodeValidation, map[string]any{"field": "role"})
	}
	member, err := s.isMember(ctx, tx, in.OrgID, email)
	if err != nil || member {
		return erpInviteOutcome{}, member, err
	}
	out := erpInviteOutcome{invitee: ERPInvitee{Email: email, Role: role}}
	q := store.New(tx)
	now := s.now()
	pending, err := q.HasPendingInvite(ctx, store.HasPendingInviteParams{OrgID: in.OrgID, Email: email, Now: now})
	if err != nil || pending {
		return out, false, err
	}
	out.token = newInviteToken()
	id := ids.New()
	if err := q.InsertInvite(ctx, store.InsertInviteParams{
		ID: id, OrgID: in.OrgID, Email: email, Role: string(role), TokenHash: inviteTokenHash(out.token),
		InvitedBy: in.InvitedBy, ExpiresAt: now.Add(InviteTTL), CreatedAt: now, Source: inviteSourceERPImport,
	}); err != nil {
		return erpInviteOutcome{}, false, err
	}
	return out, false, s.deps.Audit.Record(ctx, tx, audit.Entry{
		OrgID: &in.OrgID, ActorID: &in.InvitedBy, ActorKind: audit.ActorUser, Action: "invite.erp_import",
		TargetType: "invite", TargetID: &id, Payload: map[string]any{"role": role},
	})
}

// isMember reports whether a registered user with this email already belongs to orgID.
func (s *service) isMember(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, email string) (bool, error) {
	userID, err := s.deps.Users.UserIDByEmail(ctx, email)
	if apperr.IsCode(err, apperr.CodeUserNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = store.New(tx).GetMembership(ctx, store.GetMembershipParams{OrgID: orgID, UserID: userID})
	if db.IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

// AcceptERPInvites runs inside the email verification (or login) transaction: every acceptable
// ERP import invite of the email becomes a membership with its role; an invite to an org the
// user already joined is just closed.
func (s *service) AcceptERPInvites(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) (int, error) {
	q := store.New(tx)
	invites, err := q.ListPendingERPInvitesForUpdate(ctx, store.ListPendingERPInvitesForUpdateParams{
		Email: strings.ToLower(strings.TrimSpace(email)), Now: s.now(),
	})
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, inv := range invites {
		joined, err := s.autoJoin(ctx, q, inv, userID)
		if err != nil {
			return accepted, err
		}
		if joined {
			accepted++
		}
		if err := s.deps.Audit.Record(ctx, tx, audit.Entry{
			OrgID: &inv.OrgID, ActorID: &userID, ActorKind: audit.ActorUser, Action: "invite.auto_accept",
			TargetType: "invite", TargetID: &inv.ID, Payload: map[string]any{"role": inv.Role, "joined": joined},
		}); err != nil {
			return accepted, err
		}
	}
	return accepted, nil
}

// autoJoin closes the invite and adds the membership unless the user is already a member.
func (s *service) autoJoin(ctx context.Context, q *store.Queries, inv store.Invite, userID uuid.UUID) (bool, error) {
	now := s.now()
	n, err := q.MarkInviteAccepted(ctx, store.MarkInviteAcceptedParams{ID: inv.ID, AcceptedAt: &now, AcceptedBy: &userID})
	if err != nil || n != 1 {
		return false, err
	}
	_, err = q.GetMembership(ctx, store.GetMembershipParams{OrgID: inv.OrgID, UserID: userID})
	if err == nil {
		return false, nil
	}
	if !db.IsNoRows(err) {
		return false, err
	}
	return true, q.InsertMembership(ctx, store.InsertMembershipParams{OrgID: inv.OrgID, UserID: userID, Role: inv.Role, CreatedAt: now})
}

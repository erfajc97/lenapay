import { useState } from 'react';
import { z } from 'zod';

import type { Member, Role } from '@/shared/api/types';
import { displayName } from '@/shared/helpers/people';
import { canEditMember, canManageTeam, canTransferOwnership, grantableRoles, ownershipCandidates } from '@/shared/helpers/permissions';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg, useMembers } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';

import { useInvite } from '../mutations/useInvite';
import { useRemoveMember, useUpdateMember } from '../mutations/useMemberMutations';
import { useTransferOwnership } from '../mutations/useTransferOwnership';

const emailSchema = z.email();

/** Ownership transfer (R-ORG-3): owner only, pick another active member, explicit warning, step-up. */
function useOwnershipTransfer(orgId: string | undefined, members: Member[], myUserId: string | null) {
  const [open, setOpen] = useState(false);
  const [targetId, setTargetId] = useState<string | null>(null);
  const close = () => {
    setOpen(false);
    setTargetId(null);
  };
  const transfer = useTransferOwnership(orgId, close);
  const candidates = ownershipCandidates(members, myUserId);
  const target = candidates.find((member) => member.userId === targetId) ?? null;
  return {
    open,
    openSheet: () => setOpen(true),
    close,
    candidates,
    target,
    pick: (member: Member) => setTargetId(member.userId),
    confirm: () => {
      if (target) transfer.mutate(target);
    },
    pending: transfer.isPending,
  };
}

export function useTeam() {
  const { org } = useActiveOrg();
  const members = useMembers(org?.id);
  const accounts = useAccounts(org?.kind === 'company' ? org.id : undefined);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<Role>('operator');
  const [touched, setTouched] = useState(false);
  const invite = useInvite(org?.id, () => {
    setSheetOpen(false);
    setEmail('');
    setTouched(false);
  });
  const emailValid = emailSchema.safeParse(email.trim()).success;
  const me = useMe();
  const myUserId = me.data?.id ?? null;
  const [editing, setEditing] = useState<Member | null>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const closeMember = () => {
    setEditing(null);
    setConfirmRemove(false);
  };
  const updateMember = useUpdateMember(org?.id, closeMember);
  const removeMember = useRemoveMember(org?.id, closeMember);
  const myRole = org?.role ?? 'viewer';
  const transfer = useOwnershipTransfer(org?.id, members.data ?? [], myUserId);

  return {
    org,
    members,
    accounts: accounts.data ?? [],
    canManage: canManageTeam(myRole),
    isOwner: myRole === 'owner',
    canTransfer: canTransferOwnership(myRole),
    // The ladder note each manager needs: the owner learns they cannot be changed, an admin that admins are the owner's.
    ladderNote: myRole === 'owner' ? 'team.ownerNote' : canManageTeam(myRole) ? 'team.adminNote' : null,
    grantable: grantableRoles(myRole),
    isMe: (member: Member) => member.userId === myUserId,
    nameOf: displayName,
    sheetOpen,
    openSheet: () => setSheetOpen(true),
    closeSheet: () => setSheetOpen(false),
    email,
    setEmail: (value: string) => {
      setEmail(value);
      setTouched(true);
    },
    emailError: touched && !emailValid ? 'validation.email' : null,
    role,
    setRole,
    submit: () => {
      setTouched(true);
      if (emailValid) invite.mutate({ email: email.trim().toLowerCase(), role });
    },
    inviting: invite.isPending,
    canEdit: (member: Member) => canEditMember({ role: myRole, myUserId, member }),
    editing,
    openMember: (member: Member) => setEditing(member),
    closeMember,
    changeRole: (next: Role) => {
      if (editing && next !== editing.role) updateMember.mutate({ member: editing, role: next });
    },
    confirmRemove,
    // Removing someone is not undoable from the app: the first tap asks, the second removes.
    remove: () => {
      if (!editing) return;
      if (confirmRemove) removeMember.mutate(editing);
      else setConfirmRemove(true);
    },
    savingMember: updateMember.isPending || removeMember.isPending,
    transfer,
  };
}

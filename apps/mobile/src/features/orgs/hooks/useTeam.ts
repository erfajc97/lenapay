import { useState } from 'react';
import { z } from 'zod';

import type { Member, Role } from '@/shared/api/types';
import { canEditMember, canManageTeam } from '@/shared/helpers/permissions';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg, useMembers } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';

import { useInvite } from '../mutations/useInvite';
import { useRemoveMember, useUpdateMember } from '../mutations/useMemberMutations';

const emailSchema = z.email();

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
  const [editing, setEditing] = useState<Member | null>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const closeMember = () => {
    setEditing(null);
    setConfirmRemove(false);
  };
  const updateMember = useUpdateMember(org?.id, closeMember);
  const removeMember = useRemoveMember(org?.id, closeMember);

  return {
    org,
    members,
    accounts: accounts.data ?? [],
    canManage: org ? canManageTeam(org.role) : false,
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
    canEdit: (member: Member) => (org ? canEditMember({ role: org.role, myUserId: me.data?.id ?? null, member }) : false),
    editing,
    openMember: (member: Member) => setEditing(member),
    closeMember,
    changeRole: (role: Role) => {
      if (editing && role !== editing.role) updateMember.mutate({ member: editing, role });
    },
    confirmRemove,
    // Removing someone is not undoable from the app: the first tap asks, the second removes.
    remove: () => {
      if (!editing) return;
      if (confirmRemove) removeMember.mutate(editing);
      else setConfirmRemove(true);
    },
    savingMember: updateMember.isPending || removeMember.isPending,
  };
}

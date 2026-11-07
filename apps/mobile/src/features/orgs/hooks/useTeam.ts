import { useState } from 'react';
import { z } from 'zod';

import type { Role } from '@/shared/api/types';
import { canManageTeam } from '@/shared/helpers/permissions';
import { useActiveOrg, useMembers } from '@/shared/queries/orgs';

import { useInvite } from '../mutations/useInvite';

const emailSchema = z.email();

export function useTeam() {
  const { org } = useActiveOrg();
  const members = useMembers(org?.id);
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

  return {
    org,
    members,
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
  };
}

import { useState } from 'react';

import { useOrgs } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';

import { fullNameUpdate } from '../helpers/profileRules';
import { useUpdateProfile } from '../mutations/useProfileMutations';

/** Settings → Perfil (R-ID-5): edit the full name; the draft starts from the server value. */
export function useProfile() {
  const me = useMe();
  const orgs = useOrgs();
  const [draft, setDraft] = useState<string | null>(null);
  const [touched, setTouched] = useState(false);
  const save = useUpdateProfile(() => {
    setDraft(null);
    setTouched(false);
  });
  const name = draft ?? me.data?.fullName ?? '';
  const update = fullNameUpdate(name);
  const unchanged = update.value === (me.data?.fullName ?? null);
  return {
    me,
    orgs,
    name,
    setName: (value: string) => {
      setDraft(value);
      setTouched(true);
    },
    nameError: touched ? update.error : null,
    canSave: me.data !== undefined && update.error === null && !unchanged && !save.isPending,
    save: () => {
      setTouched(true);
      if (update.error === null) save.mutate({ fullName: update.value });
    },
    saving: save.isPending,
  };
}

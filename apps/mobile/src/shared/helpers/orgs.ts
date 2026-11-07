import type { Org } from '@/shared/api/types';

/** Personal wallets show only with the B2C flag or its preview (PRD §2). */
export function visibleOrgs(orgs: Org[], personalMode: boolean): Org[] {
  return orgs.filter((org) => org.kind === 'company' || personalMode);
}

/** The selected organization: the remembered one if still visible, else the first. */
export function pickActiveOrg(orgs: Org[], rememberedId: string | null): Org | null {
  return orgs.find((org) => org.id === rememberedId) ?? orgs[0] ?? null;
}

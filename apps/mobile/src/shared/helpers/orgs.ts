import type { Account, Me, Org, UserPreferences } from '@/shared/api/types';

/** Personal wallets show only with the B2C flag or its preview (PRD §2). */
export function visibleOrgs(orgs: Org[], personalMode: boolean): Org[] {
  return orgs.filter((org) => org.kind === 'company' || personalMode);
}

/**
 * The selected organization: the remembered one if still visible, else the first company, else
 * the first. With personal mode on the API creates a personal wallet at signup, so a company
 * member would otherwise land on an empty, unverified wallet instead of the company.
 */
export function pickActiveOrg(orgs: Org[], rememberedId: string | null): Org | null {
  return orgs.find((org) => org.id === rememberedId) ?? orgs.find((org) => org.kind === 'company') ?? orgs[0] ?? null;
}

/**
 * "Mis pagos" is open to a person whose own personal wallet passed KYC (R-MP-1): the id of that
 * org, or null. The API answers kyc.not_approved otherwise, so the entry is not offered.
 */
export function verifiedPersonalOrgId(memberships: Me['memberships'] | undefined): string | null {
  return memberships?.find((item) => item.orgKind === 'personal' && item.kycStatus === 'approved')?.orgId ?? null;
}

/** The person's own personal wallet (signup opens it; the app never creates a second one). */
export function personalOrg(orgs: Org[] | undefined): Org | null {
  return orgs?.find((org) => org.kind === 'personal') ?? null;
}

/**
 * Which org the app opens in (R-ID-5): the one picked during this session, else the person's
 * default org (synced across devices), else the one used last on this device.
 */
export function rememberedOrgId(params: { sessionOrgId: string | null; defaultOrgId: string | null | undefined; lastOrgId: string | null }): string | null {
  return params.sessionOrgId ?? params.defaultOrgId ?? params.lastOrgId;
}

/**
 * The account Pagar / Mover start with: the default account when this is the default org and the
 * account is still open; null otherwise (the flow keeps its own fallback).
 */
export function defaultAccountId(
  openAccounts: Pick<Account, 'id'>[],
  orgId: string | undefined,
  preferences: Pick<UserPreferences, 'defaultOrgId' | 'defaultAccountId'> | undefined,
): string | null {
  if (!orgId || !preferences || preferences.defaultOrgId !== orgId) return null;
  return openAccounts.find((account) => account.id === preferences.defaultAccountId)?.id ?? null;
}

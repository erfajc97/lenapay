import type { Me, Org } from '@/shared/api/types';

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

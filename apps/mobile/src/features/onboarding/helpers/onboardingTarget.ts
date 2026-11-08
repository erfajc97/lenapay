import type { Org, OrgKind } from '@/shared/api/types';

/**
 * What "Continuar" does on the onboarding start: create a company, or — for a person — open the
 * personal wallet signup already created (R-ACC-5). The API refuses a second personal org
 * (409), so the app never asks for one when it exists.
 */
export function onboardingTarget(params: { kind: OrgKind; existingPersonal: Pick<Org, 'id'> | null }): { action: 'create' } | { action: 'open'; orgId: string } {
  if (params.kind === 'personal' && params.existingPersonal) return { action: 'open', orgId: params.existingPersonal.id };
  return { action: 'create' };
}

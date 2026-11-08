import type { KycCase, KycStatus } from '@/shared/api/types';

const STEPS: KycStatus[] = ['draft', 'submitted', 'approved'];

/** When the case reached a step: submitted and decided are on the case; a draft has no date. */
function stepDate(kyc: KycCase | undefined, step: KycStatus): string | null {
  if (!kyc) return null;
  if (step === 'submitted') return kyc.submittedAt;
  if (step === 'approved' || step === 'rejected') return kyc.decidedAt;
  return null;
}

/** The timeline borrador → enviado → aprobado/rechazado; the last step shows the decision. */
export function kycTimeline(kyc: KycCase | undefined): { status: KycStatus; state: 'done' | 'current' | 'upcoming'; at: string | null }[] {
  const status = kyc?.status ?? 'draft';
  const steps: KycStatus[] = status === 'rejected' ? ['draft', 'submitted', 'rejected'] : STEPS;
  const index = steps.indexOf(status);
  return steps.map((step, position) => ({
    status: step,
    state: position < index || status === 'approved' ? 'done' : position === index ? 'current' : 'upcoming',
    at: position <= index ? stepDate(kyc, step) : null,
  }));
}

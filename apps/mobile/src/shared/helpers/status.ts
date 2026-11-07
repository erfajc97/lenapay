import type { KycStatus, MovementStatus, PayoutStatus } from '@/shared/api/types';
import type { ChipTone } from '@/shared/ui/Chip';

/** Status → chip tone; the label always comes with it (colour is never the only signal, M7). */
export const PAYOUT_STATUS_TONE: Record<PayoutStatus, ChipTone> = {
  pending_approval: 'warning',
  approved: 'brand',
  sending: 'brand',
  settled: 'success',
  failed: 'danger',
  rejected: 'danger',
  cancelled: 'neutral',
};

export const MOVEMENT_STATUS_TONE: Record<MovementStatus, ChipTone> = {
  pending: 'warning',
  settled: 'success',
  failed: 'danger',
  cancelled: 'neutral',
};

export const KYC_STATUS_TONE: Record<KycStatus, ChipTone> = {
  draft: 'neutral',
  submitted: 'warning',
  approved: 'success',
  rejected: 'danger',
};

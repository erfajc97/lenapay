import type { Rail } from '@/shared/api/types';
import type { IconName, IconTone } from '@/shared/ui/Icon';

/** Rail → badge look; the label always travels with it (M7). On-us stands out in the brand tone. */
export const RAIL_BADGE: Record<Rail, { icon: IconName; tone: IconTone; background: string }> = {
  internal: { icon: 'flash', tone: 'primary', background: 'bg-primary-bg' },
  bank: { icon: 'business-outline', tone: 'text-secondary', background: 'bg-bg-page border border-border-subtle' },
};

/** The on-us banner shows once the API says the transfer stays inside Lenapay (quote or payout). */
export function isInstantLenapay(source: { rail: Rail } | null | undefined): boolean {
  return source?.rail === 'internal';
}

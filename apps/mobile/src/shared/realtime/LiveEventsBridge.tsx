import { useActiveOrg } from '@/shared/queries/orgs';

import { useLiveEvents } from './useLiveEvents';

/** Keeps the active org's event stream open while the signed-in app is mounted. */
export function LiveEventsBridge() {
  const { org } = useActiveOrg();
  useLiveEvents(org?.id);
  return null;
}

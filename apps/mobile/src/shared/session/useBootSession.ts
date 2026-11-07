import { useEffect } from 'react';

import { useSessionStore } from '@/shared/stores/useSessionStore';

import { readRefreshToken } from './session';

/**
 * At launch: a stored refresh token means "locked" (biometric unlock next), none means
 * "signed out". Reads the local keychain only, never the network.
 */
export function useBootSession(): void {
  const setStatus = useSessionStore((state) => state.setStatus);
  useEffect(() => {
    let cancelled = false;
    void readRefreshToken().then((token) => {
      if (!cancelled) setStatus(token ? 'locked' : 'signedOut');
    });
    return () => {
      cancelled = true;
    };
  }, [setStatus]);
}

import { router } from 'expo-router';
import { useState } from 'react';

import { useApiTarget } from '@/shared/api';
import { type ApiMode, config, isValidApiUrl } from '@/shared/config/env';

import { useApplyApiTarget, useCheckApi } from '../mutations/useDeveloperMutations';

/**
 * Settings → Desarrollador (development builds only): mock or live and the API URL, saved on
 * the device. Applying signs out, because a session belongs to the API that issued it.
 */
export function useDeveloper() {
  const current = useApiTarget();
  const [mode, setMode] = useState<ApiMode>(current.mode);
  const [url, setUrl] = useState(current.url);
  const apply = useApplyApiTarget();
  const check = useCheckApi();
  const urlValid = isValidApiUrl(url);

  return {
    enabled: config.devTools,
    current,
    buildDefault: config.env,
    mode,
    setMode,
    url,
    setUrl,
    urlError: mode === 'live' && !urlValid ? 'developer.urlInvalid' : null,
    canApply: (mode === 'mock' || urlValid) && !apply.isPending,
    apply: () => apply.mutate({ mode, url }),
    reset: () => apply.mutate(null),
    applying: apply.isPending,
    check: () => check.mutate(url),
    canCheck: urlValid && !check.isPending,
    checking: check.isPending,
    close: () => router.back(),
  };
}

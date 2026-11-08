import { router } from 'expo-router';
import { useState } from 'react';

import { useApiTarget } from '@/shared/api';
import { HEALTH_CHECK_TIMEOUT_MS } from '@/shared/api/live/health';
import { type ApiMode, config, isValidApiUrl, resolveApiUrl } from '@/shared/config/env';
import { useCopy } from '@/shared/mutations/useDeviceActions';

import { type ConnectionCheckView, connectionChip, describeConnectionCheck } from '../helpers/connectionCheck';
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
  const copy = useCopy();
  const urlValid = isValidApiUrl(url);
  const testedUrl = check.variables;
  const chip = connectionChip({ pending: check.isPending, testedUrl, currentUrl: url, succeeded: check.isSuccess, failed: check.isError });
  // The card shows the last result only while it still describes the URL in the field.
  let result: ConnectionCheckView | null = null;
  if (testedUrl !== undefined && (chip === 'connected' || chip === 'offline')) {
    result = describeConnectionCheck({ url: testedUrl, outcome: check.data ? { kind: 'success', ...check.data } : { kind: 'failure', error: check.error } });
  }

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
    check: () => {
      copy.reset();
      check.mutate(resolveApiUrl(url, undefined));
    },
    canCheck: urlValid && !check.isPending,
    checking: check.isPending,
    checkTimeoutSeconds: String(HEALTH_CHECK_TIMEOUT_MS / 1_000),
    chip,
    result,
    copyFirewallCommand: () => {
      if (result?.firewallCommand) copy.mutate(result.firewallCommand);
    },
    firewallCommandCopied: copy.isSuccess && copy.variables === result?.firewallCommand,
    close: () => router.back(),
  };
}

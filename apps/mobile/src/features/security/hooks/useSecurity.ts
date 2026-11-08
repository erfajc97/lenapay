import { useQuery } from '@tanstack/react-query';

import { biometricsAvailable } from '@/shared/biometrics/biometrics';
import { useMe } from '@/shared/queries/session';
import { readBiometricsEnabled } from '@/shared/session/session';
import { BIOMETRICS_KEY } from '@/shared/stepup/useStepUpSheet';

import { useEnablePush, useToggleBiometrics } from '../mutations/useSettingsMutations';
import { BIOMETRICS_SETTING_KEY } from '../types';


export function useSecurity() {
  const me = useMe();
  const available = useQuery({ queryKey: BIOMETRICS_KEY, queryFn: biometricsAvailable, staleTime: Infinity });
  const enabled = useQuery({ queryKey: BIOMETRICS_SETTING_KEY, queryFn: readBiometricsEnabled });
  const toggle = useToggleBiometrics();
  const push = useEnablePush();
  return {
    me,
    biometricsAvailable: available.data === true,
    biometricsEnabled: enabled.data === true,
    setBiometrics: (value: boolean) => toggle.mutate(value),
    toggling: toggle.isPending,
    enablePush: () => push.mutate(),
    enablingPush: push.isPending,
  };
}

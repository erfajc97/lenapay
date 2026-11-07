import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { ApiError, errorMessageKey } from '@/shared/api/errors';
import { authenticateWithBiometrics, biometricsAvailable } from '@/shared/biometrics/biometrics';
import { useStepUpStore } from '@/shared/stores/useStepUpStore';

import { useVerifyStepUp } from './mutations/useVerifyStepUp';
import { stepUpMode } from './stepUpMode';

export const BIOMETRICS_KEY = ['device', 'biometrics'] as const;

export function useStepUpSheet() {
  const { t } = useTranslation();
  const open = useStepUpStore((state) => state.open);
  const cancel = useStepUpStore((state) => state.cancel);
  const verify = useVerifyStepUp();
  const [password, setPassword] = useState('');
  const [choosePassword, setChoosePassword] = useState(false);
  const [biometricFailed, setBiometricFailed] = useState(false);
  const biometrics = useQuery({ queryKey: BIOMETRICS_KEY, queryFn: biometricsAvailable, staleTime: Infinity });

  const lastErrorCode = verify.error instanceof ApiError ? verify.error.code : null;
  const mode = stepUpMode({ biometrics: biometrics.data === true, choosePassword, lastErrorCode });
  const errorText = biometricFailed ? t('stepUp.biometricFailed') : verify.error ? t(errorMessageKey(verify.error)) : null;

  function reset() {
    setPassword('');
    setChoosePassword(false);
    setBiometricFailed(false);
    verify.reset();
  }

  async function confirmWithBiometrics() {
    setBiometricFailed(false);
    const ok = await authenticateWithBiometrics({ prompt: t('stepUp.prompt'), cancelLabel: t('common.cancel') });
    if (!ok) {
      setBiometricFailed(true);
      return;
    }
    verify.mutate({ method: 'device_key' }, { onSuccess: reset });
  }

  function confirmWithPassword() {
    verify.mutate({ method: 'password', password }, { onSuccess: reset });
  }

  return {
    open,
    mode,
    password,
    setPassword,
    errorText,
    pending: verify.isPending,
    canSubmitPassword: password.length > 0 && !verify.isPending,
    usePassword: () => setChoosePassword(true),
    confirmWithBiometrics,
    confirmWithPassword,
    close: () => {
      reset();
      cancel();
    },
  };
}

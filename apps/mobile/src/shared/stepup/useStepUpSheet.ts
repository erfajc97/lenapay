import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Keyboard } from 'react-native';

import { ApiError, errorMessageKey } from '@/shared/api/errors';
import { authenticateWithBiometrics, biometricsAvailable } from '@/shared/biometrics/biometrics';
import { waitForKeyboardHidden } from '@/shared/helpers/keyboard';
import { devLog } from '@/shared/reporting/reporting';
import { useStepUpStore } from '@/shared/stores/useStepUpStore';

import { useVerifyStepUp } from './mutations/useVerifyStepUp';
import { stepUpMode } from './stepUpMode';

export const BIOMETRICS_KEY = ['device', 'biometrics'] as const;

export function useStepUpSheet() {
  const { t } = useTranslation();
  const open = useStepUpStore((state) => state.open);
  const cancel = useStepUpStore((state) => state.cancel);
  const dismissed = useStepUpStore((state) => state.dismissed);
  const presented = useStepUpStore((state) => state.presented);
  const request = useStepUpStore((state) => state.pending);
  // The request for which the keyboard is already gone (a new request waits again).
  const [keyboardClearFor, setKeyboardClearFor] = useState<object | null>(null);
  const verify = useVerifyStepUp();
  const [password, setPassword] = useState('');
  const [choosePassword, setChoosePassword] = useState(false);
  const [biometricFailed, setBiometricFailed] = useState(false);
  const biometrics = useQuery({ queryKey: BIOMETRICS_KEY, queryFn: biometricsAvailable, staleTime: Infinity });

  // The sheet presents only once the keyboard of the screen below (the amount keypad) is gone.
  useEffect(() => {
    if (!open || !request) return undefined;
    let live = true;
    waitForKeyboardHidden(Keyboard).then(() => {
      if (!live) return;
      devLog('stepup.keyboard_clear');
      setKeyboardClearFor(request);
    });
    return () => {
      live = false;
    };
  }, [open, request]);

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
    visible: open && request !== null && keyboardClearFor === request,
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
    dismissed,
    presented,
  };
}

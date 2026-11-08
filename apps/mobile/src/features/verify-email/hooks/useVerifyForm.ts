import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';

import { useSessionStore } from '@/shared/stores/useSessionStore';

import { useResendCode, useVerifyEmail } from '../mutations/useVerifyEmail';
import { verifySchema, type VerifyValues } from '../types';

export function useVerifyForm() {
  const verify = useVerifyEmail();
  const resend = useResendCode();
  const email = useSessionStore((state) => state.pendingEmail);
  const form = useForm<VerifyValues>({ resolver: zodResolver(verifySchema), defaultValues: { code: '' } });
  const submit = () => void form.handleSubmit((values) => verify.mutate({ email: email ?? '', code: values.code }))();
  return {
    form,
    submit,
    pending: verify.isPending,
    email,
    // One code per tap: a second tap while sending would burn the hourly quota (5 codes).
    resend: () => {
      if (!resend.isPending) resend.mutate({ email: email ?? '' });
    },
  };
}

import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';

import { useSessionStore } from '@/shared/stores/useSessionStore';

import { signupRequest } from '../helpers/signupRequest';
import { useLogin, useResendCode, useSignup, useVerifyEmail } from '../mutations/useAuthMutations';
import { loginSchema, type LoginValues, signupSchema, type SignupValues, verifySchema, type VerifyValues } from '../types';

/** Login (also "Entrar con Lebane": same identity provider and endpoint, R-ID-4). */
export function useLoginForm() {
  const login = useLogin();
  // Right after verifying the email the address is already known.
  const verifiedEmail = useSessionStore((state) => state.pendingEmail);
  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { email: verifiedEmail ?? '', password: '' } });
  const submit = () => void form.handleSubmit((values) => login.mutate(values))();
  return { form, submit, pending: login.isPending };
}

export function useSignupForm() {
  const signup = useSignup();
  const form = useForm<SignupValues>({ resolver: zodResolver(signupSchema), defaultValues: { fullName: '', email: '', password: '' } });
  const submit = () => void form.handleSubmit((values) => signup.mutate(signupRequest(values)))();
  return { form, submit, pending: signup.isPending };
}

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

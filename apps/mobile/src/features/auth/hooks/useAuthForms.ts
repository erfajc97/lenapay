import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';

import { useSessionStore } from '@/shared/stores/useSessionStore';

import { useLogin, useSignup, useVerifyEmail } from '../mutations/useAuthMutations';
import { loginSchema, type LoginValues, signupSchema, type SignupValues, verifySchema, type VerifyValues } from '../types';

export function useLoginForm(provider?: 'lebane') {
  const login = useLogin();
  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { email: '', password: '' } });
  const submit = () => void form.handleSubmit((values) => login.mutate({ ...values, provider }))();
  return { form, submit, pending: login.isPending };
}

export function useSignupForm() {
  const signup = useSignup();
  const form = useForm<SignupValues>({ resolver: zodResolver(signupSchema), defaultValues: { fullName: '', email: '', password: '' } });
  const submit = () => void form.handleSubmit((values) => signup.mutate(values))();
  return { form, submit, pending: signup.isPending };
}

export function useVerifyForm() {
  const verify = useVerifyEmail();
  const email = useSessionStore((state) => state.pendingEmail);
  const form = useForm<VerifyValues>({ resolver: zodResolver(verifySchema), defaultValues: { code: '' } });
  const submit = () => void form.handleSubmit((values) => verify.mutate({ email: email ?? '', code: values.code }))();
  return { form, submit, pending: verify.isPending, email };
}

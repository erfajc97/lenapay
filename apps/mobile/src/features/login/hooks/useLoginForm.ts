import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';

import { useSessionStore } from '@/shared/stores/useSessionStore';

import { useLogin } from '../mutations/useLogin';
import { loginSchema, type LoginValues } from '../types';

/** Login (also "Entrar con Lebane": same identity provider and endpoint, R-ID-4). */
export function useLoginForm() {
  const login = useLogin();
  // Right after verifying the email the address is already known.
  const verifiedEmail = useSessionStore((state) => state.pendingEmail);
  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { email: verifiedEmail ?? '', password: '' } });
  const submit = () => void form.handleSubmit((values) => login.mutate(values))();
  return { form, submit, pending: login.isPending };
}

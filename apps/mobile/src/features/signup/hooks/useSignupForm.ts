import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';

import { signupRequest } from '../helpers/signupRequest';
import { useSignup } from '../mutations/useSignup';
import { signupSchema, type SignupValues } from '../types';

export function useSignupForm() {
  const signup = useSignup();
  const form = useForm<SignupValues>({ resolver: zodResolver(signupSchema), defaultValues: { fullName: '', email: '', password: '' } });
  const submit = () => void form.handleSubmit((values) => signup.mutate(signupRequest(values)))();
  return { form, submit, pending: signup.isPending };
}

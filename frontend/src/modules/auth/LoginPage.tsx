import { useRef, useState } from 'react';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { LogIn } from 'lucide-react';
import { PasswordInput } from '../../components/PasswordInput';
import { useToast } from '../../components/ToastProvider';
import { useAuth } from '../../lib/auth/AuthContext';
import { firstInvalidFieldKey, useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { AuthCard } from './AuthCard';

const loginSchema = z.object({
  email: z.string().email('Email đăng nhập không hợp lệ'),
  password: z.string().min(1, 'Nhập mật khẩu'),
});

type LoginForm = z.infer<typeof loginSchema>;

export function LoginPage() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const [error, setError] = useState('');
  const formRef = useRef<HTMLFormElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);
  const form = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
    defaultValues: import.meta.env.DEV
      ? { email: 'admin@consflow.local', password: '123456' }
      : { email: '', password: '' },
  });

  if (user) return <Navigate to="/" replace />;

  return (
    <AuthCard title="Đăng nhập" description="Dùng tài khoản của anh/chị để vào Thủ Thiêm Workflow.">
        <form
          ref={formRef}
          className="space-y-4"
          noValidate
          onSubmit={form.handleSubmit(async (values) => {
            setError('');
            try {
              await login(values.email, values.password);
              navigate('/');
            } catch (err) {
              setError(err instanceof Error ? err.message : 'Đăng nhập thất bại');
            }
          }, (errors) => {
            toast.warn({ title: 'Vui lòng kiểm tra thông tin', description: 'Email hoặc mật khẩu chưa hợp lệ.' });
            navigateToValidation(firstInvalidFieldKey(errors, ['email', 'password']));
          })}
        >
          <label data-validation-key="email" className="block scroll-mt-24 rounded-md">
            <span className="text-sm font-semibold text-slate-700">Email</span>
            <input type="email" autoComplete="email" aria-invalid={Boolean(form.formState.errors.email)} aria-describedby={form.formState.errors.email ? 'login-email-error' : undefined} className={`mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring ${form.formState.errors.email ? 'border-rose-400 bg-rose-50' : 'border-line'}`} {...form.register('email')} />
            {form.formState.errors.email?.message ? <span id="login-email-error" role="alert" className="mt-1 block text-xs font-medium text-rose-600">{form.formState.errors.email.message}</span> : null}
          </label>
          <label data-validation-key="password" className="block scroll-mt-24 rounded-md">
            <span className="flex items-center justify-between gap-3 text-sm font-semibold text-slate-700">
              Mật khẩu
              <Link className="text-xs text-blue-700 hover:text-blue-800" to="/forgot-password">Quên mật khẩu?</Link>
            </span>
            <span className="mt-1 block">
              <PasswordInput
                autoComplete="current-password"
                className={`w-full rounded-md border px-3 py-2 text-sm focus-ring ${form.formState.errors.password ? 'border-rose-400 bg-rose-50' : 'border-line'}`}
                invalid={Boolean(form.formState.errors.password)}
                aria-describedby={form.formState.errors.password ? 'login-password-error' : undefined}
                {...form.register('password')}
              />
            </span>
            {form.formState.errors.password?.message ? <span id="login-password-error" role="alert" className="mt-1 block text-xs font-medium text-rose-600">{form.formState.errors.password.message}</span> : null}
          </label>
          {error ? <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p> : null}
          <button className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700">
            <LogIn className="h-4 w-4" />
            Đăng nhập
          </button>
        </form>
    </AuthCard>
  );
}

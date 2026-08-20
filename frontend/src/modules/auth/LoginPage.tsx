import { useState } from 'react';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { LogIn } from 'lucide-react';
import { useAuth } from '../../lib/auth/AuthContext';
import { AuthCard } from './AuthCard';

const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

type LoginForm = z.infer<typeof loginSchema>;

export function LoginPage() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState('');
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
          className="space-y-4"
          onSubmit={form.handleSubmit(async (values) => {
            setError('');
            try {
              await login(values.email, values.password);
              navigate('/');
            } catch (err) {
              setError(err instanceof Error ? err.message : 'Đăng nhập thất bại');
            }
          })}
        >
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Email</span>
            <input className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" {...form.register('email')} />
          </label>
          <label className="block">
            <span className="flex items-center justify-between gap-3 text-sm font-semibold text-slate-700">
              Mật khẩu
              <Link className="text-xs text-blue-700 hover:text-blue-800" to="/forgot-password">Quên mật khẩu?</Link>
            </span>
            <input type="password" className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" {...form.register('password')} />
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

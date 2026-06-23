import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { LogIn } from 'lucide-react';
import { useAuth } from '../../lib/auth/AuthContext';

const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

type LoginForm = z.infer<typeof loginSchema>;

const demoAccounts = [
  'admin@consflow.local',
  'pm@consflow.local',
  'qs@consflow.local',
  'accounting@consflow.local',
  'warehouse@consflow.local',
  'supervisor@consflow.local',
  'director@consflow.local',
];

export function LoginPage() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState('');
  const form = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: demoAccounts[0], password: '123456' },
  });

  if (user) return <Navigate to="/" replace />;

  return (
    <div className="grid min-h-screen place-items-center bg-surface px-4">
      <div className="w-full max-w-md rounded-lg border border-line bg-white p-6 shadow-panel">
        <div className="mb-6">
          <div className="mb-4 grid h-11 w-11 place-items-center rounded-lg bg-blue-600 text-sm font-black text-white">CF</div>
          <h1 className="text-2xl font-semibold text-ink">Đăng nhập Consflow</h1>
          <p className="mt-1 text-sm text-slate-600">Dùng tài khoản demo để vào hệ thống workflow công trình.</p>
        </div>

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
            <span className="text-sm font-semibold text-slate-700">Tài khoản demo</span>
            <select
              className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring"
              onChange={(event) => {
                form.setValue('email', event.target.value);
                form.setValue('password', '123456');
              }}
              value={form.watch('email')}
            >
              {demoAccounts.map((email) => (
                <option key={email}>{email}</option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Email</span>
            <input className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" {...form.register('email')} />
          </label>
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Mật khẩu</span>
            <input type="password" className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" {...form.register('password')} />
          </label>
          {error ? <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p> : null}
          <button className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700">
            <LogIn className="h-4 w-4" />
            Đăng nhập
          </button>
        </form>
      </div>
    </div>
  );
}

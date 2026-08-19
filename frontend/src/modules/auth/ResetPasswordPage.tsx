import { useEffect, useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Link } from 'react-router-dom';
import { KeyRound } from 'lucide-react';
import { z } from 'zod';
import { useToast } from '../../components/ToastProvider';
import { post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { AuthCard } from './AuthCard';
import { clearRecoveryTokenFragment, recoveryTokenFromFragment } from './recoveryToken';

const schema = z.object({
  new_password: z.string().min(8, 'Mật khẩu mới phải có ít nhất 8 ký tự').refine((value) => new TextEncoder().encode(value).length <= 72, 'Mật khẩu mới không được vượt quá 72 byte'),
  new_password_confirmation: z.string().min(1, 'Nhập lại mật khẩu mới'),
}).superRefine((values, context) => {
  if (values.new_password !== values.new_password_confirmation) context.addIssue({ code: 'custom', path: ['new_password_confirmation'], message: 'Mật khẩu xác nhận không khớp' });
});
type Values = z.infer<typeof schema>;

export function ResetPasswordPage() {
  const token = useMemo(recoveryTokenFromFragment, []);
  const [completed, setCompleted] = useState(false);
  const toast = useToast();
  const { logout } = useAuth();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { new_password: '', new_password_confirmation: '' } });
  const resetPassword = useMutation({
    mutationFn: (values: Values) => post<{ password_reset: boolean }>('/auth/password/reset', { token, ...values }),
    onSuccess: async () => {
      clearRecoveryTokenFragment();
      await logout();
      setCompleted(true);
      toast.success('Đã đặt lại mật khẩu');
    },
    onError: (error) => toast.error('Không đặt lại được mật khẩu', error),
  });

  useEffect(() => {
    if (token) clearRecoveryTokenFragment();
  }, [token]);

  if (!token) return <InvalidLink />;

  return (
    <AuthCard title="Đặt lại mật khẩu" description="Tạo mật khẩu mới cho tài khoản Consflow. Liên kết chỉ sử dụng được một lần.">
      {completed ? (
        <div className="space-y-4">
          <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm leading-6 text-emerald-800" role="status">Mật khẩu đã được cập nhật. Tất cả phiên đăng nhập cũ đã bị thu hồi.</div>
          <Link className="block w-full rounded-md bg-blue-600 px-4 py-2.5 text-center text-sm font-semibold text-white hover:bg-blue-700" to="/login">Đăng nhập</Link>
        </div>
      ) : (
        <form className="space-y-4" onSubmit={form.handleSubmit((values) => resetPassword.mutate(values))} noValidate>
          <PasswordInput label="Mật khẩu mới" error={form.formState.errors.new_password?.message} registration={form.register('new_password')} />
          <PasswordInput label="Nhập lại mật khẩu mới" error={form.formState.errors.new_password_confirmation?.message} registration={form.register('new_password_confirmation')} />
          <p className="rounded-md bg-slate-50 px-3 py-2 text-sm leading-5 text-slate-600">Mật khẩu cần ít nhất 8 ký tự và không vượt quá 72 byte.</p>
          <button disabled={resetPassword.isPending} className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
            <KeyRound className="h-4 w-4" />
            {resetPassword.isPending ? 'Đang cập nhật...' : 'Đặt lại mật khẩu'}
          </button>
        </form>
      )}
      {!completed ? <Link className="mt-4 block text-center text-sm font-semibold text-blue-700 hover:text-blue-800" to="/login">Quay lại đăng nhập</Link> : null}
    </AuthCard>
  );
}

function InvalidLink() {
  return (
    <AuthCard title="Liên kết không hợp lệ" description="Liên kết đặt lại mật khẩu bị thiếu hoặc đã được xóa khỏi trình duyệt.">
      <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-800">Hãy yêu cầu một liên kết khôi phục mới.</div>
      <Link className="mt-4 block w-full rounded-md bg-blue-600 px-4 py-2.5 text-center text-sm font-semibold text-white hover:bg-blue-700" to="/forgot-password">Yêu cầu liên kết mới</Link>
    </AuthCard>
  );
}

function PasswordInput({ label, error, registration }: { label: string; error?: string; registration: ReturnType<ReturnType<typeof useForm<Values>>['register']> }) {
  return (
    <label className="block text-sm font-semibold text-slate-700">
      {label} <span className="text-rose-600">*</span>
      <input type="password" autoComplete="new-password" className={`mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring ${error ? 'border-rose-400 bg-rose-50' : 'border-line'}`} {...registration} />
      {error ? <span className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
    </label>
  );
}

import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Link } from 'react-router-dom';
import { KeyRound } from 'lucide-react';
import { z } from 'zod';
import { PasswordInput } from '../../components/PasswordInput';
import { useToast } from '../../components/ToastProvider';
import { post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { PASSWORD_RULE_HELPER, passwordConfirmationMessage, passwordValueSchema } from '../../lib/security/password';
import { AuthCard } from './AuthCard';
import { clearRecoveryTokenFragment, recoveryTokenFromFragment } from './recoveryToken';
import { firstInvalidFieldKey, useValidationNavigation } from '../../lib/utils/formValidationNavigation';

const schema = z.object({
  new_password: passwordValueSchema(),
  new_password_confirmation: z.string().min(1, 'Nhập lại mật khẩu mới'),
}).superRefine((values, context) => {
  const confirmationError = passwordConfirmationMessage(values.new_password_confirmation, values.new_password);
  if (confirmationError) context.addIssue({ code: 'custom', path: ['new_password_confirmation'], message: confirmationError });
});
type Values = z.infer<typeof schema>;

export function ResetPasswordPage() {
  const token = useMemo(recoveryTokenFromFragment, []);
  const [completed, setCompleted] = useState(false);
  const toast = useToast();
  const { logout } = useAuth();
  const formRef = useRef<HTMLFormElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);
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
        <form ref={formRef} className="space-y-4" onSubmit={form.handleSubmit((values) => resetPassword.mutate(values), (errors) => {
          toast.warn({ title: 'Vui lòng kiểm tra thông tin', description: 'Mật khẩu mới hoặc phần xác nhận chưa hợp lệ.' });
          navigateToValidation(firstInvalidFieldKey(errors, ['new_password', 'new_password_confirmation']));
        })} noValidate>
          <PasswordField label="Mật khẩu mới" error={form.formState.errors.new_password?.message} registration={form.register('new_password')} />
          <PasswordField label="Nhập lại mật khẩu mới" error={form.formState.errors.new_password_confirmation?.message} registration={form.register('new_password_confirmation')} />
          <p className="rounded-md bg-slate-50 px-3 py-2 text-sm leading-5 text-slate-600">{PASSWORD_RULE_HELPER}</p>
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
      <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-800">Hãy gửi yêu cầu khôi phục mới để bộ phận hỗ trợ xác minh.</div>
      <Link className="mt-4 block w-full rounded-md bg-blue-600 px-4 py-2.5 text-center text-sm font-semibold text-white hover:bg-blue-700" to="/forgot-password">Gửi yêu cầu khôi phục</Link>
    </AuthCard>
  );
}

function PasswordField({ label, error, registration }: { label: string; error?: string; registration: ReturnType<ReturnType<typeof useForm<Values>>['register']> }) {
  const errorId = `${registration.name}-error`;
  return (
    <label data-validation-key={registration.name} className="block scroll-mt-24 rounded-md text-sm font-semibold text-slate-700">
      {label} <span className="text-rose-600">*</span>
      <span className="mt-1 block">
        <PasswordInput
          autoComplete="new-password"
          className={`w-full rounded-md border px-3 py-2 text-sm focus-ring ${error ? 'border-rose-400 bg-rose-50' : 'border-line'}`}
          invalid={Boolean(error)}
          aria-describedby={error ? errorId : undefined}
          {...registration}
        />
      </span>
      {error ? <span id={errorId} role="alert" className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
    </label>
  );
}

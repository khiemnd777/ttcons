import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { KeyRound } from 'lucide-react';
import { z } from 'zod';
import { PasswordInput } from '../../components/PasswordInput';
import { useToast } from '../../components/ToastProvider';
import { put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { PASSWORD_RULE_HELPER, passwordConfirmationMessage, passwordValueSchema } from '../../lib/security/password';
import { AuthCard } from './AuthCard';

const schema = z.object({
  current_password: z.string().min(1, 'Nhập mật khẩu ban đầu'),
  new_password: passwordValueSchema(),
  new_password_confirmation: z.string().min(1, 'Nhập lại mật khẩu mới'),
}).superRefine((values, context) => {
  const confirmationError = passwordConfirmationMessage(values.new_password_confirmation, values.new_password);
  if (confirmationError) context.addIssue({ code: 'custom', path: ['new_password_confirmation'], message: confirmationError });
  if (values.current_password && values.current_password === values.new_password) {
    context.addIssue({ code: 'custom', path: ['new_password'], message: 'Mật khẩu mới phải khác mật khẩu ban đầu' });
  }
});

type FormValues = z.infer<typeof schema>;

export function InitialPasswordChangePage() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const [serverError, setServerError] = useState('');
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: '', new_password: '', new_password_confirmation: '' },
  });

  return (
    <AuthCard title="Đổi mật khẩu ban đầu" description="Tài khoản đã được kích hoạt. Anh/chị cần đặt mật khẩu riêng trước khi sử dụng hệ thống.">
      <form
        className="space-y-4"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          setServerError('');
          try {
            await put('/auth/password', values);
            toast.success({ title: 'Đã đổi mật khẩu', description: 'Vui lòng đăng nhập lại bằng mật khẩu mới.' });
            await logout();
            navigate('/login', { replace: true });
          } catch (error) {
            setServerError(error instanceof Error ? error.message : 'Không đổi được mật khẩu');
          }
        })}
      >
        <PasswordField label="Mật khẩu ban đầu" name="current_password" autoComplete="current-password" form={form} />
        <PasswordField label="Mật khẩu mới" name="new_password" autoComplete="new-password" helper={PASSWORD_RULE_HELPER} form={form} />
        <PasswordField label="Nhập lại mật khẩu mới" name="new_password_confirmation" autoComplete="new-password" form={form} />
        {serverError ? <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{serverError}</p> : null}
        <button type="submit" className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={form.formState.isSubmitting}>
          <KeyRound className="h-4 w-4" />
          {form.formState.isSubmitting ? 'Đang đổi mật khẩu…' : 'Đổi mật khẩu và đăng nhập lại'}
        </button>
      </form>
    </AuthCard>
  );
}

function PasswordField({ label, name, autoComplete, helper, form }: {
  label: string;
  name: keyof FormValues;
  autoComplete: string;
  helper?: string;
  form: ReturnType<typeof useForm<FormValues>>;
}) {
  const error = form.formState.errors[name]?.message;
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <span className="mt-1 block">
        <PasswordInput
          autoComplete={autoComplete}
          className={`w-full rounded-md border px-3 py-2 text-sm focus-ring ${error ? 'border-rose-400 bg-rose-50' : 'border-line'}`}
          invalid={Boolean(error)}
          {...form.register(name)}
        />
      </span>
      {error ? <span className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
      {!error && helper ? <span className="mt-1 block text-xs leading-5 text-slate-500">{helper}</span> : null}
    </label>
  );
}

import { useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Link } from 'react-router-dom';
import { KeyRound } from 'lucide-react';
import { z } from 'zod';
import { post } from '../../lib/api/client';
import { useToast } from '../../components/ToastProvider';
import { firstInvalidFieldKey, useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { AuthCard } from './AuthCard';

const schema = z.object({ email: z.string().trim().email('Email đăng nhập không hợp lệ') });
type Values = z.infer<typeof schema>;

export function ForgotPasswordPage() {
  const [submitted, setSubmitted] = useState(false);
  const toast = useToast();
  const formRef = useRef<HTMLFormElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { email: '' } });
  const requestReset = useMutation({
    mutationFn: (values: Values) => post<{ accepted: boolean; message: string }>('/auth/password/forgot', { email: values.email.trim().toLowerCase() }),
    onSuccess: () => setSubmitted(true),
  });

  return (
    <AuthCard title="Quên mật khẩu" description="Bộ phận hỗ trợ xác minh danh tính và cấp liên kết đặt lại mật khẩu.">
      {submitted ? (
        <div className="space-y-4">
          <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm leading-6 text-emerald-800" role="status">
            Nếu tài khoản hợp lệ, bộ phận hỗ trợ sẽ xem xét yêu cầu khôi phục.
          </div>
          <button type="button" className="w-full rounded-md border border-line px-4 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => { setSubmitted(false); requestReset.reset(); }}>
            Gửi lại yêu cầu
          </button>
        </div>
      ) : (
        <form ref={formRef} className="space-y-4" onSubmit={form.handleSubmit((values) => requestReset.mutate(values), (errors) => {
          toast.warn({ title: 'Vui lòng kiểm tra thông tin', description: 'Email đăng nhập chưa hợp lệ.' });
          navigateToValidation(firstInvalidFieldKey(errors, ['email']));
        })} noValidate>
          <label data-validation-key="email" className="block scroll-mt-24 rounded-md text-sm font-semibold text-slate-700">
            Email đăng nhập <span className="text-rose-600">*</span>
            <input type="email" autoComplete="username" aria-invalid={Boolean(form.formState.errors.email)} aria-describedby={form.formState.errors.email ? 'forgot-email-error' : undefined} className={`mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring ${form.formState.errors.email ? 'border-rose-400 bg-rose-50' : 'border-line'}`} {...form.register('email')} />
            {form.formState.errors.email ? <span id="forgot-email-error" role="alert" className="mt-1 block text-xs font-medium text-rose-600">{form.formState.errors.email.message}</span> : null}
          </label>
          <button disabled={requestReset.isPending} className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
            <KeyRound className="h-4 w-4" />
            {requestReset.isPending ? 'Đang gửi...' : 'Gửi yêu cầu khôi phục'}
          </button>
          {requestReset.isError ? <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">Không kết nối được dịch vụ khôi phục. Vui lòng thử lại.</p> : null}
        </form>
      )}
      <Link className="mt-4 block text-center text-sm font-semibold text-blue-700 hover:text-blue-800" to="/login">Quay lại đăng nhập</Link>
    </AuthCard>
  );
}

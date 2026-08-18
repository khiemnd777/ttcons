import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, type FieldErrors, type UseFormRegisterReturn } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { BriefcaseBusiness, Building2, Eye, EyeOff, KeyRound, Mail, Pencil, Phone, ShieldCheck, UserRound } from 'lucide-react';
import { z } from 'zod';
import { Dialog } from '../../components/Dialog';
import { PageHeader } from '../../components/PageHeader';
import { useToast } from '../../components/ToastProvider';
import { put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { User } from '../../lib/types/domain';

type ProfileFormValues = {
  full_name: string;
  email: string;
  phone: string;
  current_password: string;
};

type PasswordFormValues = {
  current_password: string;
  new_password: string;
  new_password_confirmation: string;
};

type ProfileUpdateResponse = {
  user: User;
  reauth_required: boolean;
};

const passwordSchema = z.object({
  current_password: z.string().min(1, 'Nhập mật khẩu hiện tại'),
  new_password: z.string()
    .min(8, 'Mật khẩu mới phải có ít nhất 8 ký tự')
    .refine((value) => new TextEncoder().encode(value).length <= 72, 'Mật khẩu mới không được vượt quá 72 byte'),
  new_password_confirmation: z.string().min(1, 'Nhập lại mật khẩu mới'),
}).superRefine((values, context) => {
  if (values.new_password !== values.new_password_confirmation) {
    context.addIssue({ code: 'custom', path: ['new_password_confirmation'], message: 'Mật khẩu xác nhận không khớp' });
  }
  if (values.current_password && values.current_password === values.new_password) {
    context.addIssue({ code: 'custom', path: ['new_password'], message: 'Mật khẩu mới phải khác mật khẩu hiện tại' });
  }
});

function profileSchema(currentEmail: string) {
  return z.object({
    full_name: z.string().trim().min(2, 'Họ tên phải có ít nhất 2 ký tự').max(120, 'Họ tên không được vượt quá 120 ký tự'),
    email: z.string().trim().email('Email đăng nhập không hợp lệ'),
    phone: z.string().trim().refine(validProfilePhone, 'Số điện thoại phải có từ 8 đến 15 chữ số'),
    current_password: z.string(),
  }).superRefine((values, context) => {
    if (values.email.trim().toLowerCase() !== currentEmail.toLowerCase() && !values.current_password) {
      context.addIssue({ code: 'custom', path: ['current_password'], message: 'Nhập mật khẩu hiện tại để đổi email' });
    }
  });
}

function validProfilePhone(value: string) {
  if (!value.trim()) return true;
  const normalized = value.trim().replace(/[\s().-]/g, '');
  return /^\+?\d{8,15}$/.test(normalized);
}

export function ProfilePage() {
  const { user, refreshUser, logout } = useAuth();
  const navigate = useNavigate();
  const toast = useToast();
  const [profileOpen, setProfileOpen] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [visiblePasswords, setVisiblePasswords] = useState<Record<string, boolean>>({});
  const currentEmail = user?.email ?? '';
  const currentProfileSchema = useMemo(() => profileSchema(currentEmail), [currentEmail]);

  const profileForm = useForm<ProfileFormValues>({
    resolver: zodResolver(currentProfileSchema),
    defaultValues: { full_name: '', email: '', phone: '', current_password: '' },
  });
  const passwordForm = useForm<PasswordFormValues>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { current_password: '', new_password: '', new_password_confirmation: '' },
  });

  const updateProfile = useMutation({
    mutationFn: (values: ProfileFormValues) => put<ProfileUpdateResponse>('/auth/profile', {
      ...values,
      full_name: values.full_name.trim(),
      email: values.email.trim().toLowerCase(),
      phone: values.phone.trim(),
    }),
    onSuccess: async (result) => {
      setProfileOpen(false);
      if (result.reauth_required) {
        toast.success({ title: 'Đã cập nhật email', description: 'Vui lòng đăng nhập lại bằng email mới.' });
        await logout();
        navigate('/login', { replace: true });
        return;
      }
      try {
        await refreshUser();
        toast.success('Đã cập nhật hồ sơ cá nhân');
      } catch (error) {
        toast.error('Đã lưu nhưng chưa tải lại được hồ sơ', error);
      }
    },
    onError: (error) => toast.error('Không cập nhật được hồ sơ cá nhân', error),
  });

  const changePassword = useMutation({
    mutationFn: (values: PasswordFormValues) => put<{ password_changed: boolean; reauth_required: boolean }>('/auth/password', values),
    onSuccess: async () => {
      setPasswordOpen(false);
      toast.success({ title: 'Đã đổi mật khẩu', description: 'Vui lòng đăng nhập lại bằng mật khẩu mới.' });
      await logout();
      navigate('/login', { replace: true });
    },
    onError: (error) => toast.error('Không đổi được mật khẩu', error),
  });

  if (!user) {
    return <div className="rounded-lg border border-line bg-white p-6 text-sm text-slate-600">Không tải được hồ sơ người dùng.</div>;
  }

  const initials = user.full_name
    .split(/\s+/)
    .filter(Boolean)
    .slice(-2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || 'U';
  const roles = Array.from(new Set(user.roles ?? []));
  const emailChanged = profileForm.watch('email').trim().toLowerCase() !== user.email.toLowerCase();

  const openProfile = () => {
    profileForm.reset({
      full_name: user.full_name,
      email: user.email,
      phone: user.phone ?? '',
      current_password: '',
    });
    setProfileOpen(true);
  };
  const openPassword = () => {
    passwordForm.reset({ current_password: '', new_password: '', new_password_confirmation: '' });
    setVisiblePasswords({});
    setPasswordOpen(true);
  };

  return (
    <>
      <PageHeader title="Hồ sơ cá nhân" description="Quản lý thông tin tài khoản và bảo mật của chính bạn." />

      <section className="mb-4 flex flex-col gap-4 rounded-lg border border-line bg-white p-5 sm:flex-row sm:items-center">
        <div className="grid h-14 w-14 shrink-0 place-items-center rounded-full bg-blue-100 text-lg font-bold text-blue-700" aria-hidden="true">
          {initials}
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-xl font-semibold text-ink">{user.full_name}</h2>
          <p className="mt-1 truncate text-sm text-slate-500">{user.email}{roles.length ? ` · ${roles.join(', ')}` : ''}</p>
        </div>
        <span className="inline-flex w-fit rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700">Đang hoạt động</span>
      </section>

      <div className="grid gap-4 xl:grid-cols-2">
        <section className="rounded-lg border border-line bg-white p-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <UserRound className="h-5 w-5 text-blue-600" />
                <h2 className="text-base font-semibold text-ink">Thông tin cá nhân</h2>
              </div>
              <p className="mt-1 text-sm text-slate-500">Thông tin liên hệ và hồ sơ công việc hiện tại.</p>
            </div>
            <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={openProfile}>
              <Pencil className="h-4 w-4" />
              Chỉnh sửa thông tin
            </button>
          </div>
          <dl className="mt-5 grid gap-4 sm:grid-cols-2">
            <ProfileValue icon={UserRound} label="Họ và tên" value={user.full_name} />
            <ProfileValue icon={Mail} label="Email đăng nhập" value={user.email} />
            <ProfileValue icon={Phone} label="Số điện thoại" value={user.phone} />
            <ProfileValue icon={Building2} label="Phòng ban" value={user.department} />
            <ProfileValue icon={BriefcaseBusiness} label="Chức vụ" value={user.position} />
            <ProfileValue icon={ShieldCheck} label="Vai trò" value={roles.join(', ')} />
          </dl>
        </section>

        <section className="rounded-lg border border-line bg-white p-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <KeyRound className="h-5 w-5 text-blue-600" />
                <h2 className="text-base font-semibold text-ink">Bảo mật</h2>
              </div>
              <p className="mt-1 text-sm text-slate-500">Kiểm soát mật khẩu và phiên đăng nhập.</p>
            </div>
            <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={openPassword}>
              <KeyRound className="h-4 w-4" />
              Đổi mật khẩu
            </button>
          </div>
          <div className="mt-5 rounded-md border border-line bg-slate-50 p-4">
            <p className="text-sm font-semibold text-ink">Mật khẩu</p>
            <p className="mt-1 tracking-[0.25em] text-slate-500">••••••••••••</p>
          </div>
          <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-800">
            Đổi email hoặc mật khẩu sẽ đăng xuất tài khoản khỏi tất cả thiết bị. Anh/chị cần đăng nhập lại bằng thông tin mới.
          </div>
        </section>
      </div>

      <Dialog
        open={profileOpen}
        title="Chỉnh sửa thông tin"
        description="Cập nhật thông tin tài khoản và liên hệ cá nhân."
        onClose={() => { if (!updateProfile.isPending) setProfileOpen(false); }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" disabled={updateProfile.isPending} onClick={() => setProfileOpen(false)}>Hủy</button>
            <button type="submit" form="profile-form" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={updateProfile.isPending}>
              {updateProfile.isPending ? 'Đang lưu...' : 'Lưu thay đổi'}
            </button>
          </div>
        )}
      >
        <form id="profile-form" className="space-y-4" onSubmit={profileForm.handleSubmit((values) => updateProfile.mutate(values), (errors) => warnInvalidFields(toast.warn, errors, profileFieldLabels))} noValidate>
          <FormField label="Họ và tên" required error={profileForm.formState.errors.full_name?.message}>
            <input autoComplete="name" className={inputClass(Boolean(profileForm.formState.errors.full_name))} {...profileForm.register('full_name')} />
          </FormField>
          <FormField label="Email đăng nhập" required error={profileForm.formState.errors.email?.message}>
            <input type="email" autoComplete="email" className={inputClass(Boolean(profileForm.formState.errors.email))} {...profileForm.register('email')} />
          </FormField>
          <FormField label="Số điện thoại" error={profileForm.formState.errors.phone?.message}>
            <input type="tel" autoComplete="tel" placeholder="Ví dụ: 0901234567" className={inputClass(Boolean(profileForm.formState.errors.phone))} {...profileForm.register('phone')} />
          </FormField>
          {emailChanged ? (
            <PasswordField
              id="profile-current-password"
              label="Mật khẩu hiện tại"
              required
              autoComplete="current-password"
              visible={Boolean(visiblePasswords.profileCurrent)}
              onToggle={() => setVisiblePasswords((current) => ({ ...current, profileCurrent: !current.profileCurrent }))}
              registration={profileForm.register('current_password')}
              error={profileForm.formState.errors.current_password?.message}
            />
          ) : null}
          {emailChanged ? <p className="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">Sau khi đổi email, hệ thống sẽ đăng xuất tài khoản khỏi tất cả thiết bị.</p> : null}
        </form>
      </Dialog>

      <Dialog
        open={passwordOpen}
        title="Đổi mật khẩu"
        description="Xác nhận mật khẩu hiện tại trước khi đặt mật khẩu mới."
        onClose={() => { if (!changePassword.isPending) setPasswordOpen(false); }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" disabled={changePassword.isPending} onClick={() => setPasswordOpen(false)}>Hủy</button>
            <button type="submit" form="password-form" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={changePassword.isPending}>
              {changePassword.isPending ? 'Đang đổi...' : 'Đổi mật khẩu'}
            </button>
          </div>
        )}
      >
        <form id="password-form" className="space-y-4" onSubmit={passwordForm.handleSubmit((values) => changePassword.mutate(values), (errors) => warnInvalidFields(toast.warn, errors, passwordFieldLabels))} noValidate>
          <PasswordField
            id="current-password"
            label="Mật khẩu hiện tại"
            required
            autoComplete="current-password"
            visible={Boolean(visiblePasswords.current)}
            onToggle={() => setVisiblePasswords((current) => ({ ...current, current: !current.current }))}
            registration={passwordForm.register('current_password')}
            error={passwordForm.formState.errors.current_password?.message}
          />
          <PasswordField
            id="new-password"
            label="Mật khẩu mới"
            required
            autoComplete="new-password"
            visible={Boolean(visiblePasswords.next)}
            onToggle={() => setVisiblePasswords((current) => ({ ...current, next: !current.next }))}
            registration={passwordForm.register('new_password')}
            error={passwordForm.formState.errors.new_password?.message}
          />
          <PasswordField
            id="confirm-password"
            label="Nhập lại mật khẩu mới"
            required
            autoComplete="new-password"
            visible={Boolean(visiblePasswords.confirm)}
            onToggle={() => setVisiblePasswords((current) => ({ ...current, confirm: !current.confirm }))}
            registration={passwordForm.register('new_password_confirmation')}
            error={passwordForm.formState.errors.new_password_confirmation?.message}
          />
          <p className="rounded-md bg-slate-50 px-3 py-2 text-sm leading-5 text-slate-600">Mật khẩu mới phải có ít nhất 8 ký tự. Sau khi đổi, hệ thống sẽ đăng xuất tài khoản khỏi tất cả thiết bị.</p>
        </form>
      </Dialog>
    </>
  );
}

function ProfileValue({ icon: Icon, label, value }: { icon: typeof UserRound; label: string; value?: string }) {
  return (
    <div className="min-w-0 rounded-md border border-line p-3">
      <dt className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-400"><Icon className="h-3.5 w-3.5" />{label}</dt>
      <dd className="mt-1 break-words text-sm font-medium text-ink">{value || 'Chưa cập nhật'}</dd>
    </div>
  );
}

function FormField({ label, required, error, children }: { label: string; required?: boolean; error?: string; children: React.ReactNode }) {
  return (
    <label className="block text-sm font-medium text-slate-700">
      <span>{label}{required ? <span className="text-rose-600"> *</span> : null}</span>
      <span className="mt-1 block">{children}</span>
      {error ? <span className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
    </label>
  );
}

function PasswordField({ id, label, required, autoComplete, visible, onToggle, registration, error }: {
  id: string;
  label: string;
  required?: boolean;
  autoComplete: string;
  visible: boolean;
  onToggle: () => void;
  registration: UseFormRegisterReturn;
  error?: string;
}) {
  return (
    <FormField label={label} required={required} error={error}>
      <span className="relative block">
        <input id={id} type={visible ? 'text' : 'password'} autoComplete={autoComplete} className={`${inputClass(Boolean(error))} pr-11`} {...registration} />
        <button type="button" className="absolute inset-y-0 right-0 grid w-10 place-items-center text-slate-500 hover:text-ink" onClick={onToggle} aria-label={visible ? `Ẩn ${label.toLowerCase()}` : `Hiện ${label.toLowerCase()}`}>
          {visible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </span>
    </FormField>
  );
}

function inputClass(invalid: boolean) {
  return `w-full rounded-md border px-3 py-2 text-sm text-ink outline-none focus:ring-2 ${invalid ? 'border-rose-400 bg-rose-50 focus:border-rose-500 focus:ring-rose-100' : 'border-line bg-white focus:border-blue-500 focus:ring-blue-100'}`;
}

const profileFieldLabels: Record<keyof ProfileFormValues, string> = {
  full_name: 'Họ và tên',
  email: 'Email đăng nhập',
  phone: 'Số điện thoại',
  current_password: 'Mật khẩu hiện tại',
};

const passwordFieldLabels: Record<keyof PasswordFormValues, string> = {
  current_password: 'Mật khẩu hiện tại',
  new_password: 'Mật khẩu mới',
  new_password_confirmation: 'Nhập lại mật khẩu mới',
};

function warnInvalidFields<T extends Record<string, unknown>>(
  warn: (input: { title: string; description?: string }) => void,
  errors: FieldErrors<T>,
  labels: Record<keyof T, string>,
) {
  const invalid = Object.keys(errors).map((key) => labels[key as keyof T]).filter(Boolean);
  warn({ title: 'Vui lòng kiểm tra thông tin', description: invalid.length ? `Trường cần kiểm tra: ${invalid.join(', ')}.` : undefined });
}

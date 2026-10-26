import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, type FieldErrors, type UseFormRegisterReturn } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { BriefcaseBusiness, Building2, ImagePlus, KeyRound, Loader2, Mail, Pencil, Phone, ShieldCheck, Trash2, Upload, UserRound } from 'lucide-react';
import { z } from 'zod';
import { Dialog } from '../../components/Dialog';
import { PageHeader } from '../../components/PageHeader';
import { ContextHelp } from '../../components/ContextHelp';
import { PasswordInput } from '../../components/PasswordInput';
import { UserAvatar } from '../../components/UserAvatar';
import { useToast } from '../../components/ToastProvider';
import { api, postFormWithProgress, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { PASSWORD_RULE_HELPER, passwordConfirmationMessage, passwordValueSchema } from '../../lib/security/password';
import type { User } from '../../lib/types/domain';
import { uniqueRoleNames } from '../../lib/utils/roles';
import { firstInvalidFieldKey, useValidationNavigation } from '../../lib/utils/formValidationNavigation';

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

type RecoveryEmailFormValues = {
  recovery_email: string;
  current_password: string;
};

type RemoveRecoveryEmailFormValues = {
  current_password: string;
};

type ProfileUpdateResponse = {
  user: User;
  reauth_required: boolean;
};

const avatarAccept = 'image/png,image/jpeg,image/webp,.png,.jpg,.jpeg,.webp';
const maxAvatarBytes = 2 * 1024 * 1024;

const passwordSchema = z.object({
  current_password: z.string().min(1, 'Nhập mật khẩu hiện tại'),
  new_password: passwordValueSchema(),
  new_password_confirmation: z.string().min(1, 'Nhập lại mật khẩu mới'),
}).superRefine((values, context) => {
  const confirmationError = passwordConfirmationMessage(values.new_password_confirmation, values.new_password);
  if (confirmationError) context.addIssue({ code: 'custom', path: ['new_password_confirmation'], message: confirmationError });
  if (values.current_password && values.current_password === values.new_password) {
    context.addIssue({ code: 'custom', path: ['new_password'], message: 'Mật khẩu mới phải khác mật khẩu hiện tại' });
  }
});

const recoveryEmailSchema = z.object({
  recovery_email: z.string().trim().email('Email khôi phục không hợp lệ'),
  current_password: z.string().min(1, 'Nhập mật khẩu hiện tại'),
});

const removeRecoveryEmailSchema = z.object({
  current_password: z.string().min(1, 'Nhập mật khẩu hiện tại'),
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
  const [avatarOpen, setAvatarOpen] = useState(false);
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [avatarPreview, setAvatarPreview] = useState('');
  const [avatarError, setAvatarError] = useState('');
  const [avatarProgress, setAvatarProgress] = useState(0);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [recoveryOpen, setRecoveryOpen] = useState(false);
  const [removeRecoveryOpen, setRemoveRecoveryOpen] = useState(false);
  const [visiblePasswords, setVisiblePasswords] = useState<Record<string, boolean>>({});
  const profileFormRef = useRef<HTMLFormElement | null>(null);
  const passwordFormRef = useRef<HTMLFormElement | null>(null);
  const recoveryFormRef = useRef<HTMLFormElement | null>(null);
  const removeRecoveryFormRef = useRef<HTMLFormElement | null>(null);
  const navigateProfileValidation = useValidationNavigation(profileFormRef);
  const navigatePasswordValidation = useValidationNavigation(passwordFormRef);
  const navigateRecoveryValidation = useValidationNavigation(recoveryFormRef);
  const navigateRemoveRecoveryValidation = useValidationNavigation(removeRecoveryFormRef);
  const currentEmail = user?.email ?? '';
  const currentProfileSchema = useMemo(() => profileSchema(currentEmail), [currentEmail]);

  useEffect(() => {
    if (!avatarFile) {
      setAvatarPreview('');
      return undefined;
    }
    const preview = URL.createObjectURL(avatarFile);
    setAvatarPreview(preview);
    return () => URL.revokeObjectURL(preview);
  }, [avatarFile]);

  const profileForm = useForm<ProfileFormValues>({
    resolver: zodResolver(currentProfileSchema),
    defaultValues: { full_name: '', email: '', phone: '', current_password: '' },
  });
  const passwordForm = useForm<PasswordFormValues>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { current_password: '', new_password: '', new_password_confirmation: '' },
  });
  const recoveryForm = useForm<RecoveryEmailFormValues>({
    resolver: zodResolver(recoveryEmailSchema),
    defaultValues: { recovery_email: '', current_password: '' },
  });
  const removeRecoveryForm = useForm<RemoveRecoveryEmailFormValues>({
    resolver: zodResolver(removeRecoveryEmailSchema),
    defaultValues: { current_password: '' },
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

  const updateAvatar = useMutation({
    mutationFn: (file: File) => {
      const formData = new FormData();
      formData.append('file', file);
      return postFormWithProgress<User>('/auth/avatar/upload', formData, ({ percent }) => setAvatarProgress(percent));
    },
    onSuccess: async () => {
      setAvatarOpen(false);
      setAvatarFile(null);
      setAvatarError('');
      setAvatarProgress(0);
      try {
        await refreshUser();
        toast.success('Đã cập nhật ảnh đại diện');
      } catch (error) {
        toast.error('Đã lưu ảnh nhưng chưa tải lại được hồ sơ', error);
      }
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : 'Không tải được ảnh đại diện';
      setAvatarError(message);
      toast.error('Không tải được ảnh đại diện', error);
    },
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

  const updateRecoveryEmail = useMutation({
    mutationFn: (values: RecoveryEmailFormValues) => put<{ verification_sent: boolean; pending_recovery_email_masked: string }>('/auth/recovery-email', {
      recovery_email: values.recovery_email.trim().toLowerCase(),
      current_password: values.current_password,
    }),
    onSuccess: async () => {
      setRecoveryOpen(false);
      try {
        await refreshUser();
        toast.success({ title: 'Đã gửi email xác minh', description: 'Email khôi phục hiện tại vẫn được giữ cho đến khi địa chỉ mới được xác minh.' });
      } catch (error) {
        toast.error('Đã gửi email nhưng chưa tải lại được hồ sơ', error);
      }
    },
    onError: (error) => toast.error('Không cập nhật được email khôi phục', error),
  });

  const removeRecoveryEmail = useMutation({
    mutationFn: (values: RemoveRecoveryEmailFormValues) => api<{ recovery_email_removed: boolean }>('/auth/recovery-email', {
      method: 'DELETE',
      body: JSON.stringify(values),
    }),
    onSuccess: async () => {
      setRemoveRecoveryOpen(false);
      try {
        await refreshUser();
        toast.success('Đã gỡ email khôi phục');
      } catch (error) {
        toast.error('Đã gỡ email nhưng chưa tải lại được hồ sơ', error);
      }
    },
    onError: (error) => toast.error('Không gỡ được email khôi phục', error),
  });

  if (!user) {
    return <div className="rounded-lg border border-line bg-white p-6 text-sm text-slate-600">Không tải được hồ sơ người dùng.</div>;
  }

  const roles = uniqueRoleNames(user.roles);
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
  const resetAvatarDialog = () => {
    setAvatarFile(null);
    setAvatarError('');
    setAvatarProgress(0);
  };
  const openAvatar = () => {
    resetAvatarDialog();
    setAvatarOpen(true);
  };
  const selectAvatar = (file: File | null) => {
    if (!file) return;
    const type = file.type.toLowerCase();
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(type)) {
      setAvatarFile(null);
      setAvatarError('Chỉ hỗ trợ ảnh PNG, JPG hoặc WebP.');
      return;
    }
    if (file.size > maxAvatarBytes) {
      setAvatarFile(null);
      setAvatarError('Ảnh đại diện không được vượt quá 2 MB.');
      return;
    }
    setAvatarError('');
    setAvatarProgress(0);
    setAvatarFile(file);
  };
  const openPassword = () => {
    passwordForm.reset({ current_password: '', new_password: '', new_password_confirmation: '' });
    setVisiblePasswords({});
    setPasswordOpen(true);
  };
  const openRecovery = () => {
    recoveryForm.reset({ recovery_email: '', current_password: '' });
    setVisiblePasswords((current) => ({ ...current, recoveryCurrent: false }));
    setRecoveryOpen(true);
  };
  const openRemoveRecovery = () => {
    removeRecoveryForm.reset({ current_password: '' });
    setVisiblePasswords((current) => ({ ...current, removeRecoveryCurrent: false }));
    setRemoveRecoveryOpen(true);
  };

  return (
    <>
      <PageHeader title="Hồ sơ cá nhân" />

      <section className="mb-4 flex flex-col gap-4 rounded-lg border border-line bg-white p-5 sm:flex-row sm:items-center">
        <UserAvatar user={user} size="lg" decorative />
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-xl font-semibold text-ink">{user.full_name}</h2>
          <p className="mt-1 truncate text-sm text-slate-500">{user.email}{roles.length ? ` · ${roles.join(', ')}` : ''}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={openAvatar}>
            <ImagePlus className="h-4 w-4" />
            Thay ảnh đại diện
          </button>
          <span className="inline-flex w-fit rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700">Đang hoạt động</span>
        </div>
      </section>

      <div className="grid gap-4 xl:grid-cols-2">
        <section className="rounded-lg border border-line bg-white p-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <UserRound className="h-5 w-5 text-blue-600" />
                <h2 className="text-base font-semibold text-ink">Thông tin cá nhân</h2>
              </div>
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
          <div className="mt-4 rounded-md border border-line p-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <Mail className="h-4 w-4 text-blue-600" />
                  <p className="text-sm font-semibold text-ink">Email khôi phục</p>
                </div>
                {!user.recovery_email_available ? <p className="mt-1 text-sm text-slate-500">Quản trị hệ thống chưa bật tính năng gửi email khôi phục.</p> : null}
                {user.recovery_email_available && user.recovery_email_verified ? <p className="mt-1 break-words text-sm text-emerald-700">Đã xác minh · {user.recovery_email_masked}</p> : null}
                {user.recovery_email_available && !user.recovery_email_verified && !user.pending_recovery_email_masked ? <p className="mt-1 text-sm text-slate-500">Chưa thiết lập</p> : null}
                {user.recovery_email_available && user.pending_recovery_email_masked ? <p className="mt-1 break-words text-sm text-amber-700">Đang chờ xác minh · {user.pending_recovery_email_masked}</p> : null}
                {user.recovery_email_available && user.recovery_email_verified && user.pending_recovery_email_masked ? <p className="mt-1 text-xs leading-5 text-slate-500">Địa chỉ đã xác minh hiện tại vẫn hiển thị cho Developer khi xử lý yêu cầu.</p> : null}
              </div>
              {user.recovery_email_available ? (
                <div className="flex shrink-0 flex-wrap gap-2">
                  <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={openRecovery}>
                    {user.recovery_email_verified || user.pending_recovery_email_masked ? 'Thay đổi' : 'Thiết lập'}
                  </button>
                  {user.recovery_email_verified || user.pending_recovery_email_masked ? (
                    <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50" onClick={openRemoveRecovery}>
                      <Trash2 className="h-4 w-4" /> Gỡ
                    </button>
                  ) : null}
                </div>
              ) : null}
            </div>
          </div>
          <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-800">
            Đổi email hoặc mật khẩu sẽ đăng xuất tài khoản khỏi tất cả thiết bị. Anh/chị cần đăng nhập lại bằng thông tin mới.
          </div>
        </section>
      </div>

      <Dialog
        open={profileOpen}
        title="Chỉnh sửa thông tin"
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
        <form ref={profileFormRef} id="profile-form" className="space-y-4" onSubmit={profileForm.handleSubmit((values) => updateProfile.mutate(values), (errors) => {
          warnInvalidFields(toast.warn, errors, profileFieldLabels);
          navigateProfileValidation(firstInvalidFieldKey(errors as Record<string, unknown>, Object.keys(profileFieldLabels)));
        })} noValidate>
          <FormField fieldKey="full_name" label="Họ và tên" required error={profileForm.formState.errors.full_name?.message} errorId="profile-full-name-error">
            <input autoComplete="name" aria-invalid={Boolean(profileForm.formState.errors.full_name)} aria-describedby={profileForm.formState.errors.full_name ? 'profile-full-name-error' : undefined} className={inputClass(Boolean(profileForm.formState.errors.full_name))} {...profileForm.register('full_name')} />
          </FormField>
          <FormField fieldKey="email" label="Email đăng nhập" required error={profileForm.formState.errors.email?.message} errorId="profile-email-error">
            <input type="email" autoComplete="email" aria-invalid={Boolean(profileForm.formState.errors.email)} aria-describedby={profileForm.formState.errors.email ? 'profile-email-error' : undefined} className={inputClass(Boolean(profileForm.formState.errors.email))} {...profileForm.register('email')} />
          </FormField>
          <FormField fieldKey="phone" label="Số điện thoại" error={profileForm.formState.errors.phone?.message} errorId="profile-phone-error">
            <input type="tel" autoComplete="tel" placeholder="Ví dụ: 0901234567" aria-invalid={Boolean(profileForm.formState.errors.phone)} aria-describedby={profileForm.formState.errors.phone ? 'profile-phone-error' : undefined} className={inputClass(Boolean(profileForm.formState.errors.phone))} {...profileForm.register('phone')} />
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
        open={avatarOpen}
        title="Cập nhật ảnh đại diện"
        size="sm"
        onClose={() => {
          if (updateAvatar.isPending) return;
          setAvatarOpen(false);
          resetAvatarDialog();
        }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" disabled={updateAvatar.isPending} onClick={() => {
              setAvatarOpen(false);
              resetAvatarDialog();
            }}>Hủy</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={updateAvatar.isPending} onClick={() => {
              if (!avatarFile) {
                const message = 'Chọn một ảnh đại diện trước khi lưu.';
                setAvatarError(message);
                toast.warn({ title: 'Thiếu ảnh đại diện', description: message });
                return;
              }
              updateAvatar.mutate(avatarFile);
            }}>
              {updateAvatar.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
              {updateAvatar.isPending ? 'Đang tải...' : 'Lưu ảnh'}
            </button>
          </div>
        )}
      >
        <div className="space-y-4">
          <div className="flex justify-center">
            {avatarPreview ? <img className="h-24 w-24 rounded-full border border-line object-cover" src={avatarPreview} alt="Xem trước ảnh đại diện mới" /> : <UserAvatar user={user} size="xl" decorative />}
          </div>
          <div className={`rounded-lg border-2 border-dashed p-4 text-center ${avatarError ? 'border-rose-300 bg-rose-50' : 'border-slate-300 bg-slate-50'}`}>
            <div className="flex flex-wrap items-center justify-center gap-2">
              <p className="text-sm font-semibold text-ink">Chọn ảnh đại diện</p>
              <ContextHelp label="Về cách lưu ảnh đại diện">Ảnh được chuẩn hóa và lưu trong kho bảo vệ.</ContextHelp>
            </div>
            <p className="mt-1 text-xs text-slate-500">PNG, JPG hoặc WebP · tối đa 2 MB · ảnh được hiển thị dạng tròn.</p>
            <label className="mt-3 inline-flex cursor-pointer items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50">
              <Upload className="h-4 w-4" /> Chọn ảnh
              <input className="sr-only" type="file" accept={avatarAccept} disabled={updateAvatar.isPending} onChange={(event) => {
                selectAvatar(event.target.files?.[0] ?? null);
                event.currentTarget.value = '';
              }} />
            </label>
            {avatarFile ? <p className="mt-3 break-all text-xs text-slate-600">{avatarFile.name} · {formatFileSize(avatarFile.size)}</p> : null}
            {avatarError ? <p role="alert" className="mt-3 text-xs font-medium text-rose-700">{avatarError}</p> : null}
            {updateAvatar.isPending ? <p role="status" className="mt-3 text-xs font-medium text-blue-700">Đang tải ảnh {avatarProgress > 0 ? `${avatarProgress}%` : ''}</p> : null}
          </div>
        </div>
      </Dialog>

      <Dialog
        open={passwordOpen}
        title="Đổi mật khẩu"
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
        <form ref={passwordFormRef} id="password-form" className="space-y-4" onSubmit={passwordForm.handleSubmit((values) => changePassword.mutate(values), (errors) => {
          warnInvalidFields(toast.warn, errors, passwordFieldLabels);
          navigatePasswordValidation(firstInvalidFieldKey(errors as Record<string, unknown>, Object.keys(passwordFieldLabels)));
        })} noValidate>
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
          <p className="rounded-md bg-slate-50 px-3 py-2 text-sm leading-5 text-slate-600">{PASSWORD_RULE_HELPER} Sau khi đổi, hệ thống sẽ đăng xuất tài khoản khỏi tất cả thiết bị.</p>
        </form>
      </Dialog>

      <Dialog
        open={recoveryOpen}
        title={user.recovery_email_verified || user.pending_recovery_email_masked ? 'Thay đổi email khôi phục' : 'Thiết lập email khôi phục'}
        description="Hệ thống sẽ gửi liên kết xác minh đến địa chỉ mới. Email này không dùng để đăng nhập."
        onClose={() => { if (!updateRecoveryEmail.isPending) setRecoveryOpen(false); }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" disabled={updateRecoveryEmail.isPending} onClick={() => setRecoveryOpen(false)}>Hủy</button>
            <button type="submit" form="recovery-email-form" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={updateRecoveryEmail.isPending}>{updateRecoveryEmail.isPending ? 'Đang gửi...' : 'Gửi email xác minh'}</button>
          </div>
        )}
      >
        <form ref={recoveryFormRef} id="recovery-email-form" className="space-y-4" onSubmit={recoveryForm.handleSubmit((values) => updateRecoveryEmail.mutate(values), (errors) => {
          warnInvalidFields(toast.warn, errors, recoveryEmailFieldLabels);
          navigateRecoveryValidation(firstInvalidFieldKey(errors as Record<string, unknown>, Object.keys(recoveryEmailFieldLabels)));
        })} noValidate>
          <FormField fieldKey="recovery_email" label="Email khôi phục mới" required error={recoveryForm.formState.errors.recovery_email?.message} errorId="recovery-email-error">
            <input type="email" autoComplete="email" placeholder="vidu@email.com" aria-invalid={Boolean(recoveryForm.formState.errors.recovery_email)} aria-describedby={recoveryForm.formState.errors.recovery_email ? 'recovery-email-error' : undefined} className={inputClass(Boolean(recoveryForm.formState.errors.recovery_email))} {...recoveryForm.register('recovery_email')} />
          </FormField>
          <PasswordField
            id="recovery-current-password"
            label="Mật khẩu hiện tại"
            required
            autoComplete="current-password"
            visible={Boolean(visiblePasswords.recoveryCurrent)}
            onToggle={() => setVisiblePasswords((current) => ({ ...current, recoveryCurrent: !current.recoveryCurrent }))}
            registration={recoveryForm.register('current_password')}
            error={recoveryForm.formState.errors.current_password?.message}
          />
          <p className="rounded-md bg-blue-50 px-3 py-2 text-sm leading-5 text-blue-800">Email đăng nhập không thay đổi. Email này chỉ là thông tin liên hệ bổ sung; yêu cầu quên mật khẩu do Developer xác minh và xử lý.</p>
        </form>
      </Dialog>

      <Dialog
        open={removeRecoveryOpen}
        title="Gỡ email khôi phục"
        description="Sau khi gỡ, Developer sẽ không thấy email liên hệ bổ sung khi xử lý yêu cầu khôi phục. Tài khoản vẫn có thể gửi yêu cầu."
        onClose={() => { if (!removeRecoveryEmail.isPending) setRemoveRecoveryOpen(false); }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" disabled={removeRecoveryEmail.isPending} onClick={() => setRemoveRecoveryOpen(false)}>Hủy</button>
            <button type="submit" form="remove-recovery-email-form" className="rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:opacity-60" disabled={removeRecoveryEmail.isPending}>{removeRecoveryEmail.isPending ? 'Đang gỡ...' : 'Gỡ email khôi phục'}</button>
          </div>
        )}
      >
        <form ref={removeRecoveryFormRef} id="remove-recovery-email-form" className="space-y-4" onSubmit={removeRecoveryForm.handleSubmit((values) => removeRecoveryEmail.mutate(values), (errors) => {
          warnInvalidFields(toast.warn, errors, removeRecoveryEmailFieldLabels);
          navigateRemoveRecoveryValidation(firstInvalidFieldKey(errors as Record<string, unknown>, Object.keys(removeRecoveryEmailFieldLabels)));
        })} noValidate>
          <PasswordField
            id="remove-recovery-current-password"
            label="Mật khẩu hiện tại"
            required
            autoComplete="current-password"
            visible={Boolean(visiblePasswords.removeRecoveryCurrent)}
            onToggle={() => setVisiblePasswords((current) => ({ ...current, removeRecoveryCurrent: !current.removeRecoveryCurrent }))}
            registration={removeRecoveryForm.register('current_password')}
            error={removeRecoveryForm.formState.errors.current_password?.message}
          />
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

function FormField({ fieldKey, label, required, error, errorId, children }: { fieldKey: string; label: string; required?: boolean; error?: string; errorId?: string; children: React.ReactNode }) {
  return (
    <label data-validation-key={fieldKey} className="block scroll-mt-24 rounded-md text-sm font-medium text-slate-700">
      <span>{label}{required ? <span className="text-rose-600"> *</span> : null}</span>
      <span className="mt-1 block">{children}</span>
      {error ? <span id={errorId} role="alert" className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
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
    <FormField fieldKey={registration.name} label={label} required={required} error={error} errorId={`${id}-error`}>
      <span className="block">
        <PasswordInput
          id={id}
          autoComplete={autoComplete}
          className={inputClass(Boolean(error))}
          visible={visible}
          onVisibilityChange={onToggle}
          invalid={Boolean(error)}
          aria-describedby={error ? `${id}-error` : undefined}
          {...registration}
        />
      </span>
    </FormField>
  );
}

function inputClass(invalid: boolean) {
  return `w-full rounded-md border px-3 py-2 text-sm text-ink outline-none focus:ring-2 ${invalid ? 'border-rose-400 bg-rose-50 focus:border-rose-500 focus:ring-rose-100' : 'border-line bg-white focus:border-blue-500 focus:ring-blue-100'}`;
}

function formatFileSize(value: number) {
  return `${Math.max(1, Math.ceil(value / 1024))} KB`;
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

const recoveryEmailFieldLabels: Record<keyof RecoveryEmailFormValues, string> = {
  recovery_email: 'Email khôi phục mới',
  current_password: 'Mật khẩu hiện tại',
};

const removeRecoveryEmailFieldLabels: Record<keyof RemoveRecoveryEmailFormValues, string> = {
  current_password: 'Mật khẩu hiện tại',
};

function warnInvalidFields<T extends Record<string, unknown>>(
  warn: (input: { title: string; description?: string }) => void,
  errors: FieldErrors<T>,
  labels: Record<keyof T, string>,
) {
  const invalid = Object.keys(errors).map((key) => labels[key as keyof T]).filter(Boolean);
  warn({ title: 'Vui lòng kiểm tra thông tin', description: invalid.length ? `Trường cần kiểm tra: ${invalid.join(', ')}.` : undefined });
}

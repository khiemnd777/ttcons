import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { AlertCircle, Plus, Search, ShieldCheck, Upload } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { PageHeader } from '../../components/PageHeader';
import { DataTable, type DataTableRowKey } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RowActions } from '../../components/RowActions';
import { Dialog } from '../../components/Dialog';
import { SimpleForm } from '../_shared/SimpleForm';
import { isAssignableAppRole } from '../../lib/utils/roles';
import { enumOptions } from '../../lib/utils/status';
import {
  generateSuggestedPassword,
  PASSWORD_RULE_HELPER,
  passwordConfirmationMessage,
  passwordValidationMessage,
} from '../../lib/security/password';
import { StaffImportDialog } from './StaffImportDialog';
import type { StaffImportResult } from './staffImportTypes';
import { StaffDeleteDialog, type StaffDeletionResult } from './StaffDeleteDialog';
import { BulkSelectionBar } from '../../components/BulkSelectionBar';
import { SampleDataBadge } from '../../components/SampleDataBadge';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';

type StaffRole = {
  id: string;
  name: string;
  display_name: string;
  project_id?: string;
};

type StaffRow = Record<string, any> & {
  id: string;
  user_id?: string;
  full_name: string;
  email: string;
  has_recovery_email?: boolean;
  recovery_email_masked?: string;
  user_roles?: StaffRole[];
  is_sample?: boolean;
  sample_role_id?: string;
};

type UpdateStaffResponse = StaffRow & {
  password_changed?: boolean;
  recovery_email_changed?: boolean;
  identity_changed?: boolean;
  reauth_required?: boolean;
};

type RoleRow = Record<string, any> & {
  id: string;
  name: string;
  display_name: string;
  external_only?: boolean;
};

type ProjectOption = Record<string, any> & {
  id: string;
  name: string;
  code?: string;
};

export function StaffPage() {
  const [open, setOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [dialog, setDialog] = useState<StaffRow | null>(null);
  const [roleDialog, setRoleDialog] = useState<StaffRow | null>(null);
  const [selectedKeys, setSelectedKeys] = useState<Set<DataTableRowKey>>(new Set());
  const [deleteOpen, setDeleteOpen] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const navigate = useNavigate();
  const { logout, hasPermission } = useAuth();
  const canCreate = hasPermission('staff.create') && hasPermission('staff.account_security.manage');
  const canUpdate = hasPermission('staff.update');
  const canManageAccountSecurity = hasPermission('staff.account_security.manage');
  const canAssign = hasPermission('permission.assign');
  const canImport = hasPermission('staff.import') && canCreate && canAssign;
  const canDelete = hasPermission('staff.delete') && canManageAccountSecurity;

  const staff = useQuery({ queryKey: ['staff'], queryFn: () => get<StaffRow[]>('/staff') });
  const roles = useQuery({ queryKey: ['roles'], queryFn: () => get<RoleRow[]>('/roles'), enabled: canAssign || canImport });
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<ProjectOption[]>('/projects'), enabled: canAssign });

  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post('/staff', values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['staff'] });
      setOpen(false);
      toast.success('Đã tạo nhân sự và tài khoản đăng nhập');
    },
    onError: (error) => toast.error('Không tạo được nhân sự', error),
  });
  const update = useMutation({
    mutationFn: ({ staffId, values }: { staffId: string; values: Record<string, any> }) => put<UpdateStaffResponse>(`/staff/${staffId}`, values),
    onSuccess: async (result) => {
      if (result.reauth_required) {
        setDialog(null);
        toast.success(result.password_changed
          ? 'Đã cập nhật nhân sự và mật khẩu. Vui lòng đăng nhập lại.'
          : 'Đã cập nhật danh tính tài khoản. Vui lòng đăng nhập lại.');
        await logout();
        navigate('/login', { replace: true });
        return;
      }
      await queryClient.invalidateQueries({ queryKey: ['staff'] });
      setDialog(null);
      if (result.password_changed || result.recovery_email_changed) {
        toast.success('Đã cập nhật thông tin và bảo mật tài khoản nhân sự');
        return;
      }
      toast.success('Đã cập nhật nhân sự');
    },
    onError: (error) => toast.error('Không cập nhật được nhân sự', error),
  });
  const assignRoles = useMutation({
    mutationFn: ({ userId, roleIds, projectId }: { userId: string; roleIds: string[]; projectId: string }) => post(`/users/${userId}/roles`, { role_ids: roleIds, project_id: projectId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['staff'] });
      setRoleDialog(null);
      toast.success('Đã cập nhật vai trò cho nhân sự');
    },
    onError: (error) => toast.error('Không cập nhật được vai trò', error),
  });

  const staffFields = (record?: StaffRow) => [
    { name: 'full_name', label: 'Họ tên', defaultValue: record?.full_name ?? '', required: true },
    { name: 'email', label: 'Email đăng nhập', type: 'email' as const, defaultValue: record?.email ?? '', required: true },
    { name: 'phone', label: 'Điện thoại', defaultValue: record?.phone ?? '' },
    { name: 'department', label: 'Phòng ban', defaultValue: record?.department ?? '', placeholder: 'Không bắt buộc' },
    { name: 'position', label: 'Chức vụ', defaultValue: record?.position ?? '', placeholder: 'Không bắt buộc', helperText: 'Không cấp quyền hệ thống.' },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'active', required: true, options: enumOptions('status', ['active', 'inactive']) },
  ];

  const staffCreateFields = () => [
    ...staffFields(),
    {
      name: 'password',
      label: 'Mật khẩu ban đầu',
      type: 'password' as const,
      defaultValue: '',
      required: true,
      fullWidth: true,
      helperText: PASSWORD_RULE_HELPER,
      validate: (value: unknown) => passwordValidationMessage(String(value ?? ''), 'Mật khẩu'),
      suggestValue: generateSuggestedPassword,
      suggestLabel: 'Đề xuất mật khẩu',
    },
    {
      name: 'recovery_email',
      label: 'Email khôi phục',
      type: 'email' as const,
      defaultValue: '',
      fullWidth: true,
      placeholder: 'Không bắt buộc',
      helperText: 'Email liên hệ bổ sung cho quá trình xác minh yêu cầu khôi phục. Developer vẫn phải xác minh danh tính trước khi cấp liên kết.',
      validate: validateOptionalRecoveryEmail,
    },
  ];

  const staffEditFields = (record: StaffRow) => [
    ...staffFields(record),
    ...(canManageAccountSecurity && record.user_id ? [
      {
        name: 'new_password',
        label: 'Mật khẩu mới',
        type: 'password' as const,
        defaultValue: '',
        placeholder: 'Để trống nếu không thay đổi',
        helperText: `${PASSWORD_RULE_HELPER} Lưu mật khẩu mới sẽ đăng xuất tài khoản này khỏi mọi thiết bị.`,
        validate: validateOptionalPassword,
      },
      {
        name: 'new_password_confirmation',
        label: 'Nhập lại mật khẩu mới',
        type: 'password' as const,
        defaultValue: '',
        placeholder: 'Nhập lại nếu đổi mật khẩu',
        validate: validateOptionalPasswordConfirmation,
      },
      {
        name: 'recovery_email',
        label: 'Email khôi phục mới',
        type: 'email' as const,
        defaultValue: '',
        placeholder: 'Nhập email mới; để trống nếu không thay đổi',
        helperText: recoveryEmailHelperText(record),
        validate: validateOptionalRecoveryEmail,
      },
    ] : []),
  ];

  const createFields = useMemo(staffCreateFields, []);
  const editFields = useMemo(() => dialog ? staffEditFields(dialog) : [], [canManageAccountSecurity, dialog]);
  const selectedStaffIDs = useMemo(() => Array.from(selectedKeys, String), [selectedKeys]);

  const rowActions = (row: StaffRow) => (
    <RowActions onEdit={canUpdate ? () => setDialog(row) : undefined}>
      {canAssign ? (
        <button
          title={row.is_sample ? 'Tài khoản mẫu được gán vai trò tự động theo công trình mẫu' : row.user_id ? 'Gán vai trò' : 'Nhân sự chưa liên kết tài khoản'}
          aria-label="Gán vai trò"
          className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
          onClick={() => setRoleDialog(row)}
          disabled={!row.user_id || row.is_sample}
        >
          <ShieldCheck className="h-4 w-4" />
        </button>
      ) : null}
    </RowActions>
  );

  return (
    <>
      <PageHeader
        title="Nhân sự"
        actions={(canCreate || canImport) ? (
          <>
            {canImport ? <button className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setImportOpen(true)}><Upload className="h-4 w-4" />Import</button> : null}
            {canCreate ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Nhân sự</button> : null}
          </>
        ) : null}
      />
      {canDelete ? <BulkSelectionBar count={selectedKeys.size} itemLabel="nhân sự" actionLabel="Xóa nhân sự đã chọn" onClear={() => setSelectedKeys(new Set())} onDelete={() => setDeleteOpen(true)} /> : null}
      <DataTable
        rows={staff.data ?? []}
        loading={staff.isLoading}
        error={staff.error}
        onRetry={() => staff.refetch()}
        searchPlaceholder="Tìm nhân sự..."
        pageSize={12}
        defaultSort={{ key: 'full_name' }}
        getRowKey={(row) => row.id}
        selection={canDelete ? { selectedKeys, onSelectedKeysChange: setSelectedKeys, ariaLabel: 'Chọn tất cả nhân sự trên trang hiện tại' } : undefined}
        renderMobileCard={(row) => <StaffMobileCard row={row} actions={rowActions(row)} />}
        columns={[
          { key: 'full_name', header: 'Họ tên', sortable: true, render: (row) => <span className="flex flex-wrap items-center gap-2 font-semibold text-ink"><span>{row.full_name}</span>{row.is_sample ? <SampleDataBadge compact /> : null}</span> },
          { key: 'email', header: 'Email', sortable: true },
          { key: 'department', header: 'Phòng ban', sortable: true, render: (row) => optionalStaffValue(row.department) },
          { key: 'position', header: 'Chức vụ', render: (row) => optionalStaffValue(row.position) },
          { key: 'roles', header: 'Role', filterValue: (row) => roleLabels(row).join(' '), render: (row) => <RoleSummary roles={row.user_roles ?? []} /> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          {
            key: 'actions',
            header: 'Action',
            render: rowActions,
          },
        ]}
      />
      <StaffDeleteDialog
        open={deleteOpen}
        staffIDs={selectedStaffIDs}
        onClose={() => setDeleteOpen(false)}
        onSuccess={async (result: StaffDeletionResult) => {
          setDeleteOpen(false);
          setSelectedKeys(new Set());
          await queryClient.invalidateQueries();
          toast.success({
            title: `Đã xóa ${result.deleted_count} nhân sự`,
            description: `${result.memberships_deactivated} membership đã ngừng hoạt động; ${result.sessions_revoked} phiên đăng nhập đã bị thu hồi.`,
          });
        }}
        onError={(title, error) => toast.error(title, error)}
      />
      <StaffImportDialog
        open={importOpen}
        roles={roles.data ?? []}
        rolesLoading={roles.isLoading}
        rolesError={roles.error}
        onClose={() => setImportOpen(false)}
        onSuccess={async (result: StaffImportResult) => {
          setImportOpen(false);
          await queryClient.invalidateQueries({ queryKey: ['staff'] });
          toast.success({
            title: `Đã tạo và kích hoạt ${result.created_count} tài khoản`,
            description: `${result.role_assignment_count} vai trò đã gán. Người dùng phải đổi mật khẩu ở lần đăng nhập đầu tiên.`,
          });
        }}
        onError={(title, error) => toast.error(title, error)}
      />
      <Dialog
        open={open}
        title="Tạo nhân sự"
        description="Vai trò được gán sau khi tạo tài khoản."
        size="lg"
        onClose={() => setOpen(false)}
      >
        <SimpleForm
          submitLabel="Tạo nhân sự & tài khoản"
          cancelLabel="Hủy"
          layout="two-column"
          fields={createFields}
          sections={[
            {
              title: 'Thông tin nhân sự',
              description: 'Thông tin liên hệ và hồ sơ công việc.',
              fields: ['full_name', 'email', 'phone', 'department', 'position', 'status'],
            },
            {
              title: 'Tài khoản đăng nhập',
              description: 'Mật khẩu được lưu dưới dạng mã băm; hệ thống không thể hiển thị lại sau khi tạo.',
              fields: ['password', 'recovery_email'],
            },
          ]}
          onCancel={() => setOpen(false)}
          onSubmit={(values) => create.mutateAsync(values)}
        />
      </Dialog>
      <Dialog
        open={Boolean(dialog)}
        title="Chỉnh sửa nhân sự"
        description={dialog ? `${dialog.full_name} · ${dialog.email}` : undefined}
        size="lg"
        onClose={() => setDialog(null)}
      >
        {dialog ? (
          <>
            {canManageAccountSecurity && !dialog.user_id ? (
              <div className="mb-4 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-semibold">Chưa thể quản trị bảo mật tài khoản</p>
                  <p className="mt-1 text-xs leading-5">Nhân sự chưa liên kết tài khoản đăng nhập. Anh/chị vẫn có thể cập nhật thông tin nhân sự bên dưới.</p>
                </div>
              </div>
            ) : null}
            <SimpleForm
              submitLabel="Lưu thay đổi"
              cancelLabel="Hủy"
              layout="two-column"
              fields={editFields}
              sections={[
                {
                  title: 'Thông tin nhân sự',
                  description: 'Thông tin liên hệ và hồ sơ công việc.',
                  fields: ['full_name', 'email', 'phone', 'department', 'position', 'status'],
                },
                ...(canManageAccountSecurity && dialog.user_id ? [{
                  title: 'Bảo mật tài khoản',
                  description: 'Các trường không bắt buộc; để trống để giữ nguyên. Thay đổi được áp dụng trực tiếp, không gửi email xác nhận.',
                  fields: ['new_password', 'new_password_confirmation', 'recovery_email'],
                }] : []),
              ]}
              onCancel={() => setDialog(null)}
              onSubmit={(values) => update.mutateAsync({ staffId: dialog.id, values })}
            />
          </>
        ) : null}
      </Dialog>
      <StaffRoleDialog
        staff={roleDialog}
        roles={(roles.data ?? []).filter(isAssignableAppRole)}
        projects={projects.data ?? []}
        rolesLoading={roles.isLoading}
        projectsLoading={projects.isLoading}
        rolesError={roles.error}
        projectsError={projects.error}
        saving={assignRoles.isPending}
        onClose={() => setRoleDialog(null)}
        onSave={(roleIds, projectId) => roleDialog?.user_id ? assignRoles.mutateAsync({ userId: roleDialog.user_id, roleIds, projectId }) : Promise.resolve()}
        onWarn={(message) => toast.warn(message)}
        onRetryRoles={() => { void roles.refetch(); }}
        onRetryProjects={() => { void projects.refetch(); }}
      />
    </>
  );
}

function StaffMobileCard({ row, actions }: { row: StaffRow; actions: ReactNode }) {
  return (
    <article className="rounded-lg border border-line bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><p className="truncate text-sm font-semibold text-ink">{row.full_name}</p>{row.is_sample ? <SampleDataBadge compact /> : null}</div><p className="mt-1 truncate text-xs text-slate-500">{row.email}</p></div>
        {actions}
      </div>
      <div className="mt-3 grid grid-cols-2 gap-3 text-xs">
        <div><p className="font-medium text-slate-400">Phòng ban</p><p className="mt-1 text-slate-700">{String(row.department ?? '').trim() || '—'}</p></div>
        <div><p className="font-medium text-slate-400">Chức vụ</p><p className="mt-1 text-slate-700">{String(row.position ?? '').trim() || '—'}</p></div>
      </div>
      <div className="mt-3 flex items-end justify-between gap-3"><RoleSummary roles={row.user_roles ?? []} /><StatusBadge status={row.status} /></div>
    </article>
  );
}

function recoveryEmailHelperText(record: StaffRow) {
  if (record.recovery_email_masked) {
    return `Hiện tại: ${record.recovery_email_masked} · Đã xác minh. Đây là email liên hệ bổ sung cho Developer. Để trống để giữ nguyên.`;
  }
  if (record.has_recovery_email) {
    return 'Đã thiết lập email liên hệ bổ sung cho Developer. Để trống để giữ nguyên.';
  }
  return 'Chưa có email liên hệ bổ sung cho Developer. Email mới được xác minh ngay.';
}

function validateOptionalPassword(value: unknown) {
  const password = String(value ?? '');
  if (!password) return undefined;
  return passwordValidationMessage(password);
}

function validateOptionalPasswordConfirmation(value: unknown, values: Record<string, any>) {
  const password = String(values.new_password ?? '');
  const confirmation = String(value ?? '');
  if (!password && !confirmation) return undefined;
  return passwordConfirmationMessage(confirmation, password);
}

function validateOptionalRecoveryEmail(value: unknown) {
  const email = String(value ?? '').trim();
  if (!email) return undefined;
  if (email.length > 254 || /\s/.test(email) || !/^[^@]+@[^@]+$/.test(email)) {
    return 'Email khôi phục không hợp lệ';
  }
  return undefined;
}

function optionalStaffValue(value: unknown) {
  const text = String(value ?? '').trim();
  return text || <span className="text-slate-400">—</span>;
}

function RoleSummary({ roles }: { roles: StaffRole[] }) {
  const globalRoles = roles.filter((role) => !role.project_id);
  if (!globalRoles.length) return <span className="text-slate-400">Chưa gán</span>;
  return (
    <div className="flex flex-wrap gap-1">
      {globalRoles.slice(0, 3).map((role) => (
        <span key={role.id} className="rounded bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{role.display_name}</span>
      ))}
      {globalRoles.length > 3 ? <span className="rounded bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500">+{globalRoles.length - 3}</span> : null}
    </div>
  );
}

function StaffRoleDialog({
  staff,
  roles,
  projects,
  rolesLoading,
  projectsLoading,
  rolesError,
  projectsError,
  saving,
  onClose,
  onSave,
  onWarn,
  onRetryRoles,
  onRetryProjects,
}: {
  staff: StaffRow | null;
  roles: RoleRow[];
  projects: ProjectOption[];
  rolesLoading: boolean;
  projectsLoading: boolean;
  rolesError: unknown;
  projectsError: unknown;
  saving: boolean;
  onClose: () => void;
  onSave: (roleIds: string[], projectId: string) => Promise<unknown>;
  onWarn: (message: string) => void;
  onRetryRoles: () => void;
  onRetryProjects: () => void;
}) {
  const [scope, setScope] = useState<'global' | 'project'>('global');
  const [projectID, setProjectID] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState('');
  const [roleFilter, setRoleFilter] = useState<'all' | 'selected'>('all');
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const dialogContentRef = useRef<HTMLDivElement | null>(null);
  const navigateValidation = useValidationNavigation(dialogContentRef);
  const assignableRoleIDs = useMemo(() => new Set(roles.map((role) => role.id)), [roles]);

  useEffect(() => {
    setScope('global');
    setProjectID('');
    setQuery('');
    setRoleFilter('all');
    setSubmitAttempted(false);
  }, [staff?.id]);

  useEffect(() => {
    const targetProjectID = scope === 'project' ? projectID : '';
    setSelected(new Set((staff?.user_roles ?? []).filter((role) => (role.project_id ?? '') === targetProjectID && assignableRoleIDs.has(role.id)).map((role) => role.id)));
  }, [assignableRoleIDs, projectID, scope, staff?.user_roles]);

  const projectOptions = useMemo(() => projects.map((project) => ({ value: project.id, label: `${project.code ? `${project.code} - ` : ''}${project.name}` })), [projects]);
  const normalizedQuery = normalizeRoleSearch(query);
  const visibleRoles = useMemo(() => roles.filter((role) => {
    if (roleFilter === 'selected' && !selected.has(role.id)) return false;
    if (!normalizedQuery) return true;
    return normalizeRoleSearch(`${role.display_name} ${role.name}`).includes(normalizedQuery);
  }), [normalizedQuery, roleFilter, roles, selected]);
  const projectMissing = submitAttempted && scope === 'project' && !projectID;
  const rolesMissing = submitAttempted && selected.size === 0;
  const selectedProjectLabel = projectOptions.find((option) => option.value === projectID)?.label;
  const scopeSummary = scope === 'global' ? 'Toàn hệ thống' : selectedProjectLabel ?? 'Theo công trình';
  const loading = rolesLoading || (scope === 'project' && projectsLoading);
  const blockedByError = Boolean(rolesError) || (scope === 'project' && Boolean(projectsError));

  function toggle(roleID: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(roleID)) next.delete(roleID);
      else next.add(roleID);
      return next;
    });
  }

  function changeScope(nextScope: 'global' | 'project') {
    setScope(nextScope);
    setSubmitAttempted(false);
  }

  function save() {
    setSubmitAttempted(true);
    const missingProject = scope === 'project' && !projectID;
    const missingRoles = selected.size === 0;
    if (missingProject || missingRoles) {
      if (missingProject && missingRoles) onWarn('Chọn công trình và ít nhất một vai trò trước khi lưu');
      else if (missingProject) onWarn('Chọn công trình trước khi lưu vai trò theo công trình');
      else onWarn('Chọn ít nhất một vai trò cho nhân sự');
      navigateValidation(missingProject ? 'project_id' : 'role_ids');
      return;
    }
    void onSave(Array.from(selected), scope === 'project' ? projectID : '');
  }

  return (
    <Dialog
      open={Boolean(staff)}
      title="Gán vai trò"
      description={staff ? `${staff.full_name} · ${staff.email}` : undefined}
      onClose={onClose}
      size="lg"
      footer={
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs font-medium text-slate-500">{selected.size} vai trò · {scopeSummary}</p>
          <div className="flex w-full items-center gap-2 sm:w-auto">
            <button className="flex-1 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60 sm:flex-none" onClick={onClose} disabled={saving}>Huỷ</button>
            <button className="flex-1 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 sm:flex-none" onClick={save} disabled={saving || loading || blockedByError}>
              {saving ? 'Đang lưu...' : 'Lưu vai trò'}
            </button>
          </div>
        </div>
      }
    >
      <div ref={dialogContentRef} className="space-y-5">
        <fieldset>
          <legend className="text-sm font-semibold text-slate-700">Phạm vi áp dụng</legend>
          <div className="mt-2 grid grid-cols-2 rounded-md border border-line bg-slate-50 p-1">
            {([
              { value: 'global' as const, label: 'Toàn hệ thống' },
              { value: 'project' as const, label: 'Theo công trình' },
            ]).map((option) => (
              <button
                key={option.value}
                type="button"
                aria-pressed={scope === option.value}
                className={`rounded px-3 py-2 text-sm font-semibold transition ${scope === option.value ? 'bg-white text-blue-700 shadow-sm ring-1 ring-blue-100' : 'text-slate-600 hover:text-ink'}`}
                onClick={() => changeScope(option.value)}
                disabled={saving}
              >
                {option.label}
              </button>
            ))}
          </div>
          {scope === 'global' ? (
            <p className="mt-2 text-xs leading-5 text-slate-500">Vai trò có hiệu lực toàn hệ thống; quyền vào từng công trình vẫn phụ thuộc phạm vi được cấp.</p>
          ) : (
            <div className="mt-3" data-validation-key="project_id" data-validation-error={projectMissing ? 'true' : undefined}>
              <label className="block">
                <span className="text-sm font-semibold text-slate-700">Công trình <span className="text-rose-600">*</span></span>
                <select
                  name="project_id"
                  className={`mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring disabled:bg-slate-100 disabled:text-slate-500 ${projectMissing ? 'border-rose-400 bg-rose-50' : 'border-line'}`}
                  value={projectID}
                  onChange={(event) => setProjectID(event.target.value)}
                  disabled={saving || projectsLoading || Boolean(projectsError)}
                  aria-invalid={projectMissing}
                >
                  <option value="">{projectsLoading ? 'Đang tải công trình...' : 'Chọn công trình'}</option>
                  {projectOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </label>
              {projectMissing ? <p className="mt-1 text-xs font-medium text-rose-600">Chọn công trình trước khi lưu.</p> : null}
              {projectsError ? <InlineQueryError title="Không tải được danh sách công trình" error={projectsError} onRetry={onRetryProjects} /> : null}
            </div>
          )}
        </fieldset>

        <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
          Lưu sẽ thay thế toàn bộ vai trò hiện tại trong phạm vi đang chọn.
        </div>

        <section aria-labelledby="role-selection-title">
          <div className="flex items-center justify-between gap-3">
            <h3 id="role-selection-title" className="text-sm font-semibold text-slate-700">Chọn vai trò</h3>
            <span className="text-xs font-medium text-slate-500">{selected.size} vai trò đã chọn</span>
          </div>

          <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <label className="relative block w-full sm:max-w-sm">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                type="search"
                className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100"
                placeholder="Tìm theo tên hoặc mã vai trò..."
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                disabled={saving || rolesLoading || Boolean(rolesError)}
              />
            </label>
            <div className="inline-flex w-full rounded-md border border-line bg-slate-50 p-1 sm:w-auto" role="group" aria-label="Lọc danh sách vai trò">
              <button
                type="button"
                aria-pressed={roleFilter === 'all'}
                className={`flex-1 rounded px-3 py-1.5 text-xs font-semibold sm:flex-none ${roleFilter === 'all' ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600 hover:text-ink'}`}
                onClick={() => setRoleFilter('all')}
                disabled={saving || rolesLoading || Boolean(rolesError)}
              >
                Tất cả {roles.length}
              </button>
              <button
                type="button"
                aria-pressed={roleFilter === 'selected'}
                className={`flex-1 rounded px-3 py-1.5 text-xs font-semibold sm:flex-none ${roleFilter === 'selected' ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600 hover:text-ink'}`}
                onClick={() => setRoleFilter('selected')}
                disabled={saving || rolesLoading || Boolean(rolesError)}
              >
                Đã chọn {selected.size}
              </button>
            </div>
          </div>

          <div className={`mt-3 overflow-hidden rounded-lg border ${rolesMissing ? 'border-rose-400 ring-1 ring-rose-100' : 'border-line'}`} data-validation-key="role_ids" data-validation-error={rolesMissing ? 'true' : undefined}>
            <div className="consflow-table-viewport max-h-[42vh]">
              <table className="min-w-full border-separate border-spacing-0">
                <thead className="bg-slate-50">
                  <tr>
                    <th className="w-20 border-b border-line px-3 py-2.5 text-center text-xs font-semibold uppercase tracking-wide text-slate-500">Chọn</th>
                    <th className="border-b border-line px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Vai trò</th>
                    <th className="hidden border-b border-line px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 sm:table-cell">Mã kỹ thuật</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line bg-white">
                  {rolesLoading ? (
                    [0, 1, 2, 3, 4].map((item) => (
                      <tr key={item} className="animate-pulse">
                        <td className="px-3 py-3"><div className="mx-auto h-4 w-4 rounded bg-slate-200" /></td>
                        <td className="px-3 py-3"><div className="h-4 w-2/3 rounded bg-slate-200" /></td>
                        <td className="hidden px-3 py-3 sm:table-cell"><div className="h-4 w-3/4 rounded bg-slate-100" /></td>
                      </tr>
                    ))
                  ) : rolesError ? (
                    <tr>
                      <td colSpan={3} className="p-4"><InlineQueryError title="Không tải được danh sách vai trò" error={rolesError} onRetry={onRetryRoles} /></td>
                    </tr>
                  ) : visibleRoles.length ? (
                    visibleRoles.map((role) => {
                      const checked = selected.has(role.id);
                      const inputID = `staff-role-${role.id}`;
                      return (
                        <tr key={role.id} aria-selected={checked} className={checked ? 'bg-blue-50/70' : 'hover:bg-slate-50'}>
                          <td className="px-3 py-3 text-center align-middle">
                            <input
                              id={inputID}
                              className="h-4 w-4 rounded border-line text-blue-600 focus:ring-blue-100"
                              type="checkbox"
                              checked={checked}
                              onChange={() => toggle(role.id)}
                              disabled={saving}
                              aria-label={`Chọn vai trò ${role.display_name}`}
                            />
                          </td>
                          <td className="px-3 py-3 align-middle text-sm">
                            <label htmlFor={inputID} className="block cursor-pointer font-semibold text-ink">{role.display_name}</label>
                            <label htmlFor={inputID} className="mt-1 block cursor-pointer font-mono text-xs text-slate-500 sm:hidden">{role.name}</label>
                          </td>
                          <td className="hidden px-3 py-3 align-middle sm:table-cell">
                            <label htmlFor={inputID} className="cursor-pointer font-mono text-xs text-slate-600">{role.name}</label>
                          </td>
                        </tr>
                      );
                    })
                  ) : (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center">
                        <p className="text-sm font-semibold text-slate-700">
                          {!roles.length ? 'Chưa có vai trò người dùng App để gán' : roleFilter === 'selected' ? 'Chưa có vai trò đã chọn phù hợp' : 'Không tìm thấy vai trò phù hợp'}
                        </p>
                        {roles.length && (query.trim() || roleFilter === 'selected') ? (
                          <button
                            type="button"
                            className="mt-3 text-xs font-semibold text-blue-700 hover:text-blue-800"
                            onClick={() => { setQuery(''); setRoleFilter('all'); }}
                          >
                            Xem tất cả vai trò
                          </button>
                        ) : null}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
          {rolesMissing ? <p className="mt-1 text-xs font-medium text-rose-600">Chọn ít nhất một vai trò trước khi lưu.</p> : null}
        </section>
      </div>
    </Dialog>
  );
}

function InlineQueryError({ title, error, onRetry }: { title: string; error: unknown; onRetry: () => void }) {
  return (
    <div className="flex flex-col gap-3 rounded-md border border-rose-100 bg-rose-50 p-3 text-left text-sm text-rose-700 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex min-w-0 items-start gap-2">
        <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
        <div>
          <p className="font-semibold">{title}</p>
          <p className="mt-1 text-xs">{queryErrorMessage(error)}</p>
        </div>
      </div>
      <button type="button" className="rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={onRetry}>Thử lại</button>
    </div>
  );
}

function normalizeRoleSearch(value: string) {
  return value
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .replace(/[đĐ]/g, 'd')
    .toLocaleLowerCase('vi-VN')
    .trim();
}

function queryErrorMessage(error: unknown) {
  return error instanceof Error && error.message ? error.message : 'Vui lòng thử lại.';
}

function roleLabels(row: StaffRow) {
  return (row.user_roles ?? []).map((role) => `${role.display_name} ${role.name}`);
}

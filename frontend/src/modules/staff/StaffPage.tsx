import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, Plus, Search, ShieldCheck } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { Dialog } from '../../components/Dialog';
import { SimpleForm } from '../_shared/SimpleForm';
import { isAssignableAppRole } from '../../lib/utils/roles';
import { enumOptions } from '../../lib/utils/status';

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
  user_roles?: StaffRole[];
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
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: StaffRow } | null>(null);
  const [roleDialog, setRoleDialog] = useState<StaffRow | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasPermission } = useAuth();
  const canCreate = hasPermission('staff.create');
  const canUpdate = hasPermission('staff.update');
  const canAssign = hasPermission('permission.assign');

  const staff = useQuery({ queryKey: ['staff'], queryFn: () => get<StaffRow[]>('/staff') });
  const roles = useQuery({ queryKey: ['roles'], queryFn: () => get<RoleRow[]>('/roles'), enabled: canAssign });
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<ProjectOption[]>('/projects'), enabled: canAssign });

  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post('/staff', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['staff'] });
      setOpen(false);
      toast.success('Đã tạo nhân sự');
    },
    onError: (error) => toast.error('Không tạo được nhân sự', error),
  });
  const update = useMutation({
    mutationFn: ({ staffId, values }: { staffId: string; values: Record<string, any> }) => put(`/staff/${staffId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['staff'] });
      setDialog(null);
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
    { name: 'full_name', label: 'Họ tên', defaultValue: record?.full_name ?? 'Nhân sự mới', required: true },
    { name: 'email', label: 'Email', defaultValue: record?.email ?? `staff${Date.now().toString().slice(-4)}@consflow.local`, required: true },
    { name: 'phone', label: 'Điện thoại', defaultValue: record?.phone ?? '' },
    { name: 'department', label: 'Phòng ban', defaultValue: record?.department ?? 'Dự án', required: true },
    { name: 'position', label: 'Chức vụ', defaultValue: record?.position ?? 'Nhân viên', required: true },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'active', required: true, options: enumOptions('status', ['active', 'inactive']) },
  ];

  return (
    <>
      <PageHeader
        title="Nhân sự"
        description="Quản lý nhân sự, phòng ban, chức vụ, trạng thái hoạt động và phân công vào công trình theo role."
        actions={canCreate ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Nhân sự</button> : null}
      />
      <DataTable
        rows={staff.data ?? []}
        loading={staff.isLoading}
        error={staff.error}
        onRetry={() => staff.refetch()}
        searchPlaceholder="Tìm nhân sự..."
        pageSize={12}
        defaultSort={{ key: 'full_name' }}
        columns={[
          { key: 'full_name', header: 'Họ tên', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.full_name}</span> },
          { key: 'email', header: 'Email', sortable: true },
          { key: 'department', header: 'Phòng ban', sortable: true },
          { key: 'position', header: 'Chức vụ' },
          { key: 'roles', header: 'Role', filterValue: (row) => roleLabels(row).join(' '), render: (row) => <RoleSummary roles={row.user_roles ?? []} /> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          {
            key: 'actions',
            header: 'Action',
            render: (row) => (
              <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={canUpdate ? () => setDialog({ mode: 'edit', record: row }) : undefined}>
                {canAssign ? (
                  <button
                    title={row.user_id ? 'Gán vai trò' : 'Nhân sự chưa liên kết tài khoản'}
                    aria-label="Gán vai trò"
                    className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
                    onClick={() => setRoleDialog(row)}
                    disabled={!row.user_id}
                  >
                    <ShieldCheck className="h-4 w-4" />
                  </button>
                ) : null}
              </RowActions>
            ),
          },
        ]}
      />
      <FormDrawer open={open} title="Tạo nhân sự" onClose={() => setOpen(false)}>
        <SimpleForm fields={staffFields()} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết nhân sự"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'full_name', label: 'Họ tên' },
          { key: 'email', label: 'Email' },
          { key: 'phone', label: 'Điện thoại' },
          { key: 'department', label: 'Phòng ban' },
          { key: 'position', label: 'Chức vụ' },
          { key: 'roles', label: 'Role', render: (row) => <RoleSummary roles={row.user_roles ?? []} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa nhân sự" onClose={() => setDialog(null)}>
        {dialog?.record ? <SimpleForm submitLabel="Lưu thay đổi" fields={staffFields(dialog.record)} onSubmit={(values) => update.mutateAsync({ staffId: dialog.record.id, values })} /> : null}
      </FormDrawer>
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
      <div className="space-y-5">
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
            <div className="mt-3">
              <label className="block">
                <span className="text-sm font-semibold text-slate-700">Công trình <span className="text-rose-600">*</span></span>
                <select
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

          <div className={`mt-3 overflow-hidden rounded-lg border ${rolesMissing ? 'border-rose-400 ring-1 ring-rose-100' : 'border-line'}`}>
            <div className="max-h-[42vh] overflow-auto">
              <table className="min-w-full border-separate border-spacing-0">
                <thead className="sticky top-0 z-10 bg-slate-50">
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

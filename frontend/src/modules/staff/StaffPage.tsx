import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, ShieldCheck } from 'lucide-react';
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
      toast.success('Đã cập nhật role cho nhân sự');
    },
    onError: (error) => toast.error('Không cập nhật được role', error),
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
                    title={row.user_id ? 'Gán role' : 'Nhân sự chưa liên kết user'}
                    aria-label="Gán role"
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
        loading={roles.isLoading || projects.isLoading}
        saving={assignRoles.isPending}
        onClose={() => setRoleDialog(null)}
        onSave={(roleIds, projectId) => roleDialog?.user_id ? assignRoles.mutateAsync({ userId: roleDialog.user_id, roleIds, projectId }) : Promise.resolve()}
        onWarn={(message) => toast.warn(message)}
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
  loading,
  saving,
  onClose,
  onSave,
  onWarn,
}: {
  staff: StaffRow | null;
  roles: RoleRow[];
  projects: ProjectOption[];
  loading: boolean;
  saving: boolean;
  onClose: () => void;
  onSave: (roleIds: string[], projectId: string) => Promise<unknown>;
  onWarn: (message: string) => void;
}) {
  const [scope, setScope] = useState<'global' | 'project'>('global');
  const [projectID, setProjectID] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const assignableRoleIDs = useMemo(() => new Set(roles.map((role) => role.id)), [roles]);

  useEffect(() => {
    setScope('global');
    setProjectID('');
    setSelected(new Set((staff?.user_roles ?? []).filter((role) => !role.project_id && assignableRoleIDs.has(role.id)).map((role) => role.id)));
  }, [assignableRoleIDs, staff?.id, staff?.user_roles]);

  useEffect(() => {
    const targetProjectID = scope === 'project' ? projectID : '';
    setSelected(new Set((staff?.user_roles ?? []).filter((role) => (role.project_id ?? '') === targetProjectID && assignableRoleIDs.has(role.id)).map((role) => role.id)));
  }, [assignableRoleIDs, projectID, scope, staff?.user_roles]);

  const projectOptions = useMemo(() => projects.map((project) => ({ value: project.id, label: `${project.code ? `${project.code} - ` : ''}${project.name}` })), [projects]);

  function toggle(roleID: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(roleID)) next.delete(roleID);
      else next.add(roleID);
      return next;
    });
  }

  function save() {
    if (scope === 'project' && !projectID) {
      onWarn('Chọn công trình trước khi lưu role theo công trình');
      return;
    }
    if (!selected.size) {
      onWarn('Chọn ít nhất một role cho nhân sự');
      return;
    }
    void onSave(Array.from(selected), scope === 'project' ? projectID : '');
  }

  return (
    <Dialog
      open={Boolean(staff)}
      title={`Gán role${staff ? ` - ${staff.full_name}` : ''}`}
      description="Danh sách role được lưu theo scope đang chọn và thay thế snapshot role hiện tại của scope đó."
      onClose={onClose}
      size="lg"
      footer={
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-medium text-slate-500">{selected.size} role đã chọn</p>
          <div className="flex items-center gap-2">
            <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} disabled={saving}>Huỷ</button>
            <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" onClick={save} disabled={saving || loading}>
              {saving ? 'Đang lưu...' : 'Lưu role'}
            </button>
          </div>
        </div>
      }
    >
      <div className="mb-4 grid gap-3 md:grid-cols-2">
        <label className="block">
          <span className="text-sm font-semibold text-slate-700">Scope</span>
          <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" value={scope} onChange={(event) => setScope(event.target.value as 'global' | 'project')}>
            <option value="global">Toàn hệ thống</option>
            <option value="project">Theo công trình</option>
          </select>
        </label>
        <label className="block">
          <span className="text-sm font-semibold text-slate-700">Công trình</span>
          <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring disabled:bg-slate-100 disabled:text-slate-500" value={projectID} onChange={(event) => setProjectID(event.target.value)} disabled={scope !== 'project'}>
            <option value="">Chọn công trình</option>
            {projectOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
      </div>
      {loading ? (
        <div className="rounded-md border border-line p-4 text-sm text-slate-600">Đang tải role...</div>
      ) : roles.length ? (
        <div className="grid max-h-[44vh] gap-2 overflow-auto pr-1 md:grid-cols-2">
          {roles.map((role) => (
            <label key={role.id} className="flex items-start gap-3 rounded-md border border-line p-3 text-sm hover:bg-slate-50">
              <input className="mt-1 h-4 w-4 rounded border-line text-blue-600 focus:ring-blue-100" type="checkbox" checked={selected.has(role.id)} onChange={() => toggle(role.id)} />
              <span>
                <span className="block font-semibold text-ink">{role.display_name}</span>
                <code className="mt-1 block rounded bg-slate-100 px-2 py-1 text-xs text-slate-600">{role.name}</code>
              </span>
            </label>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-line p-4 text-sm text-slate-600">Chưa có role để gán.</div>
      )}
    </Dialog>
  );
}

function roleLabels(row: StaffRow) {
  return (row.user_roles ?? []).map((role) => `${role.display_name} ${role.name}`);
}

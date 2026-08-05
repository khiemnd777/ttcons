import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, ShieldCheck } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { Dialog } from '../../components/Dialog';
import { appRoleScopeLabel } from '../../lib/utils/roles';
import { SimpleForm } from '../_shared/SimpleForm';

type PermissionRow = Record<string, any> & {
  id: string;
  key: string;
  name: string;
  module: string;
};

type RoleRow = Record<string, any> & {
  id: string;
  name: string;
  display_name: string;
  permission_ids?: string[];
  permission_count?: number;
  external_only?: boolean;
};

export function RolesPage() {
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: RoleRow } | null>(null);
  const [permissionRole, setPermissionRole] = useState<RoleRow | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasPermission } = useAuth();
  const canCreate = hasPermission('role.create');
  const canUpdate = hasPermission('role.update');
  const canAssign = hasPermission('permission.assign');

  const roles = useQuery({ queryKey: ['roles'], queryFn: () => get<RoleRow[]>('/roles') });
  const permissions = useQuery({ queryKey: ['permissions'], queryFn: () => get<PermissionRow[]>('/permissions'), enabled: canAssign });

  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post('/roles', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setOpen(false);
      toast.success('Đã tạo role');
    },
    onError: (error) => toast.error('Không tạo được role', error),
  });
  const update = useMutation({
    mutationFn: ({ roleId, values }: { roleId: string; values: Record<string, any> }) => put(`/roles/${roleId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setDialog(null);
      toast.success('Đã cập nhật role');
    },
    onError: (error) => toast.error('Không cập nhật được role', error),
  });
  const assignPermissions = useMutation({
    mutationFn: ({ roleId, permissionIds }: { roleId: string; permissionIds: string[] }) => post(`/roles/${roleId}/permissions`, { permission_ids: permissionIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setPermissionRole(null);
      toast.success('Đã cập nhật quyền cho role');
    },
    onError: (error) => toast.error('Không cập nhật được quyền', error),
  });

  const roleFields = (record?: RoleRow) => [
    { name: 'name', label: 'Role key', defaultValue: record?.name ?? `role_${Date.now().toString().slice(-4)}`, required: true, disabledWhen: () => Boolean(record?.system_role) },
    { name: 'display_name', label: 'Tên hiển thị', defaultValue: record?.display_name ?? 'Role mới', required: true },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '' },
  ];

  return (
    <>
      <PageHeader
        title="Vai trò"
        description="Role hệ thống và role theo công trình; pipeline node gán theo role thay vì gán cứng người."
        actions={canCreate ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Role</button> : null}
      />
      <DataTable
        rows={roles.data ?? []}
        loading={roles.isLoading}
        error={roles.error}
        onRetry={() => roles.refetch()}
        searchPlaceholder="Tìm role..."
        pageSize={12}
        defaultSort={{ key: 'display_name' }}
        columns={[
          { key: 'display_name', header: 'Vai trò', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.display_name}</span> },
          { key: 'name', header: 'Key', sortable: true },
          { key: 'scope', header: 'Phạm vi', render: (row) => <span className={row.external_only ? 'rounded-md bg-amber-100 px-2 py-1 text-xs font-semibold text-amber-800' : 'text-slate-600'}>{appRoleScopeLabel(row)}</span> },
          { key: 'permission_count', header: 'Quyền', align: 'right', sortable: true, sortValue: (row) => Number(row.permission_count ?? row.permission_ids?.length ?? 0), render: (row) => row.permission_count ?? row.permission_ids?.length ?? 0 },
          { key: 'system_role', header: 'System', render: (row) => row.system_role ? 'Có' : 'Không' },
          {
            key: 'actions',
            header: 'Action',
            render: (row) => (
              <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={canUpdate && !row.external_only ? () => setDialog({ mode: 'edit', record: row }) : undefined}>
                {row.external_only ? <span className="px-1 text-xs font-semibold text-amber-700">Chỉ xem</span> : canAssign ? (
                  <button title="Gán quyền" aria-label="Gán quyền" className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100" onClick={() => setPermissionRole(row)}>
                    <ShieldCheck className="h-4 w-4" />
                  </button>
                ) : null}
              </RowActions>
            ),
          },
        ]}
      />
      <FormDrawer open={open} title="Tạo role" onClose={() => setOpen(false)}>
        <SimpleForm fields={roleFields()} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết role"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'display_name', label: 'Tên hiển thị' },
          { key: 'name', label: 'Key' },
          { key: 'description', label: 'Mô tả' },
          { key: 'scope', label: 'Phạm vi', render: (row) => appRoleScopeLabel(row) },
          { key: 'permission_count', label: 'Số quyền' },
          { key: 'system_role', label: 'System role' },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa role" onClose={() => setDialog(null)}>
        {dialog?.record ? <SimpleForm submitLabel="Lưu thay đổi" fields={roleFields(dialog.record)} onSubmit={(values) => update.mutateAsync({ roleId: dialog.record.id, values })} /> : null}
      </FormDrawer>
      <PermissionMatrixDialog
        role={permissionRole}
        permissions={permissions.data ?? []}
        loading={permissions.isLoading}
        error={permissions.error}
        saving={assignPermissions.isPending}
        onRetry={() => permissions.refetch()}
        onClose={() => setPermissionRole(null)}
        onSave={(permissionIds) => permissionRole ? assignPermissions.mutateAsync({ roleId: permissionRole.id, permissionIds }) : Promise.resolve()}
      />
    </>
  );
}

function PermissionMatrixDialog({
  role,
  permissions,
  loading,
  error,
  saving,
  onRetry,
  onClose,
  onSave,
}: {
  role: RoleRow | null;
  permissions: PermissionRow[];
  loading: boolean;
  error: unknown;
  saving: boolean;
  onRetry: () => void;
  onClose: () => void;
  onSave: (permissionIds: string[]) => Promise<unknown>;
}) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState('');
  const [module, setModule] = useState('all');

  useEffect(() => {
    setSelected(new Set(role?.permission_ids ?? []));
    setQuery('');
    setModule('all');
  }, [role?.id]);

  const modules = useMemo(() => Array.from(new Set(permissions.map((permission) => permission.module))).sort(), [permissions]);
  const visiblePermissions = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return permissions.filter((permission) => {
      const moduleMatch = module === 'all' || permission.module === module;
      const queryMatch = !normalized || `${permission.key} ${permission.name} ${permission.module}`.toLowerCase().includes(normalized);
      return moduleMatch && queryMatch;
    });
  }, [module, permissions, query]);

  function toggle(permissionID: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(permissionID)) next.delete(permissionID);
      else next.add(permissionID);
      return next;
    });
  }

  return (
    <Dialog
      open={Boolean(role)}
      title={`Gán quyền${role ? ` - ${role.display_name}` : ''}`}
      description="Danh sách này sẽ thay thế snapshot quyền hiện tại của role sau khi lưu."
      onClose={onClose}
      size="xl"
      footer={
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-medium text-slate-500">{selected.size} quyền đã chọn</p>
          <div className="flex items-center gap-2">
            <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} disabled={saving}>Huỷ</button>
            <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" onClick={() => onSave(Array.from(selected))} disabled={saving}>
              {saving ? 'Đang lưu...' : 'Lưu quyền'}
            </button>
          </div>
        </div>
      }
    >
      <div className="mb-4 grid gap-3 md:grid-cols-[1fr_220px]">
        <input className="rounded-md border border-line px-3 py-2 text-sm focus-ring" placeholder="Tìm permission..." value={query} onChange={(event) => setQuery(event.target.value)} />
        <select className="rounded-md border border-line px-3 py-2 text-sm focus-ring" value={module} onChange={(event) => setModule(event.target.value)}>
          <option value="all">Tất cả module</option>
          {modules.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
      </div>
      {loading ? (
        <div className="rounded-md border border-line p-4 text-sm text-slate-600">Đang tải quyền...</div>
      ) : error ? (
        <div className="flex items-center justify-between gap-3 rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700">
          <span>Không tải được quyền</span>
          <button className="rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold" onClick={onRetry}>Thử lại</button>
        </div>
      ) : visiblePermissions.length ? (
        <div className="grid max-h-[52vh] gap-2 overflow-auto pr-1 md:grid-cols-2">
          {visiblePermissions.map((permission) => (
            <label key={permission.id} className="flex items-start gap-3 rounded-md border border-line p-3 text-sm hover:bg-slate-50">
              <input className="mt-1 h-4 w-4 rounded border-line text-blue-600 focus:ring-blue-100" type="checkbox" checked={selected.has(permission.id)} onChange={() => toggle(permission.id)} />
              <span className="min-w-0">
                <span className="block font-semibold text-ink">{permission.name}</span>
                <code className="mt-1 block truncate rounded bg-slate-100 px-2 py-1 text-xs text-slate-600">{permission.key}</code>
                <span className="mt-1 block text-xs font-medium uppercase text-slate-400">{permission.module}</span>
              </span>
            </label>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-line p-4 text-sm text-slate-600">Không có quyền phù hợp.</div>
      )}
    </Dialog>
  );
}

import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  AlertCircle,
  Eye,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  UserMinus,
  Users,
} from 'lucide-react';
import clsx from 'clsx';
import { ApiError, del, post, put } from '../../lib/api/client';
import type {
  ProjectMember,
  ProjectMemberAccessRole,
  ProjectMembersResponse,
  ProjectMemberStaffOption,
} from '../../lib/types/domain';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { RowActions } from '../../components/RowActions';
import { useToast } from '../../components/ToastProvider';
import { IdentityReferenceLabel } from '../../components/IdentityReferenceLabel';

type MemberDialog =
  | { mode: 'view'; member: ProjectMember }
  | { mode: 'edit'; member: ProjectMember }
  | { mode: 'add' }
  | null;

type MemberMutationInput = {
  memberId?: string;
  values: {
    staff_id: string;
    role_ids: string[];
    project_role_ids: string[];
    notes: string;
  };
};

export function ProjectMembersPanel({
  projectId,
  data,
  loading,
  error,
  onRetry,
}: {
  projectId: string;
  data?: ProjectMembersResponse;
  loading: boolean;
  error: unknown;
  onRetry: () => void;
}) {
  const [query, setQuery] = useState('');
  const [department, setDepartment] = useState('');
  const [role, setRole] = useState('');
  const [status, setStatus] = useState<'active' | 'inactive' | 'all'>('active');
  const [dialog, setDialog] = useState<MemberDialog>(null);
  const [deactivateTarget, setDeactivateTarget] = useState<ProjectMember | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const canManage = Boolean(data?.capabilities.can_manage);

  const save = useMutation({
    mutationFn: ({ memberId, values }: MemberMutationInput) => memberId
      ? put<ProjectMember>(`/projects/${projectId}/members/${memberId}`, values)
      : post<ProjectMember>(`/projects/${projectId}/members`, values),
    onSuccess: (_, input) => {
      queryClient.invalidateQueries({ queryKey: ['project-members', projectId] });
      setDialog(null);
      toast.success(input.memberId ? 'Đã cập nhật thành viên' : 'Đã thêm thành viên công trình');
    },
    onError: (mutationError) => {
      if (mutationError instanceof ApiError && mutationError.code === 'VALIDATION_ERROR') {
        toast.warn({ title: 'Chưa thể lưu thành viên', description: mutationError.message });
        return;
      }
      toast.error('Không lưu được thành viên', mutationError);
    },
  });
  const deactivate = useMutation({
    mutationFn: (member: ProjectMember) => del<ProjectMember>(`/projects/${projectId}/members/${member.id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-members', projectId] });
      setDeactivateTarget(null);
      toast.success('Đã ngừng tham gia thành viên');
    },
    onError: (mutationError) => {
      if (mutationError instanceof ApiError && mutationError.code === 'VALIDATION_ERROR') {
        toast.warn({ title: 'Chưa thể ngừng tham gia', description: mutationError.message });
        return;
      }
      toast.error('Không cập nhật được trạng thái thành viên', mutationError);
    },
  });

  const items = data?.items ?? [];
  const departments = useMemo(
    () => Array.from(new Set(items.map((item) => item.department).filter(Boolean) as string[])).sort(localeCompareVI),
    [items],
  );
  const roles = useMemo(
    () => Array.from(new Set(items.flatMap((item) => item.role_ids ?? []))).sort(localeCompareVI),
    [items],
  );
  const filtered = useMemo(() => {
    const normalizedQuery = normalizeSearch(query);
    return items.filter((member) => {
      if (status === 'active' && !member.active) return false;
      if (status === 'inactive' && member.active) return false;
      if (department && member.department !== department) return false;
      if (role && !member.role_ids.includes(role)) return false;
      if (!normalizedQuery) return true;
      return normalizeSearch([
        member.full_name,
        member.email,
        member.department,
        member.position,
        ...member.role_ids,
        ...member.global_roles.map((item) => item.display_name),
        ...member.project_roles.map((item) => item.display_name),
      ].filter(Boolean).join(' ')).includes(normalizedQuery);
    });
  }, [department, items, query, role, status]);

  if (loading) return <ProjectMembersLoading />;
  if (error) {
    return (
      <section className="rounded-lg border border-rose-200 bg-white p-5 shadow-panel">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3">
            <AlertCircle className="mt-0.5 h-5 w-5 text-rose-600" />
            <div>
              <h2 className="font-semibold text-rose-800">Không tải được danh sách thành viên</h2>
              <p className="mt-1 text-sm text-rose-700">{error instanceof Error ? error.message : 'Vui lòng thử lại.'}</p>
            </div>
          </div>
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50" onClick={onRetry}>
            <RefreshCw className="h-4 w-4" />
            Thử lại
          </button>
        </div>
      </section>
    );
  }

  const hasFilters = Boolean(query.trim() || department || role || status !== 'active');

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <Users className="h-5 w-5 text-blue-700" />
            <h2 className="font-semibold text-ink">Thành viên công trình</h2>
          </div>
          <p className="mt-1 text-sm text-slate-600">
            {data?.summary.active ?? 0} đang hoạt động · {data?.summary.departments ?? 0} phòng ban · {data?.summary.roles ?? 0} vai trò
          </p>
        </div>
        {canManage ? (
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setDialog({ mode: 'add' })}>
            <Plus className="h-4 w-4" />
            Thêm thành viên
          </button>
        ) : null}
      </div>

      <div className="grid gap-2 rounded-lg border border-line bg-white p-3 shadow-panel sm:grid-cols-2 xl:grid-cols-[minmax(260px,1fr)_200px_200px_180px]">
        <label className="relative block">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm focus-ring" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm tên, phòng ban, vai trò" />
        </label>
        <select className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring" value={department} onChange={(event) => setDepartment(event.target.value)}>
          <option value="">Tất cả phòng ban</option>
          {departments.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
        <select className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring" value={role} onChange={(event) => setRole(event.target.value)}>
          <option value="">Tất cả vai trò</option>
          {roles.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
        <select className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring" value={status} onChange={(event) => setStatus(event.target.value as typeof status)}>
          <option value="active">Đang hoạt động</option>
          {data?.capabilities.can_view_inactive ? <option value="inactive">Đã ngừng tham gia</option> : null}
          {data?.capabilities.can_view_inactive ? <option value="all">Tất cả trạng thái</option> : null}
        </select>
      </div>

      {!items.length ? (
        <EmptyState
          title="Công trình chưa có thành viên"
          action={canManage ? <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setDialog({ mode: 'add' })}>Thêm thành viên đầu tiên</button> : undefined}
        />
      ) : !filtered.length ? (
        <EmptyState
          title="Không tìm thấy thành viên phù hợp"
          action={hasFilters ? <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => { setQuery(''); setDepartment(''); setRole(''); setStatus('active'); }}>Xóa bộ lọc</button> : undefined}
        />
      ) : (
        <>
          <div className="hidden md:block">
            <DataTable
              rows={filtered}
              pageSize={20}
              getRowKey={(member) => member.id}
              columns={[
                { key: 'full_name', header: 'Nhân sự', sortable: true, render: (member) => <MemberIdentity member={member} /> },
                { key: 'department', header: 'Phòng ban / Chức vụ', sortable: true, render: (member) => <DepartmentPosition member={member} /> },
                { key: 'role_ids', header: 'Vai trò công trình', render: (member) => <RoleChips roles={member.role_ids} /> },
                { key: 'access_roles', header: 'Quyền truy cập', render: (member) => <AccessRoleSummary member={member} /> },
                { key: 'active', header: 'Trạng thái', sortable: true, render: (member) => <MemberStatus active={member.active} /> },
                { key: 'actions', header: '', align: 'right', render: (member) => <MemberActions member={member} canManage={canManage} onView={() => setDialog({ mode: 'view', member })} onEdit={() => setDialog({ mode: 'edit', member })} onDeactivate={() => setDeactivateTarget(member)} /> },
              ]}
            />
          </div>
          <div className="space-y-3 md:hidden">
            {filtered.map((member) => (
              <article key={member.id} className={clsx('rounded-lg border border-line bg-white p-4 shadow-panel', !member.active && 'bg-slate-50 opacity-80')}>
                <div className="flex items-start justify-between gap-3">
                  <MemberIdentity member={member} />
                  <MemberActions member={member} canManage={canManage} onView={() => setDialog({ mode: 'view', member })} onEdit={() => setDialog({ mode: 'edit', member })} onDeactivate={() => setDeactivateTarget(member)} />
                </div>
                <div className="mt-3 grid gap-3 border-t border-line pt-3 text-sm">
                  <DepartmentPosition member={member} />
                  <RoleChips roles={member.role_ids} />
                  <div className="flex items-center justify-between gap-3"><AccessRoleSummary member={member} /><MemberStatus active={member.active} /></div>
                </div>
              </article>
            ))}
          </div>
        </>
      )}

      <ProjectMemberDetailDialog member={dialog?.mode === 'view' ? dialog.member : null} onClose={() => setDialog(null)} />
      <ProjectMemberFormDialog
        open={dialog?.mode === 'add' || dialog?.mode === 'edit'}
        member={dialog?.mode === 'edit' ? dialog.member : null}
        data={data}
        saving={save.isPending}
        onClose={() => setDialog(null)}
        onSave={(values) => save.mutateAsync({ memberId: dialog?.mode === 'edit' ? dialog.member.id : undefined, values })}
        onWarn={(description) => toast.warn({ title: 'Kiểm tra thông tin thành viên', description })}
      />
      <Dialog
        open={Boolean(deactivateTarget)}
        title="Ngừng tham gia công trình"
        description={deactivateTarget ? `${deactivateTarget.full_name} sẽ mất các role được cấp riêng trong công trình. Lịch sử công việc và audit vẫn được giữ lại.` : undefined}
        onClose={() => setDeactivateTarget(null)}
        size="sm"
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white" onClick={() => setDeactivateTarget(null)}>Hủy</button>
            <button type="button" className="rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:opacity-60" disabled={deactivate.isPending} onClick={() => deactivateTarget && deactivate.mutate(deactivateTarget)}>{deactivate.isPending ? 'Đang xử lý...' : 'Ngừng tham gia'}</button>
          </div>
        )}
      >
        <p className="text-sm leading-6 text-slate-700">Thao tác này không xóa thành viên hoặc dữ liệu lịch sử. Có thể kích hoạt lại bằng luồng “Thêm thành viên”.</p>
      </Dialog>
    </section>
  );
}

function MemberActions({ member, canManage, onView, onEdit, onDeactivate }: { member: ProjectMember; canManage: boolean; onView: () => void; onEdit: () => void; onDeactivate: () => void }) {
  return (
    <RowActions onView={onView} onEdit={canManage && member.active ? onEdit : undefined}>
      {canManage && member.active && !member.is_project_manager ? (
        <button type="button" title="Ngừng tham gia" aria-label="Ngừng tham gia" className="rounded-md border border-rose-200 p-1.5 text-rose-600 hover:bg-rose-50" onClick={onDeactivate}>
          <UserMinus className="h-4 w-4" />
        </button>
      ) : null}
    </RowActions>
  );
}

function MemberIdentity({ member }: { member: ProjectMember }) {
  return (
    <div className="flex min-w-0 items-center gap-3">
      <span className="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-blue-50 text-xs font-bold text-blue-700">{initials(member.full_name)}</span>
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5">
          <p className="truncate font-semibold text-ink"><IdentityReferenceLabel name={member.full_name} deleted={member.account_deleted} /></p>
          {member.is_project_manager ? <span className="rounded bg-blue-50 px-1.5 py-0.5 text-[10px] font-bold uppercase text-blue-700">PM chính</span> : null}
        </div>
        {member.email ? <p className="truncate text-xs text-slate-500">{member.email}</p> : null}
      </div>
    </div>
  );
}

function DepartmentPosition({ member }: { member: ProjectMember }) {
  return (
    <div>
      <p className="font-medium text-slate-700">{member.department || 'Chưa xác định'}</p>
      <p className="mt-0.5 text-xs text-slate-500">{member.position || 'Chưa có chức vụ'}</p>
    </div>
  );
}

function RoleChips({ roles }: { roles: string[] }) {
  if (!roles.length) return <span className="text-sm text-slate-400">Chưa gán</span>;
  return <div className="flex flex-wrap gap-1">{roles.map((role) => <span key={role} className="rounded bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-700">{role}</span>)}</div>;
}

function AccessRoleSummary({ member }: { member: ProjectMember }) {
  const roles = member.project_roles.length ? member.project_roles : member.global_roles;
  if (!roles.length) return <span className="text-sm text-amber-700">Chưa có role truy cập</span>;
  return (
    <div className="min-w-0">
      <div className="flex items-center gap-1.5 text-sm font-medium text-slate-700"><ShieldCheck className="h-4 w-4 shrink-0 text-emerald-600" /><span className="truncate">{roles.map((role) => role.display_name).join(', ')}</span></div>
      <p className="mt-0.5 text-xs text-slate-500">{member.project_roles.length ? 'Theo công trình' : 'Theo role toàn hệ thống'}</p>
    </div>
  );
}

function MemberStatus({ active }: { active: boolean }) {
  return <span className={clsx('inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold', active ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-200 text-slate-600')}>{active ? 'Đang hoạt động' : 'Đã ngừng tham gia'}</span>;
}

function ProjectMemberDetailDialog({ member, onClose }: { member: ProjectMember | null; onClose: () => void }) {
  return (
    <Dialog open={Boolean(member)} title="Chi tiết thành viên" description={member?.full_name} onClose={onClose} size="md">
      {member ? (
        <div className="space-y-5">
          <div className="grid gap-4 rounded-md border border-line bg-slate-50 p-4 sm:grid-cols-2">
            <Detail label="Phòng ban" value={member.department} />
            <Detail label="Chức vụ" value={member.position} />
            <Detail label="Email" value={member.email} />
            <Detail label="Điện thoại" value={member.phone} />
          </div>
          <DetailSection title="Vai trò vận hành"><RoleChips roles={member.role_ids} /></DetailSection>
          <DetailSection title="Role riêng trong công trình"><AccessRoleChips roles={member.project_roles} empty="Không có role riêng theo công trình" /></DetailSection>
          <DetailSection title="Role toàn hệ thống"><AccessRoleChips roles={member.global_roles} empty="Không có role toàn hệ thống" /></DetailSection>
          <DetailSection title="Ghi chú phân công"><p className="text-sm leading-6 text-slate-700">{member.notes || 'Chưa có ghi chú'}</p></DetailSection>
        </div>
      ) : null}
    </Dialog>
  );
}

function ProjectMemberFormDialog({ open, member, data, saving, onClose, onSave, onWarn }: {
  open: boolean;
  member: ProjectMember | null;
  data?: ProjectMembersResponse;
  saving: boolean;
  onClose: () => void;
  onSave: (values: MemberMutationInput['values']) => Promise<unknown>;
  onWarn: (message: string) => void;
}) {
  const [staffID, setStaffID] = useState('');
  const [roleIDs, setRoleIDs] = useState<string[]>([]);
  const [projectRoleIDs, setProjectRoleIDs] = useState<string[]>([]);
  const [notes, setNotes] = useState('');
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!open) return;
    setStaffID(member?.staff_id ?? '');
    setRoleIDs(member?.role_ids ?? []);
    setProjectRoleIDs(member?.project_roles.map((role) => role.id) ?? []);
    setNotes(member?.notes ?? '');
    setSubmitted(false);
  }, [member, open]);

  const staffOptions = data?.options.staff ?? [];
  const accessRoles = data?.options.access_roles ?? [];
  const roleLabels = data?.options.role_labels ?? [];
  const selectedStaff = staffOptions.find((option) => option.id === staffID);
  const globalRoles = member?.staff_id === staffID ? member.global_roles : selectedStaff?.global_roles ?? [];
  const accessReady = globalRoles.some((role) => role.grants_project_view)
    || projectRoleIDs.some((roleID) => accessRoles.find((role) => role.id === roleID)?.grants_project_view);

  function selectStaff(nextStaffID: string) {
    setStaffID(nextStaffID);
    const previous = data?.items.find((item) => item.staff_id === nextStaffID && !item.active);
    if (previous) {
      setRoleIDs(previous.role_ids);
      setProjectRoleIDs(previous.project_roles.map((item) => item.id));
      setNotes(previous.notes ?? '');
      return;
    }
    const option = staffOptions.find((item) => item.id === nextStaffID);
    setRoleIDs(option?.global_roles.length ? [option.global_roles[0].display_name] : []);
    setProjectRoleIDs([]);
    setNotes('');
  }

  function submit() {
    setSubmitted(true);
    const problems: string[] = [];
    if (!staffID) problems.push('chọn nhân sự');
    if (!roleIDs.length) problems.push('chọn ít nhất một vai trò vận hành');
    if (!accessReady) problems.push('chọn role có quyền project.view hoặc dùng nhân sự đã có role toàn hệ thống phù hợp');
    if (problems.length) {
      onWarn(`Cần ${problems.join('; ')}.`);
      return;
    }
    void onSave({ staff_id: staffID, role_ids: roleIDs, project_role_ids: projectRoleIDs, notes });
  }

  return (
    <Dialog
      open={open}
      title={member ? 'Chỉnh sửa thành viên' : 'Thêm thành viên công trình'}
      description="Vai trò vận hành và role phân quyền là hai lớp riêng biệt. Quyền thực tế được cấp theo role."
      onClose={onClose}
      size="lg"
      footer={(
        <div className="flex justify-end gap-2">
          <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white" onClick={onClose}>Hủy</button>
          <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={saving} onClick={submit}>{saving ? 'Đang lưu...' : member ? 'Lưu thay đổi' : 'Thêm thành viên'}</button>
        </div>
      )}
    >
      <div className="space-y-5">
        <Field label="Nhân sự" required invalid={submitted && !staffID}>
          <select className={fieldClass(submitted && !staffID)} value={staffID} disabled={Boolean(member)} onChange={(event) => selectStaff(event.target.value)}>
            <option value="">Chọn nhân sự đang hoạt động</option>
            {staffOptions.map((option) => (
              <option key={option.id} value={option.id} disabled={Boolean(option.member_active && option.id !== member?.staff_id)}>
                {option.full_name}{option.department ? ` · ${option.department}` : ''}{option.member_active && option.id !== member?.staff_id ? ' · Đã tham gia' : option.member_id ? ' · Có thể kích hoạt lại' : ''}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Vai trò vận hành tại công trình" required invalid={submitted && !roleIDs.length}>
          <div className={clsx('grid gap-2 rounded-md border p-3 sm:grid-cols-2 lg:grid-cols-3', submitted && !roleIDs.length ? 'border-rose-300 bg-rose-50/40' : 'border-line')}>
            {roleLabels.map((label) => <CheckOption key={label} label={label} checked={roleIDs.includes(label)} onChange={() => setRoleIDs(toggleValue(roleIDs, label))} />)}
          </div>
        </Field>

        <Field label="Role phân quyền riêng trong công trình" invalid={submitted && !accessReady}>
          <div className={clsx('grid gap-2 rounded-md border p-3 sm:grid-cols-2', submitted && !accessReady ? 'border-rose-300 bg-rose-50/40' : 'border-line')}>
            {accessRoles.map((accessRole) => <CheckOption key={accessRole.id} label={accessRole.display_name} description={accessRole.description} checked={projectRoleIDs.includes(accessRole.id)} onChange={() => setProjectRoleIDs(toggleValue(projectRoleIDs, accessRole.id))} />)}
          </div>
          {globalRoles.length ? (
            <div className="mt-2 rounded-md bg-emerald-50 px-3 py-2 text-xs text-emerald-800">
              Role toàn hệ thống hiện có: {globalRoles.map((role) => role.display_name).join(', ')}. Các role này không bị thay đổi tại đây.
            </div>
          ) : <p className="mt-2 text-xs text-slate-500">Nếu nhân sự chưa có role toàn hệ thống cấp `project.view`, phải chọn ít nhất một role phù hợp ở trên.</p>}
        </Field>

        <Field label="Ghi chú phân công">
          <textarea className="min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" maxLength={1000} value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="Phạm vi phụ trách hoặc lưu ý bàn giao" />
          <p className="mt-1 text-right text-xs text-slate-400">{notes.length}/1000</p>
        </Field>
      </div>
    </Dialog>
  );
}

function Field({ label, required, invalid, children }: { label: string; required?: boolean; invalid?: boolean; children: React.ReactNode }) {
  return <div><p className={clsx('mb-2 text-sm font-semibold', invalid ? 'text-rose-700' : 'text-slate-700')}>{label}{required ? <span className="ml-1 text-rose-600">*</span> : null}</p>{children}</div>;
}

function CheckOption({ label, description, checked, onChange }: { label: string; description?: string; checked: boolean; onChange: () => void }) {
  return (
    <label className={clsx('flex cursor-pointer items-start gap-2 rounded-md border px-3 py-2', checked ? 'border-blue-200 bg-blue-50' : 'border-line bg-white hover:bg-slate-50')}>
      <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600" checked={checked} onChange={onChange} />
      <span className="min-w-0"><span className="block text-sm font-semibold text-slate-700">{label}</span>{description ? <span className="mt-0.5 block text-xs leading-4 text-slate-500">{description}</span> : null}</span>
    </label>
  );
}

function AccessRoleChips({ roles, empty }: { roles: ProjectMemberAccessRole[]; empty: string }) {
  if (!roles.length) return <p className="text-sm text-slate-500">{empty}</p>;
  return <div className="flex flex-wrap gap-2">{roles.map((role) => <span key={`${role.scope}-${role.id}`} className="rounded-md border border-emerald-100 bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-800">{role.display_name}</span>)}</div>;
}

function Detail({ label, value }: { label: string; value?: string }) {
  return <div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><p className="mt-1 text-sm font-medium text-slate-700">{value || '-'}</p></div>;
}

function DetailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return <section><h3 className="mb-2 text-sm font-semibold text-ink">{title}</h3>{children}</section>;
}

function ProjectMembersLoading() {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between"><div className="h-10 w-64 animate-pulse rounded bg-slate-200" /><div className="h-9 w-36 animate-pulse rounded bg-slate-200" /></div>
      <div className="h-14 animate-pulse rounded-lg border border-line bg-white" />
      <div className="space-y-px overflow-hidden rounded-lg border border-line bg-line">{Array.from({ length: 5 }).map((_, index) => <div key={index} className="h-16 animate-pulse bg-white" />)}</div>
    </div>
  );
}

function fieldClass(invalid: boolean) {
  return clsx('w-full rounded-md border bg-white px-3 py-2 text-sm focus-ring disabled:bg-slate-100', invalid ? 'border-rose-300 bg-rose-50/30' : 'border-line');
}

function toggleValue(values: string[], value: string) {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
}

function initials(value: string) {
  return value.trim().split(/\s+/).slice(-2).map((part) => part[0]?.toUpperCase()).join('') || '?';
}

function normalizeSearch(value: string) {
  return value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().trim();
}

function localeCompareVI(left: string, right: string) {
  return left.localeCompare(right, 'vi');
}

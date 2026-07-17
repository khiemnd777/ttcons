import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ChevronDown,
  ChevronRight,
  Eye,
  Folder,
  FolderOpen,
  Pencil,
  Plus,
  Search,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { useAuth } from '../../lib/auth/AuthContext';
import { SimpleForm } from '../_shared/SimpleForm';
import type { Partner, Project } from '../../lib/types/domain';
import { partnerOptions } from '../core-data/partnerUtils';
import {
  projectWorkspaceGroupPath,
  visibleProjectWorkspaceGroups,
  type ProjectWorkspaceGroup,
} from './projectWorkspaceNavigation';

export function ProjectsPage() {
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: Project } | null>(null);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [expandedProjectIds, setExpandedProjectIds] = useState<Set<string>>(new Set());
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Project[]>('/projects') });
  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<Record<string, any>[]>('/pipeline-templates') });
  const partners = useQuery({ queryKey: ['partners', 'client', 'active'], queryFn: () => get<Partner[]>('/partners?status=active&type=client') });
  const createProject = useMutation({
    mutationFn: (values: Record<string, any>) => post<Project>('/projects', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setOpen(false);
      toast.success('Đã tạo công trình');
    },
    onError: (error) => toast.error('Không tạo được công trình', error),
  });
  const updateProject = useMutation({
    mutationFn: ({ projectId, values }: { projectId: string; values: Record<string, any> }) => put<Project>(`/projects/${projectId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setDialog(null);
      toast.success('Đã cập nhật công trình');
    },
    onError: (error) => toast.error('Không cập nhật được công trình', error),
  });

  const projectFields = (record?: Project) => [
    { name: 'name', label: 'Tên công trình', defaultValue: record?.name ?? 'Công trình mới', required: true },
    { name: 'code', label: 'Mã công trình', defaultValue: record?.code ?? `PRJ-${Date.now().toString().slice(-5)}`, required: true },
    {
      name: 'partner_id',
      label: 'Chủ đầu tư catalog',
      type: 'select' as const,
      defaultValue: record?.partner_id ?? '',
      options: partnerOptions(partners.data ?? [], ['client']),
      helperText: 'Khi chọn catalog, backend tự snapshot tên chủ đầu tư vào công trình.',
    },
    {
      name: 'client_name',
      label: 'Khách hàng / Chủ đầu tư',
      defaultValue: record?.client_name ?? 'Khách hàng mới',
      required: true,
      hiddenWhen: (values: Record<string, any>) => Boolean(values.partner_id),
    },
    { name: 'address', label: 'Địa chỉ', defaultValue: record?.address ?? '' },
    { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date' as const, defaultValue: record?.planned_start_date?.slice?.(0, 10) ?? '' },
    { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date' as const, defaultValue: record?.planned_end_date?.slice?.(0, 10) ?? '' },
    { name: 'estimated_budget', label: 'Ngân sách dự kiến', type: 'number' as const, defaultValue: record?.estimated_budget ?? 300000000, min: 0 },
    { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number' as const, defaultValue: record?.contract_amount ?? 450000000, min: 0 },
    {
      name: 'pipeline_template_id',
      label: 'Pipeline template',
      type: 'select' as const,
      defaultValue: record?.pipeline_template_id ?? '',
      options: (templates.data ?? [])
        .filter((template) => template.status === 'published')
        .map((template) => ({ label: `${template.name} · revision ${template.revision_no ?? template.version}`, value: template.id })),
    },
  ];

  const visibleFolders = useMemo(
    () => visibleProjectWorkspaceGroups(hasAnyPermission),
    [hasAnyPermission],
  );
  const statusOptions = useMemo(() => {
    const values = new Set((projects.data ?? []).map((project) => project.status).filter(Boolean));
    return Array.from(values).sort();
  }, [projects.data]);
  const filteredProjects = useMemo(() => {
    const query = search.trim().toLowerCase();
    return (projects.data ?? []).filter((project) => {
      const matchesStatus = statusFilter ? project.status === statusFilter : true;
      const haystack = [project.name, project.code, project.client_name, project.pm_name].filter(Boolean).join(' ').toLowerCase();
      return matchesStatus && (!query || haystack.includes(query));
    });
  }, [projects.data, search, statusFilter]);

  useEffect(() => {
    if (!projects.data?.length) return;
    setExpandedProjectIds((current) => {
      if (current.size) return current;
      return new Set([projects.data[0].id]);
    });
  }, [projects.data]);

  function toggleProject(projectId: string) {
    setExpandedProjectIds((current) => {
      const next = new Set(current);
      if (next.has(projectId)) next.delete(projectId);
      else next.add(projectId);
      return next;
    });
  }

  return (
    <>
      <PageHeader
        title="Công trình"
        description="Chọn công trình trước, sau đó mở cây nghiệp vụ bên trong công trình."
        actions={
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" />
            Tạo công trình
          </button>
        }
      />

      <section className="rounded-lg border border-line bg-white shadow-panel">
        <div className="border-b border-line p-4">
          <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm focus-ring"
                value={search}
                placeholder="Tìm công trình, mã, chủ đầu tư"
                onChange={(event) => setSearch(event.target.value)}
              />
            </label>
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
            >
              <option value="">Tất cả trạng thái</option>
              {statusOptions.map((status) => (
                <option key={status} value={status}>{status}</option>
              ))}
            </select>
          </div>
        </div>

        <ProjectExplorerHeader />

        {projects.isLoading ? (
          <div className="p-8">
            <LoadingSpinner />
          </div>
        ) : null}

        {projects.error ? (
          <div className="p-5">
            <div className="rounded-md border border-rose-200 bg-rose-50 p-4">
              <p className="text-sm font-semibold text-rose-800">Không tải được danh sách công trình</p>
              <p className="mt-1 text-sm text-rose-700">{projects.error instanceof Error ? projects.error.message : 'Vui lòng thử lại.'}</p>
              <button className="mt-3 rounded-md bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-100" onClick={() => projects.refetch()}>
                Tải lại
              </button>
            </div>
          </div>
        ) : null}

        {!projects.isLoading && !projects.error && !filteredProjects.length ? (
          <div className="p-6">
            <EmptyState title={projects.data?.length ? 'Không tìm thấy công trình phù hợp' : 'Chưa có công trình'} />
          </div>
        ) : null}

        {!projects.isLoading && !projects.error && filteredProjects.length ? (
          <div className="divide-y divide-line">
            {filteredProjects.map((project) => {
              const expanded = expandedProjectIds.has(project.id);
              return (
                <div key={project.id}>
                  <ProjectExplorerRow
                    project={project}
                    expanded={expanded}
                    onToggle={() => toggleProject(project.id)}
                    onView={() => setDialog({ mode: 'view', record: project })}
                    onEdit={() => setDialog({ mode: 'edit', record: project })}
                  />
                  {expanded ? (
                    <div className="border-t border-slate-100 bg-slate-50/70 py-2">
                      {visibleFolders.map((folder) => (
                        <ProjectFolderRow key={folder.key} project={project} folder={folder} />
                      ))}
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        ) : null}
      </section>

      <FormDrawer open={open} title="Tạo công trình mới" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Tạo công trình"
          fields={projectFields()}
          onSubmit={(values) => createProject.mutateAsync(values)}
        />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết công trình"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'name', label: 'Tên công trình' },
          { key: 'code', label: 'Mã' },
          { key: 'client_name', label: 'Chủ đầu tư' },
          { key: 'partner_name', label: 'Đối tác catalog' },
          { key: 'partner_tax_code', label: 'MST đối tác' },
          { key: 'address', label: 'Địa chỉ' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'estimated_budget', label: 'Ngân sách', render: (row) => <CurrencyText value={row.estimated_budget} /> },
          { key: 'contract_amount', label: 'Hợp đồng', render: (row) => <CurrencyText value={row.contract_amount} /> },
          { key: 'progress_percent', label: 'Tiến độ', render: (row) => `${row.progress_percent ?? 0}%` },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa công trình" onClose={() => setDialog(null)}>
        {dialog?.record ? (
          <SimpleForm
            submitLabel="Lưu thay đổi"
            fields={projectFields(dialog.record)}
            onSubmit={(values) => updateProject.mutateAsync({
              projectId: dialog.record.id,
              values: { ...values, partner_id: values.partner_id || null, pipeline_template_id: values.pipeline_template_id || null },
            })}
          />
        ) : null}
      </FormDrawer>
    </>
  );
}

function ProjectExplorerHeader() {
  return (
    <div className="hidden grid-cols-[minmax(0,1.6fr)_160px_150px_120px_140px] gap-4 border-b border-line bg-slate-50 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-500 lg:grid">
      <span>Cây công trình</span>
      <span>Trạng thái</span>
      <span>Phụ trách</span>
      <span>Tiến độ</span>
      <span className="text-right">Mở</span>
    </div>
  );
}

function ProjectExplorerRow({
  project,
  expanded,
  onToggle,
  onView,
  onEdit,
}: {
  project: Project;
  expanded: boolean;
  onToggle: () => void;
  onView: () => void;
  onEdit: () => void;
}) {
  return (
    <div className="grid gap-3 px-4 py-3 lg:grid-cols-[minmax(0,1.6fr)_160px_150px_120px_140px] lg:items-center">
      <div className="flex min-w-0 items-start gap-3">
        <button
          type="button"
          className="mt-0.5 rounded p-1 text-slate-500 hover:bg-slate-100 hover:text-blue-700 focus-ring"
          aria-label={expanded ? 'Thu gọn công trình' : 'Mở rộng công trình'}
          onClick={onToggle}
        >
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </button>
        <FolderOpen className="mt-1 h-4 w-4 shrink-0 text-blue-700" />
        <div className="min-w-0">
          <Link to={`/projects/${project.id}`} className="block truncate text-sm font-semibold text-ink hover:text-blue-700">
            {project.code} · {project.name}
          </Link>
          <p className="mt-1 truncate text-xs text-slate-500">
            {project.client_name || project.partner_name || 'Chưa có chủ đầu tư'}
          </p>
        </div>
      </div>

      <div><StatusBadge status={project.status} /></div>
      <div className="text-sm text-slate-700">{project.pm_name ?? '-'}</div>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <div className="h-1.5 min-w-16 flex-1 overflow-hidden rounded-full bg-slate-100">
            <div className="h-full rounded-full bg-blue-600" style={{ width: `${Math.min(100, Math.max(0, Number(project.progress_percent ?? 0)))}%` }} />
          </div>
          <span className="text-xs font-semibold text-slate-600">{project.progress_percent ?? 0}%</span>
        </div>
      </div>
      <div className="flex items-center justify-start gap-2 lg:justify-end">
        <button
          type="button"
          className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100"
          title="Xem nhanh"
          aria-label="Xem nhanh công trình"
          onClick={onView}
        >
          <Eye className="h-4 w-4" />
        </button>
        <button
          type="button"
          className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100"
          title="Chỉnh sửa"
          aria-label="Chỉnh sửa công trình"
          onClick={onEdit}
        >
          <Pencil className="h-4 w-4" />
        </button>
        <Link
          title="Mở cây nghiệp vụ"
          aria-label="Mở cây nghiệp vụ công trình"
          className="rounded-md border border-line p-1.5 text-blue-700 hover:bg-blue-50"
          to={`/projects/${project.id}`}
        >
          <FolderOpen className="h-4 w-4" />
        </Link>
      </div>
    </div>
  );
}

function ProjectFolderRow({ project, folder }: { project: Project; folder: ProjectWorkspaceGroup }) {
  const href = projectWorkspaceGroupPath(project.id, folder);
  return (
    <Link
      to={href}
      className="grid gap-3 px-4 py-2 hover:bg-white lg:grid-cols-[minmax(0,1.6fr)_160px_150px_120px_140px] lg:items-center"
    >
      <div className="ml-8 flex min-w-0 items-start gap-3 border-l border-slate-200 pl-4">
        <Folder className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-slate-700">{folder.explorerLabel ?? folder.label}</p>
          <p className="mt-0.5 truncate text-xs text-slate-500">{folder.description}</p>
        </div>
      </div>
      <span className="hidden text-xs text-slate-400 lg:block" />
      <span className="hidden text-xs text-slate-400 lg:block" />
      <span className="hidden text-xs text-slate-400 lg:block" />
      <span className="hidden text-right text-xs font-semibold text-blue-700 lg:block">Mở</span>
    </Link>
  );
}

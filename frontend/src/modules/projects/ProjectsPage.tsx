import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  CircleDot,
  Eye,
  Folder,
  FolderOpen,
  Pencil,
  Plus,
  Search,
  Upload,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { get, post, postForm, put } from '../../lib/api/client';
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
import { dateText } from '../../lib/utils/format';
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
  const [clientBoqFile, setClientBoqFile] = useState<File | null>(null);
  const [expandedProjectIds, setExpandedProjectIds] = useState<Set<string>>(new Set());
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Project[]>('/projects') });
  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<Record<string, any>[]>('/pipeline-templates') });
  const partners = useQuery({ queryKey: ['partners', 'client', 'active'], queryFn: () => get<Partner[]>('/partners?status=active&type=client') });
  const createProject = useMutation({
    mutationFn: async ({ values, boqFile }: { values: Record<string, any>; boqFile?: File | null }) => {
      const project = await post<Project>('/projects', values);
      if (values.workflow_entry_mode === 'client_boq' && boqFile) {
        const form = new FormData();
        form.append('file', boqFile);
        form.append('document_type', 'client_boq');
        form.append('name', boqFile.name);
        await postForm<Record<string, any>>(`/projects/${project.id}/documents/upload`, form);
      }
      return project;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setOpen(false);
      setClientBoqFile(null);
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
    {
      name: 'workflow_entry_mode',
      label: 'Điểm bắt đầu quy trình',
      type: 'select' as const,
      defaultValue: record?.workflow_entry_mode ?? 'survey_required',
      required: true,
      options: workflowEntryModeOptions,
      disabledWhen: record ? () => Boolean(record.current_pipeline_run_id) : undefined,
      helperText: record?.current_pipeline_run_id
        ? 'Điểm bắt đầu đã khóa vì công trình đã có quy trình đang chạy.'
        : 'Nếu chủ đầu tư đã có BOQ, công trình vào thẳng bước BOQ và không đi qua khảo sát/họp/thiết kế.',
    },
    { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date' as const, defaultValue: record?.planned_start_date?.slice?.(0, 10) ?? '' },
    { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date' as const, defaultValue: record?.planned_end_date?.slice?.(0, 10) ?? '' },
    { name: 'estimated_budget', label: 'Ngân sách dự kiến', type: 'number' as const, defaultValue: record?.estimated_budget ?? 300000000, min: 0 },
    { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number' as const, defaultValue: record?.contract_amount ?? 450000000, min: 0 },
    {
      name: 'pipeline_template_id',
      label: 'Pipeline template',
      type: 'select' as const,
      defaultValue: record?.pipeline_template_id ?? latestPublishedTemplateId(templates.data ?? []),
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
                    <ProjectExpandedWorkspace project={project} folders={visibleFolders} />
                  ) : null}
                </div>
              );
            })}
          </div>
        ) : null}
      </section>

      <FormDrawer open={open} title="Tạo công trình mới" onClose={() => { setOpen(false); setClientBoqFile(null); }}>
        <SimpleForm
          submitLabel="Tạo công trình"
          fields={projectFields()}
          renderAside={(values) => (
            <ProjectEntryModeAside
              entryMode={values.workflow_entry_mode}
              boqFile={clientBoqFile}
              onFileChange={setClientBoqFile}
            />
          )}
          onSubmit={(values) => {
            if (values.workflow_entry_mode === 'client_boq' && !clientBoqFile) {
              toast.warn({ title: 'Thiếu BOQ đầu vào', description: 'Chọn tệp BOQ của chủ đầu tư trước khi tạo công trình theo nhánh Báo giá.' });
              return;
            }
            return createProject.mutateAsync({ values, boqFile: clientBoqFile });
          }}
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
          { key: 'workflow_entry_mode', label: 'Điểm bắt đầu', render: (row) => workflowEntryModeLabel(row.workflow_entry_mode) },
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

function ProjectEntryModeAside({ entryMode, boqFile, onFileChange }: {
  entryMode: string;
  boqFile: File | null;
  onFileChange: (file: File | null) => void;
}) {
  const clientBoq = entryMode === 'client_boq';
  return (
    <div className="space-y-3">
      <div>
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Nhánh workflow</p>
        <h3 className="mt-1 text-sm font-semibold text-ink">{clientBoq ? 'Báo giá từ BOQ có sẵn' : 'Khảo sát trước báo giá'}</h3>
      </div>
      <div className="space-y-2 text-xs leading-5 text-slate-600">
        <p className={clientBoq ? 'text-slate-400 line-through' : 'font-semibold text-blue-700'}>1. Khảo sát → Họp khách hàng → Thiết kế</p>
        <p className="font-semibold text-blue-700">2. BOQ/Báo giá</p>
        <p>3. Hợp đồng đầu ra → bóc tách/mua hàng → triển khai</p>
      </div>
      {clientBoq ? (
        <label className="block rounded-md border border-dashed border-blue-300 bg-blue-50 p-3">
          <span className="flex items-center gap-2 text-sm font-semibold text-blue-900"><Upload className="h-4 w-4" /> BOQ của chủ đầu tư *</span>
          <input
            className="mt-2 block w-full text-xs text-slate-600"
            type="file"
            accept="application/pdf,image/png,image/jpeg,image/webp"
            onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
          />
          <p className="mt-2 break-all text-xs text-blue-800">{boqFile?.name ?? 'Chưa chọn tệp'}</p>
        </label>
      ) : (
        <div className="rounded-md border border-line bg-white p-3 text-xs leading-5 text-slate-600">
          Công trình sẽ bắt đầu bằng khảo sát và chỉ vào BOQ sau khi hoàn tất nhánh đầu vào.
        </div>
      )}
    </div>
  );
}

function latestPublishedTemplateId(templates: Record<string, any>[]) {
  return [...templates]
    .filter((template) => template.status === 'published')
    .sort((left, right) => Number(right.revision_no ?? right.version ?? 0) - Number(left.revision_no ?? left.version ?? 0))[0]?.id ?? '';
}

function workflowEntryModeLabel(mode?: string) {
  return workflowEntryModeOptions.find((option) => option.value === mode)?.label ?? mode ?? 'Chưa xác định';
}

const workflowEntryModeOptions = [
  { value: 'survey_required', label: 'Khảo sát trước báo giá' },
  { value: 'client_boq', label: 'Chủ đầu tư có sẵn BOQ · vào thẳng Báo giá' },
];

function ProjectExpandedWorkspace({ project, folders }: { project: Project; folders: ProjectWorkspaceGroup[] }) {
  return (
    <div className="border-t border-slate-100 bg-slate-50/70 p-3 sm:p-4">
      <div className="grid items-stretch gap-4 lg:grid-cols-[minmax(0,7fr)_minmax(320px,5fr)]">
        <section className="min-w-0" aria-label="Phân hệ nghiệp vụ">
          <p className="px-1 text-xs font-semibold uppercase tracking-wide text-slate-500">Phân hệ nghiệp vụ</p>
          <div className="mt-2 divide-y divide-slate-100 overflow-hidden rounded-lg border border-line bg-white">
            {folders.map((folder) => (
              <ProjectFolderRow key={folder.key} project={project} folder={folder} />
            ))}
          </div>
        </section>

        <ProjectQuickDashboard project={project} />
      </div>
    </div>
  );
}

function ProjectQuickDashboard({ project }: { project: Project }) {
  const progress = boundedPercent(project.progress_percent);
  const openIssues = nonNegativeCount(project.open_issues);
  const pendingApprovals = nonNegativeCount(project.pending_approvals);
  const hasPlannedEnd = Boolean(project.planned_end_date);

  return (
    <aside className="flex min-w-0 flex-col" aria-label={`Tổng quan nhanh ${project.code}`}>
      <p className="px-1 text-xs font-semibold uppercase tracking-wide text-slate-500">Tổng quan nhanh</p>
      <div className="mt-2 grid flex-1 grid-cols-2 gap-px overflow-hidden rounded-lg border border-line bg-line">
        <ProjectQuickMetric
          label="Tiến độ"
          value={`${progress}%`}
          sub="Tiến độ tổng thể"
          icon={<CircleDot className="h-4 w-4" />}
          tone="blue"
          progress={progress}
        />
        <ProjectQuickMetric
          label="Vướng mắc"
          value={openIssues}
          sub={openIssues > 0 ? 'Cần xử lý' : 'Không có'}
          icon={<AlertTriangle className="h-4 w-4" />}
          tone={openIssues > 0 ? 'rose' : 'slate'}
        />
        <ProjectQuickMetric
          label="Chờ phê duyệt"
          value={pendingApprovals}
          sub={pendingApprovals > 0 ? 'Đang chờ' : 'Không có'}
          icon={<CheckCircle2 className="h-4 w-4" />}
          tone={pendingApprovals > 0 ? 'amber' : 'slate'}
        />
        <ProjectQuickMetric
          label="Hạn dự kiến"
          value={hasPlannedEnd ? dateText(project.planned_end_date) : 'Chưa thiết lập'}
          sub={project.planned_start_date ? `Bắt đầu ${dateText(project.planned_start_date)}` : 'Chưa có mốc bắt đầu'}
          icon={<CalendarDays className="h-4 w-4" />}
          tone="slate"
        />
      </div>
    </aside>
  );
}

type ProjectQuickMetricTone = 'blue' | 'rose' | 'amber' | 'slate';

const projectQuickMetricToneClasses: Record<ProjectQuickMetricTone, { icon: string; value: string; progress: string }> = {
  blue: { icon: 'bg-blue-50 text-blue-700', value: 'text-blue-700', progress: 'bg-blue-600' },
  rose: { icon: 'bg-rose-50 text-rose-700', value: 'text-rose-700', progress: 'bg-rose-600' },
  amber: { icon: 'bg-amber-50 text-amber-700', value: 'text-amber-700', progress: 'bg-amber-600' },
  slate: { icon: 'bg-slate-100 text-slate-600', value: 'text-ink', progress: 'bg-slate-500' },
};

function ProjectQuickMetric({
  label,
  value,
  sub,
  icon,
  tone,
  progress,
}: {
  label: string;
  value: ReactNode;
  sub: string;
  icon: ReactNode;
  tone: ProjectQuickMetricTone;
  progress?: number;
}) {
  const toneClasses = projectQuickMetricToneClasses[tone];
  return (
    <div className="min-w-0 bg-white p-3">
      <div className="flex items-start justify-between gap-2">
        <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
        <span className={`grid h-7 w-7 shrink-0 place-items-center rounded-md ${toneClasses.icon}`}>{icon}</span>
      </div>
      <div className={`mt-2 break-words text-lg font-bold leading-tight tabular-nums sm:text-xl ${toneClasses.value}`}>{value}</div>
      {progress !== undefined ? (
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-100" aria-hidden="true">
          <div className={`h-full rounded-full ${toneClasses.progress}`} style={{ width: `${progress}%` }} />
        </div>
      ) : null}
      <p className="mt-2 truncate text-xs text-slate-500">{sub}</p>
    </div>
  );
}

function boundedPercent(value: unknown) {
  const parsed = Number(value ?? 0);
  if (!Number.isFinite(parsed)) return 0;
  return Math.min(100, Math.max(0, Math.round(parsed)));
}

function nonNegativeCount(value: unknown) {
  const parsed = Number(value ?? 0);
  if (!Number.isFinite(parsed)) return 0;
  return Math.max(0, Math.round(parsed));
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
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5 hover:bg-slate-50"
    >
      <div className="ml-3 flex min-w-0 items-start gap-3 border-l border-slate-200 pl-3 sm:ml-6 sm:pl-4">
        <Folder className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-slate-700">{folder.explorerLabel ?? folder.label}</p>
          <p className="mt-0.5 truncate text-xs text-slate-500">{folder.description}</p>
        </div>
      </div>
      <span className="text-right text-xs font-semibold text-blue-700">Mở</span>
    </Link>
  );
}

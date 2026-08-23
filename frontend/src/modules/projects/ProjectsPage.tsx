import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  ChevronsUpDown,
  CircleDot,
  Folder,
  FolderOpen,
  FileDown,
  FileUp,
  Pencil,
  Plus,
  Search,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { get, post, put, streamSSE, type StreamStatus } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { IdentityReferenceLabel } from '../../components/IdentityReferenceLabel';
import { useAuth } from '../../lib/auth/AuthContext';
import { SimpleForm } from '../_shared/SimpleForm';
import type { Partner, Project, ProjectExplorerPage } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { partnerOptions } from '../core-data/partnerUtils';
import { BulkSelectionBar } from '../../components/BulkSelectionBar';
import { BulkDeleteDialog, type BulkDeletionResult } from '../../components/BulkDeleteDialog';
import { SelectionCheckbox } from '../../components/SelectionCheckbox';
import {
  projectWorkspaceGroupPath,
  visibleProjectWorkspaceGroups,
  type ProjectWorkspaceGroup,
} from './projectWorkspaceNavigation';
import {
  nextProjectExplorerSort,
  projectExplorerSortFromValue,
  projectExplorerSortOptions,
  projectExplorerSortValue,
  type ProjectExplorerSort,
  type ProjectExplorerSortKey,
} from './projectExplorerSort';
import { ProjectImportDialog } from './ProjectImportDialog';
import { ProjectExportDialog } from './ProjectExportDialog';

const PROJECT_EXPLORER_DESKTOP_GRID_CLASS = 'lg:grid-cols-[112px_minmax(0,1.6fr)_135px_100px_112px_100px]';
const PROJECT_EXPLORER_SELECTABLE_DESKTOP_GRID_CLASS = 'lg:grid-cols-[32px_112px_minmax(0,1.6fr)_135px_100px_112px_100px]';

export function ProjectsPage() {
  const [open, setOpen] = useState(false);
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [search, setSearch] = useState('');
	const [debouncedSearch, setDebouncedSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [projectSort, setProjectSort] = useState<ProjectExplorerSort | null>(null);
	const [page, setPage] = useState(1);
	const [pageSize, setPageSize] = useState(25);
	const [streamStatus, setStreamStatus] = useState<StreamStatus>('reconnecting');
  const [expandedProjectIds, setExpandedProjectIds] = useState<Set<string>>(new Set());
  const [selectedProjectIds, setSelectedProjectIds] = useState<Set<string>>(new Set());
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission, user } = useAuth();
	const canDelete = hasAnyPermission(['project.delete']);
	const canImport = hasAnyPermission(['project.import']);
	const canExport = hasAnyPermission(['project.export']);
	const canSelect = canDelete || canExport;
	const explorerParams = useMemo(() => {
		const params = new URLSearchParams({
			page: String(page),
			page_size: String(pageSize),
			search: debouncedSearch,
			status: statusFilter,
			sort: projectSort?.key ?? 'created_at',
			direction: projectSort?.direction ?? 'desc',
		});
		return params.toString();
	}, [debouncedSearch, page, pageSize, projectSort, statusFilter]);
	const projects = useQuery({
		queryKey: ['project-explorer', user?.id, explorerParams],
		queryFn: () => get<ProjectExplorerPage>(`/projects/explorer?${explorerParams}`),
		staleTime: 0,
		refetchInterval: streamStatus === 'connected' ? false : 60_000,
	});
  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<Record<string, any>[]>('/pipeline-templates') });
  const partners = useQuery({ queryKey: ['partners', 'client', 'active'], queryFn: () => get<Partner[]>('/partners?status=active&type=client') });
  const createProject = useMutation({
    mutationFn: (values: Record<string, any>) => post<Project>('/projects', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
		queryClient.invalidateQueries({ queryKey: ['project-explorer'] });
      setOpen(false);
      toast.success('Đã tạo công trình');
    },
    onError: (error) => toast.error('Không tạo được công trình', error),
  });
  const updateProject = useMutation({
    mutationFn: ({ projectId, values }: { projectId: string; values: Record<string, any> }) => put<Project>(`/projects/${projectId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
		queryClient.invalidateQueries({ queryKey: ['project-explorer'] });
      setEditingProject(null);
      toast.success('Đã cập nhật công trình');
    },
    onError: (error) => toast.error('Không cập nhật được công trình', error),
  });

  const projectFields = (record?: Project) => [
    { name: 'name', label: 'Tên công trình', defaultValue: record?.name ?? 'Công trình mới', required: true },
    { name: 'code', label: 'Mã công trình', defaultValue: record?.code ?? `PRJ-${Date.now().toString().slice(-5)}`, required: true },
    {
      name: 'partner_id',
      label: 'Chọn chủ đầu tư có sẵn',
      type: 'select' as const,
      defaultValue: record?.partner_id ?? '',
      options: partnerOptions(partners.data ?? [], ['client']),
      helperText: 'Khi chọn, hệ thống lưu tên chủ đầu tư hiện tại vào công trình.',
      fullWidth: true,
    },
    {
      name: 'client_name',
      label: 'Khách hàng / Chủ đầu tư',
      defaultValue: record?.client_name ?? 'Khách hàng mới',
      required: true,
      hiddenWhen: (values: Record<string, any>) => Boolean(values.partner_id),
      fullWidth: true,
    },
    { name: 'address', label: 'Địa chỉ', defaultValue: record?.address ?? '', fullWidth: true },
    { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date' as const, defaultValue: record?.planned_start_date?.slice?.(0, 10) ?? '' },
    { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date' as const, defaultValue: record?.planned_end_date?.slice?.(0, 10) ?? '' },
    { name: 'estimated_budget', label: 'Ngân sách dự kiến', type: 'currency' as const, defaultValue: record?.estimated_budget ?? 300000000, min: 0 },
    { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'currency' as const, defaultValue: record?.contract_amount ?? 450000000, min: 0 },
    {
      name: 'pipeline_template_id',
      label: 'Pipeline template',
      type: 'select' as const,
      defaultValue: record?.pipeline_template_id ?? latestPublishedTemplateId(templates.data ?? []),
      options: (templates.data ?? [])
        .filter((template) => template.status === 'published')
        .map((template) => ({ label: `${template.name} · revision ${template.revision_no ?? template.version}`, value: template.id })),
      fullWidth: true,
    },
  ];

  const visibleFolders = useMemo(
    () => visibleProjectWorkspaceGroups(hasAnyPermission),
    [hasAnyPermission],
  );
  const statusOptions = projects.data?.options.statuses ?? [];
	const filteredProjects = projects.data?.items ?? [];
	const selectedVisibleCount = filteredProjects.filter((project) => selectedProjectIds.has(project.id)).length;
	const allVisibleSelected = Boolean(filteredProjects.length && selectedVisibleCount === filteredProjects.length);
	const someVisibleSelected = selectedVisibleCount > 0 && !allVisibleSelected;
	const selectedProjectIDs = useMemo(() => Array.from(selectedProjectIds), [selectedProjectIds]);

	useEffect(() => {
		const timer = window.setTimeout(() => {
			setDebouncedSearch(search.trim());
			setPage(1);
		}, 300);
		return () => window.clearTimeout(timer);
	}, [search]);

	useEffect(() => {
		setSelectedProjectIds(new Set());
	}, [explorerParams]);

	useEffect(() => {
		let invalidateTimer: number | undefined;
		const stop = streamSSE('/projects/explorer/stream', {
			onStatus: setStreamStatus,
			onEvent: (message) => {
				if (message.event !== 'project_explorer_changed') return;
				if (invalidateTimer !== undefined) window.clearTimeout(invalidateTimer);
				invalidateTimer = window.setTimeout(() => {
					void queryClient.invalidateQueries({ queryKey: ['project-explorer'] });
				}, 300);
			},
		});
		return () => {
			stop();
			if (invalidateTimer !== undefined) window.clearTimeout(invalidateTimer);
		};
	}, [queryClient]);

	useEffect(() => {
		if (projects.data && projects.data.total_pages > 0 && page > projects.data.total_pages) {
			setPage(projects.data.total_pages);
		}
	}, [page, projects.data]);

  useEffect(() => {
		const rows = projects.data?.items ?? [];
		if (!rows.length) {
			setExpandedProjectIds(new Set());
			setSelectedProjectIds(new Set());
			return;
		}
    setExpandedProjectIds((current) => {
			const visible = new Set(rows.map((project) => project.id));
			const retained = new Set(Array.from(current).filter((projectID) => visible.has(projectID)));
			return retained;
    });
		setSelectedProjectIds((current) => {
			const visible = new Set(rows.map((project) => project.id));
			return new Set(Array.from(current).filter((projectID) => visible.has(projectID)));
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

	function toggleSelectedProject(projectId: string) {
		setSelectedProjectIds((current) => {
			const next = new Set(current);
			if (next.has(projectId)) next.delete(projectId);
			else next.add(projectId);
			return next;
		});
	}

	function toggleAllVisibleProjects() {
		setSelectedProjectIds((current) => {
			const next = new Set(current);
			if (allVisibleSelected) filteredProjects.forEach((project) => next.delete(project.id));
			else filteredProjects.forEach((project) => next.add(project.id));
			return next;
		});
	}

  return (
    <>
      <PageHeader
        title="Công trình"
        description="Chọn công trình trước, sau đó mở cây nghiệp vụ bên trong công trình."
        horizontalFrom="xl"
        actions={
          <>
            {canExport ? (
              <button className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setExportOpen(true)}>
                <FileDown className="h-4 w-4" />
                Xuất dữ liệu
              </button>
            ) : null}
            {canImport ? (
              <button className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setImportOpen(true)}>
                <FileUp className="h-4 w-4" />
                Nhập dữ liệu
              </button>
            ) : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}>
              <Plus className="h-4 w-4" />
              Tạo công trình
            </button>
          </>
        }
      />

		{canDelete ? <BulkSelectionBar count={selectedProjectIds.size} itemLabel="công trình" actionLabel="Xóa công trình đã chọn" onClear={() => setSelectedProjectIds(new Set())} onDelete={() => setDeleteOpen(true)} /> : null}
		{canExport && !canDelete && selectedProjectIds.size > 0 ? (
			<div className="mb-4 flex flex-col gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
				<p className="text-sm font-semibold text-blue-900">Đã chọn {selectedProjectIds.size} công trình trên trang hiện tại</p>
				<div className="flex gap-2">
					<button type="button" className="rounded-md border border-blue-200 bg-white px-3 py-1.5 text-sm font-semibold text-blue-800" onClick={() => setSelectedProjectIds(new Set())}>Bỏ chọn</button>
					<button type="button" className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-semibold text-white" onClick={() => setExportOpen(true)}>Xuất đã chọn</button>
				</div>
			</div>
		) : null}

      <section className="rounded-lg border border-line bg-white shadow-panel">
        <div className="border-b border-line p-4">
          <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm focus-ring"
                value={search}
                placeholder="Tìm công trình, mã, chủ đầu tư"
				onChange={(event) => { setSearch(event.target.value); setSelectedProjectIds(new Set()); }}
              />
            </label>
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
              value={statusFilter}
				onChange={(event) => {
					setStatusFilter(event.target.value);
					setPage(1);
				}}
            >
              <option value="">Tất cả trạng thái</option>
              {statusOptions.map((status) => (
                <option key={status} value={status}>{status}</option>
              ))}
            </select>
            <select
              aria-label="Sắp xếp công trình"
              className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring lg:hidden"
              value={projectExplorerSortValue(projectSort)}
				onChange={(event) => {
					setProjectSort(projectExplorerSortFromValue(event.target.value));
					setPage(1);
				}}
            >
              <option value="">Thứ tự mặc định</option>
              {projectExplorerSortOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </div>
        </div>

        <ProjectExplorerHeader
          sort={projectSort}
			selectable={canSelect}
			allSelected={allVisibleSelected}
			someSelected={someVisibleSelected}
			onToggleAll={toggleAllVisibleProjects}
			onSort={(key) => {
				setProjectSort((current) => nextProjectExplorerSort(key, current));
				setPage(1);
			}}
        />

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
			<EmptyState title={debouncedSearch || statusFilter ? 'Không tìm thấy công trình phù hợp' : 'Chưa có công trình'} />
          </div>
        ) : null}

        {!projects.isLoading && !projects.error && filteredProjects.length ? (
          <div className="space-y-3 bg-slate-50/80 p-3 lg:space-y-0 lg:bg-transparent lg:p-0 lg:divide-y lg:divide-line">
            {filteredProjects.map((project) => {
              const expanded = expandedProjectIds.has(project.id);
              return (
                <div
                  key={project.id}
                  className="overflow-hidden rounded-lg border border-line bg-white shadow-sm lg:rounded-none lg:border-0 lg:shadow-none"
                >
                  <ProjectExplorerRow
                    project={project}
                    expanded={expanded}
					selectable={canSelect}
					selected={selectedProjectIds.has(project.id)}
					onSelectedChange={() => toggleSelectedProject(project.id)}
                    onToggle={() => toggleProject(project.id)}
                    onEdit={() => setEditingProject(project)}
                  />
                  {expanded ? (
                    <ProjectExpandedWorkspace project={project} folders={visibleFolders} />
                  ) : null}
                </div>
              );
            })}
          </div>
        ) : null}

		{!projects.isLoading && !projects.error && projects.data ? (
			<ProjectExplorerPagination
				page={page}
				pageSize={pageSize}
				total={projects.data.total}
				totalPages={projects.data.total_pages}
				onPageChange={setPage}
				onPageSizeChange={(nextPageSize) => {
					setPageSize(nextPageSize);
					setPage(1);
				}}
			/>
		) : null}
      </section>

		<BulkDeleteDialog
			kind="project"
			open={deleteOpen}
			ids={selectedProjectIDs}
			onClose={() => setDeleteOpen(false)}
			onSuccess={async (result: BulkDeletionResult) => {
				setDeleteOpen(false);
				setSelectedProjectIds(new Set());
				await Promise.all([
					queryClient.invalidateQueries({ queryKey: ['projects'] }),
					queryClient.invalidateQueries({ queryKey: ['project-explorer'] }),
				]);
				toast.success({ title: `Đã xóa ${result.deleted_count} công trình`, description: 'Công trình đã chuyển sang Đã hủy; toàn bộ lịch sử nghiệp vụ vẫn được giữ nguyên.' });
			}}
			onError={(title, error) => toast.error(title, error)}
		/>

		<ProjectImportDialog
			open={importOpen}
			onClose={() => setImportOpen(false)}
			onSuccess={async (result) => {
				setImportOpen(false);
				setSelectedProjectIds(new Set());
				await Promise.all([
					queryClient.invalidateQueries({ queryKey: ['projects'] }),
					queryClient.invalidateQueries({ queryKey: ['project-explorer'] }),
				]);
				toast.success({ title: `Đã tạo ${result.created_count} công trình`, description: 'Các công trình ở trạng thái Nháp; Pipeline chưa được khởi chạy.' });
			}}
			onError={(title, error) => toast.error(title, error)}
		/>

		<ProjectExportDialog
			open={exportOpen}
			selectedProjectIDs={selectedProjectIDs}
			filteredCount={projects.data?.total ?? 0}
			filters={{
				search: debouncedSearch,
				status: statusFilter,
				sort: projectSort?.key ?? 'created_at',
				direction: projectSort?.direction ?? 'desc',
			}}
			onClose={() => setExportOpen(false)}
			onSuccess={(scope) => toast.success(scope === 'selected' ? 'Đã xuất các công trình được chọn' : 'Đã xuất công trình theo bộ lọc')}
			onError={(title, error) => toast.error(title, error)}
		/>

      <FormDrawer open={open} title="Tạo công trình mới" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Tạo công trình"
          fields={projectFields()}
          onSubmit={(values) => createProject.mutateAsync(values)}
        />
      </FormDrawer>
      <FormDrawer open={Boolean(editingProject)} title="Chỉnh sửa công trình" onClose={() => setEditingProject(null)}>
        {editingProject ? (
          <div className="space-y-5">
            <ProjectEditSummary project={editingProject} />
            <SimpleForm
              submitLabel="Lưu thay đổi"
              fields={projectFields(editingProject)}
              layout="two-column"
              sections={[
                {
                  title: 'Thông tin công trình',
                  fields: ['name', 'code', 'partner_id', 'client_name', 'address'],
                },
                {
                  title: 'Kế hoạch & giá trị',
                  fields: ['planned_start_date', 'planned_end_date', 'estimated_budget', 'contract_amount'],
                },
                {
                  title: 'Quy trình',
                  fields: ['pipeline_template_id'],
                },
              ]}
              onCancel={() => setEditingProject(null)}
              onSubmit={(values) => updateProject.mutateAsync({
                projectId: editingProject.id,
                values: { ...values, partner_id: values.partner_id || null, pipeline_template_id: values.pipeline_template_id || null },
              })}
            />
          </div>
        ) : null}
      </FormDrawer>
    </>
  );
}

function ProjectExplorerPagination({
	page,
	pageSize,
	total,
	totalPages,
	onPageChange,
	onPageSizeChange,
}: {
	page: number;
	pageSize: number;
	total: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	onPageSizeChange: (pageSize: number) => void;
}) {
	const first = total === 0 ? 0 : (page - 1) * pageSize + 1;
	const last = Math.min(total, page * pageSize);
	return (
		<div className="flex flex-col gap-3 border-t border-line px-4 py-3 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between">
			<p>Hiển thị <span className="font-semibold text-ink">{first}–{last}</span> trong <span className="font-semibold text-ink">{total}</span> công trình</p>
			<div className="flex flex-wrap items-center gap-2">
				<label className="flex items-center gap-2">
					<span>Số dòng</span>
					<select
						aria-label="Số công trình mỗi trang"
						className="rounded-md border border-line bg-white px-2 py-1.5 focus-ring"
						value={pageSize}
						onChange={(event) => onPageSizeChange(Number(event.target.value))}
					>
						{[25, 50, 100].map((value) => <option key={value} value={value}>{value}</option>)}
					</select>
				</label>
				<button
					type="button"
					className="rounded-md border border-line bg-white px-3 py-1.5 font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
					disabled={page <= 1}
					onClick={() => onPageChange(page - 1)}
				>Trước</button>
				<span className="min-w-20 text-center font-medium">Trang {totalPages === 0 ? 0 : page}/{totalPages}</span>
				<button
					type="button"
					className="rounded-md border border-line bg-white px-3 py-1.5 font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
					disabled={totalPages === 0 || page >= totalPages}
					onClick={() => onPageChange(page + 1)}
				>Sau</button>
			</div>
		</div>
	);
}

function latestPublishedTemplateId(templates: Record<string, any>[]) {
  return [...templates]
    .filter((template) => template.status === 'published')
    .sort((left, right) => Number(right.revision_no ?? right.version ?? 0) - Number(left.revision_no ?? left.version ?? 0))[0]?.id ?? '';
}

function ProjectEditSummary({ project }: { project: Project }) {
  const progress = boundedPercent(project.progress_percent ?? 0);

  return (
    <section className="rounded-lg border border-line bg-slate-50 p-4" aria-label="Tổng quan công trình">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">Tổng quan</h3>
          <p className="mt-1 truncate text-xs text-slate-500">{project.code} · {project.name}</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Trạng thái</span>
          <StatusBadge status={project.status} />
        </div>
      </div>
      <dl className="mt-4 grid gap-x-5 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ</dt>
          <dd className="mt-1 flex items-center gap-2 text-sm font-semibold text-ink">
            <span>{progress}%</span>
            <span className="h-1.5 min-w-20 flex-1 overflow-hidden rounded-full bg-slate-200" aria-hidden="true">
              <span className="block h-full rounded-full bg-blue-600" style={{ width: `${progress}%` }} />
            </span>
          </dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Chủ đầu tư đã chọn</dt>
          <dd className="mt-1 break-words text-sm text-ink">{project.partner_name || '-'}</dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">MST đối tác</dt>
          <dd className="mt-1 break-words text-sm text-ink">{project.partner_tax_code || '-'}</dd>
        </div>
      </dl>
    </section>
  );
}

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

function ProjectExplorerHeader({
  sort,
  selectable,
  allSelected,
  someSelected,
  onToggleAll,
  onSort,
}: {
  sort: ProjectExplorerSort | null;
  selectable: boolean;
  allSelected: boolean;
  someSelected: boolean;
  onToggleAll: () => void;
  onSort: (key: ProjectExplorerSortKey) => void;
}) {
  const gridClass = selectable ? PROJECT_EXPLORER_SELECTABLE_DESKTOP_GRID_CLASS : PROJECT_EXPLORER_DESKTOP_GRID_CLASS;
  return (
    <div className={`hidden ${gridClass} gap-3 border-b border-line bg-slate-50 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-500 lg:grid`}>
      {selectable ? <SelectionCheckbox checked={allSelected} indeterminate={someSelected} ariaLabel="Chọn tất cả công trình trên trang hiện tại" onChange={onToggleAll} /> : null}
      <span><span className="sr-only">Thao tác</span></span>
      <ProjectExplorerSortHeader label="Cây công trình" sortKey="project" sort={sort} onSort={onSort} />
      <ProjectExplorerSortHeader label="Trạng thái" sortKey="status" sort={sort} onSort={onSort} />
      <ProjectExplorerSortHeader label="Phụ trách" sortKey="owner" sort={sort} onSort={onSort} />
      <span>Tiến độ</span>
      <ProjectExplorerSortHeader label="Ngày tạo" sortKey="created_at" sort={sort} onSort={onSort} />
    </div>
  );
}

function ProjectExplorerSortHeader({
  label,
  sortKey,
  sort,
  onSort,
}: {
  label: string;
  sortKey: ProjectExplorerSortKey;
  sort: ProjectExplorerSort | null;
  onSort: (key: ProjectExplorerSortKey) => void;
}) {
  const activeDirection = sort?.key === sortKey ? sort.direction : null;
  const nextDirection = sortKey === 'created_at'
    ? activeDirection === 'asc' ? 'mới nhất trước' : 'cũ nhất trước'
    : activeDirection === 'asc' ? 'giảm dần' : 'tăng dần';

  return (
    <button
      type="button"
      className={`inline-flex w-fit items-center gap-1 rounded-sm text-left uppercase hover:text-ink focus-ring ${activeDirection ? 'text-blue-700' : ''}`}
      aria-label={`Sắp xếp ${label} ${nextDirection}`}
      onClick={() => onSort(sortKey)}
    >
      <span>{label}</span>
      {activeDirection === 'asc' ? (
        <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" />
      ) : activeDirection === 'desc' ? (
        <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
      ) : (
        <ChevronsUpDown className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
      )}
    </button>
  );
}

function ProjectExplorerRow({
  project,
  expanded,
  selectable,
  selected,
  onSelectedChange,
  onToggle,
  onEdit,
}: {
  project: Project;
  expanded: boolean;
  selectable: boolean;
  selected: boolean;
  onSelectedChange: () => void;
  onToggle: () => void;
  onEdit: () => void;
}) {
  const gridClass = selectable ? PROJECT_EXPLORER_SELECTABLE_DESKTOP_GRID_CLASS : PROJECT_EXPLORER_DESKTOP_GRID_CLASS;
  return (
    <div className={`relative grid gap-3 px-4 py-3 ${selectable ? 'pl-12' : ''} ${selected ? 'bg-blue-50/60' : ''} ${gridClass} lg:items-center lg:pl-4`}>
      {selectable ? <div className="absolute left-4 top-4 lg:static"><SelectionCheckbox checked={selected} ariaLabel={`Chọn công trình ${project.code} · ${project.name}`} onChange={onSelectedChange} /></div> : null}
      <div className="flex items-center justify-start gap-2">
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
      <div className="text-sm text-slate-700"><IdentityReferenceLabel name={project.pm_name} deleted={project.pm_account_deleted} liveOwnership /></div>
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-slate-100">
            <div className="h-full rounded-full bg-blue-600" style={{ width: `${Math.min(100, Math.max(0, Number(project.progress_percent ?? 0)))}%` }} />
          </div>
          <span className="shrink-0 text-xs font-semibold tabular-nums text-slate-600">{project.progress_percent ?? 0}%</span>
        </div>
      </div>
      <div className="whitespace-nowrap text-sm tabular-nums text-slate-600">
        <span className="font-medium text-slate-500 lg:hidden">Ngày tạo: </span>
        {dateText(project.created_at)}
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

import { Link, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  ArrowLeft,
  ChevronDown,
  Folder,
  FolderOpen,
  LogOut,
  PanelLeftClose,
} from 'lucide-react';
import clsx from 'clsx';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ProjectOperationsResponse } from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';

type WorkspaceChild = {
  label: string;
  to: string;
  permissions?: string[];
};

type WorkspaceGroup = {
  key: string;
  label: string;
  to: string;
  permissions?: string[];
  children: WorkspaceChild[];
};

const workspaceGroups: WorkspaceGroup[] = [
  {
    key: 'overview',
    label: 'Tổng quan',
    to: '',
    permissions: ['project.view'],
    children: [],
  },
  {
    key: 'planning',
    label: 'Kế hoạch & Workflow',
    to: 'planning',
    permissions: ['planning.view', 'pipeline.view', 'boq.view'],
    children: [
      { label: 'Pipeline', to: 'pipeline', permissions: ['pipeline.view'] },
      { label: 'Planning', to: 'planning', permissions: ['planning.view', 'pipeline.view'] },
      { label: 'BOQ / Dự toán', to: 'boq', permissions: ['boq.view'] },
    ],
  },
  {
    key: 'materials',
    label: 'Mua sắm & Vật tư',
    to: 'procurement',
    permissions: ['procurement.view', 'warehouse.view'],
    children: [
      { label: 'Mua sắm', to: 'procurement', permissions: ['procurement.view'] },
      { label: 'Kho vật tư', to: 'warehouse', permissions: ['warehouse.view'] },
    ],
  },
  {
    key: 'quality',
    label: 'Thi công & Chất lượng',
    to: 'operations',
    permissions: ['operation.view', 'acceptance.view', 'rework.view'],
    children: [
      { label: 'Thi công', to: 'operations', permissions: ['operation.view'] },
      { label: 'Nghiệm thu', to: 'acceptance', permissions: ['acceptance.view'] },
      { label: 'Rework', to: 'rework', permissions: ['rework.view'] },
    ],
  },
  {
    key: 'finance',
    label: 'Bàn giao & Tài chính',
    to: 'handover',
    permissions: ['handover.view', 'accounting.view', 'accounting.payable_view', 'accounting.receivable_view'],
    children: [
      { label: 'Bàn giao', to: 'handover', permissions: ['handover.view'] },
      { label: 'Kế toán', to: 'accounting', permissions: ['accounting.view'] },
      { label: 'Công nợ', to: 'debt', permissions: ['accounting.payable_view', 'accounting.receivable_view'] },
    ],
  },
  {
    key: 'governance',
    label: 'Quản trị công trình',
    to: '',
    permissions: ['project.view', 'audit.view'],
    children: [
      { label: 'Thành viên', to: '?tab=members', permissions: ['project.view'] },
      { label: 'Hồ sơ', to: '?tab=files', permissions: ['project.view'] },
      { label: 'Audit', to: 'audit', permissions: ['audit.view'] },
    ],
  },
];

export function ProjectWorkspaceLayout() {
  const { id = '' } = useParams();
  const { user, logout, hasAnyPermission } = useAuth();
  const navigate = useNavigate();
  const operations = useQuery({
    queryKey: ['project-operations', id],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id),
    retry: false,
  });

  const project = operations.data?.project_summary ?? operations.data?.project;
  const visibleGroups = workspaceGroups
    .map((group) => ({
      ...group,
      children: group.children.filter((child) => hasAnyPermission(child.permissions)),
    }))
    .filter((group) => hasAnyPermission(group.permissions) || group.children.length > 0);

  return (
    <div className="min-h-screen bg-surface">
      <header className="sticky top-0 z-30 border-b border-line bg-white px-4 py-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={() => navigate('/projects')}
            >
              <ArrowLeft className="h-4 w-4" />
              Công trình
            </button>
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-ink">
                {project ? `${project.code} · ${project.name}` : operations.isLoading ? 'Đang tải công trình...' : 'Workspace công trình'}
              </p>
              <p className="truncate text-xs text-slate-500">
                {[
                  project?.client_name ? `Chủ đầu tư: ${project.client_name}` : null,
                  project?.pm_name ? `Phụ trách: ${project.pm_name}` : null,
                  project?.status ? `Trạng thái: ${statusLabel(project.status)}` : null,
                ].filter(Boolean).join(' · ') || 'Cây nghiệp vụ là menu chính trong scope công trình'}
              </p>
            </div>
          </div>
          <div className="flex items-center justify-between gap-3 lg:justify-end">
            <div className="min-w-0 text-left lg:text-right">
              <p className="truncate text-sm font-semibold text-ink">{user?.full_name}</p>
              <p className="truncate text-xs text-slate-500">{user?.roles?.join(', ')}</p>
            </div>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={() => {
                void logout().finally(() => navigate('/login'));
              }}
            >
              <LogOut className="h-4 w-4" />
              Đăng xuất
            </button>
          </div>
        </div>
      </header>

      <div className="grid min-h-[calc(100vh-73px)] lg:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="border-b border-line bg-white lg:sticky lg:top-[73px] lg:h-[calc(100vh-73px)] lg:overflow-auto lg:border-b-0 lg:border-r">
          <div className="border-b border-line px-4 py-3">
            <div className="flex items-center gap-2">
              <PanelLeftClose className="h-4 w-4 text-blue-700" />
              <p className="text-sm font-semibold text-ink">Cây nghiệp vụ</p>
            </div>
          </div>
          <nav className="px-3 py-3">
            <div className="mb-2 rounded-md border border-line bg-slate-50 px-3 py-2">
              <div className="flex min-w-0 items-center gap-2">
                <ChevronDown className="h-4 w-4 shrink-0 text-slate-500" />
                <FolderOpen className="h-4 w-4 shrink-0 text-blue-700" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink">{project?.name ?? 'Công trình'}</p>
                  <p className="truncate text-xs text-slate-500">{project?.code ?? id}</p>
                </div>
              </div>
            </div>

            <div className="space-y-1">
              {visibleGroups.map((group) => (
                <WorkspaceGroupNode key={group.key} group={group} projectId={id} />
              ))}
            </div>
          </nav>
        </aside>

        <main className="min-w-0 px-4 py-5 md:px-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function WorkspaceGroupNode({ group, projectId }: { group: WorkspaceGroup; projectId: string }) {
  const location = useLocation();
  const groupTarget = projectWorkspacePath(projectId, group.children[0]?.to ?? group.to);
  const currentTarget = `${location.pathname}${location.search}`;
  const groupActive = group.children.length
    ? group.children.some((child) => {
      const childTarget = projectWorkspacePath(projectId, child.to);
      return childTarget.includes('?') ? currentTarget === childTarget : location.pathname === childTarget;
    })
    : currentTarget === groupTarget;
  return (
    <div>
      <Link
        to={groupTarget}
        className={clsx(
          'flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold focus-ring',
          groupActive ? 'bg-blue-50 text-blue-700' : 'text-slate-700 hover:bg-slate-50 hover:text-ink',
        )}
      >
        <ChevronDown className="h-4 w-4 shrink-0" />
        <Folder className="h-4 w-4 shrink-0" />
        <span className="min-w-0 truncate">{group.label}</span>
      </Link>
      {group.children.length ? (
        <div className="mb-1">
          {group.children.map((child) => {
            const childTarget = projectWorkspacePath(projectId, child.to);
            const active = childTarget.includes('?') ? currentTarget === childTarget : location.pathname === childTarget;
            return (
              <Link
                key={`${group.key}-${child.label}`}
                to={childTarget}
                className={clsx(
                  'ml-8 flex w-[calc(100%-2rem)] items-center gap-2 border-l border-slate-200 px-4 py-1.5 text-sm focus-ring',
                  active ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50 hover:text-blue-700',
                )}
              >
                <Folder className="h-3.5 w-3.5 shrink-0" />
                <span className="min-w-0 truncate">{child.label}</span>
              </Link>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

function projectWorkspacePath(projectId: string, target: string) {
  if (!target) return `/projects/${projectId}`;
  if (target.startsWith('?')) return `/projects/${projectId}${target}`;
  return `/projects/${projectId}/${target}`;
}

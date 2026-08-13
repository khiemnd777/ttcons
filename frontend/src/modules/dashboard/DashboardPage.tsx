import { useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import {
  AlertCircle,
  AlertTriangle,
  ArrowRight,
  Building2,
  CheckCircle2,
  ClipboardCheck,
  Clock3,
  FolderKanban,
  LockKeyhole,
  Receipt,
  RefreshCw,
  RotateCcw,
  WalletCards,
} from 'lucide-react';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import { PageHeader } from '../../components/PageHeader';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { EnumChip } from '../../components/EnumChip';
import { ApprovalDecisionActions } from '../../components/ApprovalPanel';
import {
  approvalCanDecide,
  approvalRequesterLabel,
  approvalRequiredRolesLabel,
} from '../../components/approvalPresentation';
import type { ApprovalRequest } from '../../lib/types/domain';

type DashboardStatusRow = {
  status: string;
  total: number | string;
};

type DashboardData = {
  total_projects?: number | string;
  active_projects?: number | string;
  blocked_projects?: number | string;
  pending_approvals?: number | string;
  pending_reworks?: number | string;
  receivable_remaining?: number | string;
  payable_remaining?: number | string;
  cashflow_month?: number | string;
  charts?: {
    project_status?: DashboardStatusRow[];
  };
};

type DashboardProject = {
  id: string;
  name: string;
  status: string;
  progress_percent?: number | string;
  open_issues?: number | string;
  pending_approvals?: number | string;
};

type AttentionTone = 'rose' | 'amber' | 'orange';

const attentionToneClasses: Record<AttentionTone, { icon: string; value: string; link: string }> = {
  rose: {
    icon: 'bg-rose-50 text-rose-700',
    value: 'text-rose-700',
    link: 'text-rose-700 hover:text-rose-800',
  },
  amber: {
    icon: 'bg-amber-50 text-amber-800',
    value: 'text-amber-800',
    link: 'text-amber-800 hover:text-amber-900',
  },
  orange: {
    icon: 'bg-orange-50 text-orange-800',
    value: 'text-orange-800',
    link: 'text-orange-800 hover:text-orange-900',
  },
};

export function DashboardPage() {
  const { hasAnyPermission } = useAuth();
  const [showAllApprovals, setShowAllApprovals] = useState(false);
  const canViewProjects = hasAnyPermission(['project.view']);
  const canViewConstructionActions = hasAnyPermission(['operation.view', 'quality.view', 'rework.view', 'replanning.view']);

  const dashboard = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => get<DashboardData>('/reports/dashboard'),
  });
  const projects = useQuery({
    queryKey: ['projects'],
    queryFn: () => get<DashboardProject[]>('/projects'),
    enabled: canViewProjects,
  });
  const approvals = useQuery({
    queryKey: ['approvals', 'pending'],
    queryFn: () => get<ApprovalRequest[]>('/approvals/pending'),
  });

  const attentionProjects = useMemo(
    () => [...(projects.data ?? [])]
      .filter((project) => project.status === 'blocked' || metricNumber(project.open_issues) > 0 || metricNumber(project.pending_approvals) > 0)
      .sort(compareAttentionProjects),
    [projects.data],
  );

  const pendingApprovals = approvals.data ?? [];
  const displayedApprovals = showAllApprovals ? pendingApprovals : pendingApprovals.slice(0, 3);
  const dashboardData = dashboard.data;
  const refreshing = dashboard.isFetching || approvals.isFetching || (canViewProjects && projects.isFetching);

  function refreshDashboard() {
    const refreshes = [dashboard.refetch(), approvals.refetch()];
    if (canViewProjects) refreshes.push(projects.refetch());
    void Promise.all(refreshes);
  }

  return (
    <>
      <PageHeader
        title="Tổng quan điều hành"
        description="Theo dõi việc cần xử lý, sức khỏe công trình và tình hình tài chính trong phạm vi được phép xem."
        actions={(
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={refreshing}
            onClick={refreshDashboard}
          >
            <RefreshCw className={clsx('h-4 w-4', refreshing && 'animate-spin')} />
            {refreshing ? 'Đang làm mới' : 'Làm mới'}
          </button>
        )}
      />

      <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-attention-title">
        <div className="border-b border-line px-4 py-3">
          <h2 id="dashboard-attention-title" className="text-sm font-semibold text-ink">Cần xử lý</h2>
          <p className="mt-1 text-xs text-slate-500">Ưu tiên các điểm đang chặn vận hành hoặc chờ quyết định.</p>
        </div>
        {dashboard.isLoading ? (
          <PanelLoading columns={3} />
        ) : dashboard.isError || !dashboardData ? (
          <PanelState
            kind="error"
            title="Không tải được số liệu cần xử lý"
            action={<RetryButton onClick={() => void dashboard.refetch()} />}
          />
        ) : (
          <div className="grid sm:grid-cols-3 sm:divide-x sm:divide-line">
            <AttentionMetric
              label="Bị chặn"
              value={metricNumber(dashboardData.blocked_projects)}
              unit="công trình"
              icon={<AlertTriangle className="h-5 w-5" />}
              tone="rose"
              action={canViewProjects ? { label: 'Xem công trình', to: '/projects' } : undefined}
            />
            <AttentionMetric
              label="Chờ phê duyệt"
              value={metricNumber(dashboardData.pending_approvals)}
              unit="yêu cầu"
              icon={<ClipboardCheck className="h-5 w-5" />}
              tone="amber"
              action={{ label: 'Xem danh sách', to: '#dashboard-approvals' }}
            />
            <AttentionMetric
              label="Làm lại cần xử lý"
              value={metricNumber(dashboardData.pending_reworks)}
              unit="yêu cầu"
              icon={<RotateCcw className="h-5 w-5" />}
              tone="orange"
              action={canViewConstructionActions ? { label: 'Xem nguồn', to: '/operations?tab=actions' } : undefined}
            />
          </div>
        )}
      </section>

      <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,0.85fr)]">
        <section id="dashboard-approvals" className="scroll-mt-20 overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-approvals-title">
          <PanelHeader
            id="dashboard-approvals-title"
            title="Yêu cầu đang chờ phê duyệt"
            count={approvals.isSuccess ? pendingApprovals.length : undefined}
            description="Trong phạm vi quản lý; mỗi dòng cho biết người gửi và vai trò đang chờ duyệt."
          />
          {approvals.isLoading ? (
            <PanelLoading rows={3} />
          ) : approvals.isError ? (
            <PanelState
              kind="error"
              title="Không tải được các yêu cầu đang chờ phê duyệt"
              action={<RetryButton onClick={() => void approvals.refetch()} />}
            />
          ) : displayedApprovals.length ? (
            <>
              <ul className="divide-y divide-line">
                {displayedApprovals.map((approval) => (
                  <ApprovalItem key={approval.id} approval={approval} canViewProjects={canViewProjects} />
                ))}
              </ul>
              {pendingApprovals.length > 3 ? (
                <div className="border-t border-line bg-slate-50 px-4 py-3 text-right">
                  <button
                    type="button"
                    className="text-xs font-semibold text-blue-700 hover:text-blue-800"
                    onClick={() => setShowAllApprovals((current) => !current)}
                  >
                    {showAllApprovals ? 'Thu gọn' : `Xem thêm ${pendingApprovals.length - 3} yêu cầu`}
                  </button>
                </div>
              ) : null}
            </>
          ) : (
            <PanelState kind="success" title="Không có yêu cầu đang chờ phê duyệt" />
          )}
        </section>

        <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-projects-title">
          <PanelHeader
            id="dashboard-projects-title"
            title="Công trình cần theo dõi"
            count={projects.isSuccess ? attentionProjects.length : undefined}
            description="Bị chặn, có vấn đề mở hoặc đang chờ phê duyệt."
            action={canViewProjects ? <PanelLink to="/projects" label="Tất cả công trình" /> : undefined}
          />
          {!canViewProjects ? (
            <PanelState kind="permission" title="Không có quyền xem danh sách công trình" />
          ) : projects.isLoading ? (
            <PanelLoading rows={3} />
          ) : projects.isError ? (
            <PanelState
              kind="error"
              title="Không tải được danh sách công trình"
              action={<RetryButton onClick={() => void projects.refetch()} />}
            />
          ) : attentionProjects.length ? (
            <>
              <ul className="divide-y divide-line">
                {attentionProjects.slice(0, 5).map((project) => <ProjectAttentionItem key={project.id} project={project} />)}
              </ul>
              {attentionProjects.length > 5 ? (
                <div className="border-t border-line bg-slate-50 px-4 py-3 text-xs font-medium text-slate-500">
                  Hiển thị 5 / {integerText(attentionProjects.length)} công trình cần theo dõi
                </div>
              ) : null}
            </>
          ) : (
            <PanelState kind="success" title="Không có công trình cần chú ý" />
          )}
        </section>
      </div>

      <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,0.85fr)]">
        <ProjectStatusPanel dashboard={dashboard} data={dashboardData} />
        <FinancePanel dashboard={dashboard} data={dashboardData} />
      </div>
    </>
  );
}

function AttentionMetric({
  label,
  value,
  unit,
  icon,
  tone,
  action,
}: {
  label: string;
  value: number;
  unit: string;
  icon: ReactNode;
  tone: AttentionTone;
  action?: { label: string; to: string };
}) {
  const classes = attentionToneClasses[tone];
  const hasItems = value > 0;

  return (
    <div className="border-b border-line px-4 py-4 last:border-b-0 sm:border-b-0">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
          <p className={clsx('mt-2 text-2xl font-semibold', hasItems ? classes.value : 'text-slate-500')}>
            {integerText(value)} <span className="text-sm font-medium text-slate-500">{unit}</span>
          </p>
        </div>
        <div className={clsx('rounded-md p-2', hasItems ? classes.icon : 'bg-slate-100 text-slate-500')}>{icon}</div>
      </div>
      <div className="mt-3 min-h-5 text-xs font-semibold">
        {hasItems && action ? (
          action.to.startsWith('#') ? (
            <a className={clsx('inline-flex items-center gap-1', classes.link)} href={action.to}>
              {action.label} <ArrowRight className="h-3.5 w-3.5" />
            </a>
          ) : (
            <Link className={clsx('inline-flex items-center gap-1', classes.link)} to={action.to}>
              {action.label} <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          )
        ) : (
          <span className="text-slate-500">{hasItems ? 'Xem trong phạm vi được phân quyền' : 'Không có tồn đọng'}</span>
        )}
      </div>
    </div>
  );
}

function ApprovalItem({ approval, canViewProjects }: { approval: ApprovalRequest; canViewProjects: boolean }) {
  const destination = approvalDestination(approval, canViewProjects);
  const canDecide = approvalCanDecide(approval);

  return (
    <li className="px-4 py-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <EnumChip group="target_type" value={approval.target_type} />
            <StatusBadge status={approval.status} />
            {approval.created_at ? (
              <span className="inline-flex items-center gap-1 text-xs text-slate-500">
                <Clock3 className="h-3.5 w-3.5" /> {dateText(approval.created_at)}
              </span>
            ) : null}
          </div>
          <h3 className="mt-2 text-sm font-semibold leading-5 text-ink">{approval.reason || 'Yêu cầu phê duyệt'}</h3>
          <p className="mt-1 truncate text-xs text-slate-500">{approval.project_name || 'Chưa xác định công trình'}</p>
          <dl className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs leading-5">
            <div className="flex min-w-0 gap-1">
              <dt className="shrink-0 text-slate-500">Người gửi:</dt>
              <dd className="break-words font-semibold text-slate-700">{approvalRequesterLabel(approval)}</dd>
            </div>
            <div className="flex min-w-0 gap-1">
              <dt className="shrink-0 text-slate-500">Chờ duyệt:</dt>
              <dd className="break-words font-semibold text-slate-700">{approvalRequiredRolesLabel(approval)}</dd>
            </div>
          </dl>
        </div>
        <div className="flex shrink-0 flex-wrap justify-start gap-2 sm:justify-end">
          {canDecide ? (
            <ApprovalDecisionActions request={approval} compact />
          ) : (
            <span className="inline-flex items-center rounded-md bg-slate-100 px-2.5 py-1.5 text-xs font-semibold text-slate-500">
              Chỉ theo dõi
            </span>
          )}
          {destination ? (
            <Link className="inline-flex items-center justify-center gap-1 rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-100" to={destination.to}>
              {destination.label} <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          ) : null}
        </div>
      </div>
    </li>
  );
}

function ProjectAttentionItem({ project }: { project: DashboardProject }) {
  const progress = clampPercent(project.progress_percent);
  const openIssues = metricNumber(project.open_issues);
  const pendingApprovals = metricNumber(project.pending_approvals);

  return (
    <li className="px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="min-w-0 truncate text-sm font-semibold text-ink">{project.name}</h3>
            <StatusBadge status={project.status} />
          </div>
          <div className="mt-3 flex items-center gap-3">
            <div className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-slate-100" aria-label={`Tiến độ ${progress}%`}>
              <div className="h-full rounded-full bg-blue-600" style={{ width: `${progress}%` }} />
            </div>
            <span className="w-10 text-right text-xs font-semibold text-slate-600">{progress}%</span>
          </div>
          <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-500">
            {project.status === 'blocked' ? <span className="font-medium text-rose-700">Công trình đang bị chặn</span> : null}
            {openIssues > 0 ? <span>{integerText(openIssues)} vấn đề mở</span> : null}
            {pendingApprovals > 0 ? <span>{integerText(pendingApprovals)} chờ phê duyệt</span> : null}
          </div>
        </div>
        <Link
          aria-label={`Mở công trình ${project.name}`}
          className="rounded-md border border-line p-2 text-slate-600 hover:bg-slate-100 hover:text-ink"
          title="Mở công trình"
          to={`/projects/${project.id}`}
        >
          <ArrowRight className="h-4 w-4" />
        </Link>
      </div>
    </li>
  );
}

function ProjectStatusPanel({
  dashboard,
  data,
}: {
  dashboard: { isLoading: boolean; isError: boolean; refetch: () => Promise<unknown> };
  data?: DashboardData;
}) {
  const rows = data?.charts?.project_status ?? [];
  const total = rows.reduce((sum, row) => sum + metricNumber(row.total), 0);
  const completed = rows.find((row) => row.status === 'completed');

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-status-title">
      <PanelHeader id="dashboard-status-title" title="Tình trạng công trình" description="Cơ cấu theo trạng thái hiện tại, không phải phần trăm tiến độ." />
      {dashboard.isLoading ? (
        <PanelLoading rows={4} />
      ) : dashboard.isError || !data ? (
        <PanelState
          kind="error"
          title="Không tải được tình trạng công trình"
          action={<RetryButton onClick={() => void dashboard.refetch()} />}
        />
      ) : rows.length && total > 0 ? (
        <div className="p-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <SummaryValue label="Tổng công trình" value={metricNumber(data.total_projects)} icon={<Building2 className="h-4 w-4" />} />
            <SummaryValue label="Đang triển khai" value={metricNumber(data.active_projects)} icon={<FolderKanban className="h-4 w-4" />} />
            <SummaryValue label="Hoàn tất" value={metricNumber(completed?.total)} icon={<CheckCircle2 className="h-4 w-4" />} />
          </div>
          <div className="mt-5 flex h-3 overflow-hidden rounded-full bg-slate-100" aria-label="Cơ cấu công trình theo trạng thái">
            {rows.map((row) => {
              const percentage = percentageOf(row.total, total);
              return percentage > 0 ? (
                <div
                  key={row.status}
                  className={statusBarClass(row.status)}
                  style={{ width: `${percentage}%` }}
                  title={`${statusLabel(row.status)}: ${percentageText(percentage)}%`}
                />
              ) : null;
            })}
          </div>
          <div className="mt-4 grid gap-x-6 gap-y-3 sm:grid-cols-2">
            {rows.map((row) => (
              <div key={row.status} className="flex items-center justify-between gap-3">
                <StatusBadge status={row.status} />
                <span className="text-xs font-semibold text-slate-600">
                  {integerText(metricNumber(row.total))} · {percentageText(percentageOf(row.total, total))}%
                </span>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <PanelState kind="empty" title="Chưa có dữ liệu trạng thái công trình" />
      )}
    </section>
  );
}

function FinancePanel({
  dashboard,
  data,
}: {
  dashboard: { isLoading: boolean; isError: boolean; refetch: () => Promise<unknown> };
  data?: DashboardData;
}) {
  const cashflow = metricNumber(data?.cashflow_month);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-finance-title">
      <PanelHeader id="dashboard-finance-title" title="Tài chính tổng hợp" description="Công nợ còn lại và dòng tiền theo sổ thanh toán tháng hiện tại." />
      {dashboard.isLoading ? (
        <PanelLoading rows={3} />
      ) : dashboard.isError || !data ? (
        <PanelState
          kind="error"
          title="Không tải được số liệu tài chính"
          action={<RetryButton onClick={() => void dashboard.refetch()} />}
        />
      ) : (
        <div className="divide-y divide-line">
          <FinanceRow label="Phải thu còn lại" value={data.receivable_remaining} icon={<Receipt className="h-4 w-4" />} />
          <FinanceRow label="Phải trả còn lại" value={data.payable_remaining} icon={<WalletCards className="h-4 w-4" />} />
          <FinanceRow
            label="Dòng tiền ròng tháng này"
            value={data.cashflow_month}
            icon={<RefreshCw className="h-4 w-4" />}
            valueClassName={cashflow < 0 ? 'text-rose-700' : 'text-ink'}
          />
        </div>
      )}
    </section>
  );
}

function PanelHeader({
  id,
  title,
  count,
  description,
  action,
}: {
  id?: string;
  title: string;
  count?: number;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-line px-4 py-3">
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h2 id={id} className="text-sm font-semibold text-ink">{title}</h2>
          {count !== undefined ? (
            <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{integerText(count)}</span>
          ) : null}
        </div>
        {description ? <p className="mt-1 text-xs text-slate-500">{description}</p> : null}
      </div>
      {action}
    </div>
  );
}

function PanelLink({ to, label }: { to: string; label: string }) {
  return (
    <Link className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700 hover:text-blue-800" to={to}>
      {label} <ArrowRight className="h-3.5 w-3.5" />
    </Link>
  );
}

function PanelLoading({ rows = 1, columns = 1 }: { rows?: number; columns?: number }) {
  const count = Math.max(rows, columns);
  return (
    <div className={clsx('grid animate-pulse', columns > 1 ? 'divide-y divide-line sm:grid-cols-3 sm:divide-x sm:divide-y-0' : 'divide-y divide-line')}>
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="px-4 py-5">
          <div className="h-3 w-28 rounded bg-slate-100" />
          <div className="mt-3 h-5 w-40 max-w-full rounded bg-slate-100" />
          <div className="mt-2 h-3 w-24 rounded bg-slate-100" />
        </div>
      ))}
    </div>
  );
}

function PanelState({ kind, title, action }: { kind: 'error' | 'success' | 'empty' | 'permission'; title: string; action?: ReactNode }) {
  const icon = kind === 'error'
    ? <AlertCircle className="h-5 w-5 text-rose-600" />
    : kind === 'permission'
      ? <LockKeyhole className="h-5 w-5 text-slate-500" />
      : <CheckCircle2 className={clsx('h-5 w-5', kind === 'success' ? 'text-emerald-600' : 'text-slate-400')} />;

  return (
    <div className="flex min-h-32 flex-col items-center justify-center gap-3 px-4 py-6 text-center">
      {icon}
      <p className="text-sm font-medium text-slate-600">{title}</p>
      {action}
    </div>
  );
}

function RetryButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      className="rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-100"
      onClick={onClick}
    >
      Thử lại
    </button>
  );
}

function SummaryValue({ label, value, icon }: { label: string; value: number; icon: ReactNode }) {
  return (
    <div className="rounded-md bg-slate-50 px-3 py-3">
      <div className="flex items-center gap-2 text-xs font-medium text-slate-500">{icon}{label}</div>
      <p className="mt-2 text-xl font-semibold text-ink">{integerText(value)}</p>
    </div>
  );
}

function FinanceRow({ label, value, icon, valueClassName = 'text-ink' }: { label: string; value: unknown; icon: ReactNode; valueClassName?: string }) {
  return (
    <div className="flex items-center justify-between gap-4 px-4 py-4">
      <div className="flex items-center gap-2 text-sm font-medium text-slate-600">
        <span className="rounded-md bg-slate-100 p-2 text-slate-600">{icon}</span>
        {label}
      </div>
      <span className={clsx('text-right text-base font-semibold', valueClassName)}><CurrencyText value={value} /></span>
    </div>
  );
}

function approvalDestination(approval: ApprovalRequest, canViewProjects: boolean) {
  if (!canViewProjects || !approval.project_id) return null;
  if (approval.target_type === 'work_item' && approval.target_id) {
    return { to: `/projects/${approval.project_id}/work-items/${approval.target_id}`, label: 'Mở nguồn' };
  }
  return { to: `/projects/${approval.project_id}`, label: 'Mở công trình' };
}

function compareAttentionProjects(left: DashboardProject, right: DashboardProject) {
  const blockedDifference = Number(right.status === 'blocked') - Number(left.status === 'blocked');
  if (blockedDifference) return blockedDifference;
  const approvalDifference = metricNumber(right.pending_approvals) - metricNumber(left.pending_approvals);
  if (approvalDifference) return approvalDifference;
  const issueDifference = metricNumber(right.open_issues) - metricNumber(left.open_issues);
  if (issueDifference) return issueDifference;
  const progressDifference = clampPercent(left.progress_percent) - clampPercent(right.progress_percent);
  if (progressDifference) return progressDifference;
  return left.name.localeCompare(right.name, 'vi');
}

function statusBarClass(status: string) {
  if (status === 'blocked') return 'bg-rose-500';
  if (status === 'completed') return 'bg-teal-500';
  if (status === 'in_progress') return 'bg-blue-600';
  if (status === 'planning') return 'bg-sky-400';
  if (status === 'acceptance') return 'bg-emerald-500';
  if (status === 'handover') return 'bg-cyan-500';
  return 'bg-slate-400';
}

function percentageOf(value: unknown, total: number) {
  if (total <= 0) return 0;
  return (metricNumber(value) / total) * 100;
}

function clampPercent(value: unknown) {
  return Math.min(100, Math.max(0, Math.round(metricNumber(value))));
}

function metricNumber(value: unknown) {
  const parsed = Number(value ?? 0);
  return Number.isFinite(parsed) ? parsed : 0;
}

function integerText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 0 }).format(value);
}

function percentageText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 }).format(value);
}

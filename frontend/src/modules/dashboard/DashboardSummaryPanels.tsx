import type { ReactNode } from 'react';
import { AlertCircle, AlertTriangle, Building2, CheckCircle2, ClipboardCheck, FolderKanban, TrendingUp } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { statusLabel } from '../../lib/utils/status';
import type { DashboardProjectSnapshot } from './dashboardTypes';

export function ProjectSnapshotPanel({ snapshot }: { snapshot: DashboardProjectSnapshot }) {
  const total = Math.max(snapshot.total_projects, 0);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-snapshot-title">
      <PanelHeader
        id="dashboard-snapshot-title"
        title="Toàn cảnh công trình"
        description="Trạng thái hiện tại trong phạm vi công trình được phép xem; không thay đổi theo kỳ được chọn."
      />
      <div className="grid gap-px bg-line sm:grid-cols-2 xl:grid-cols-4">
        <SnapshotMetric label="Tổng công trình" value={snapshot.total_projects} icon={<Building2 className="h-4 w-4" />} />
        <SnapshotMetric label="Đang hoạt động" value={snapshot.active_projects} icon={<FolderKanban className="h-4 w-4" />} />
        <SnapshotMetric label="Hoàn tất" value={snapshot.completed_projects} icon={<CheckCircle2 className="h-4 w-4" />} />
        <SnapshotMetric label="Tiến độ bình quân" value={`${decimalText(snapshot.average_progress)}%`} icon={<TrendingUp className="h-4 w-4" />} />
      </div>
      <div className="grid gap-4 p-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(15rem,0.65fr)]">
        <div>
          <div className="flex h-3 overflow-hidden rounded-full bg-slate-100" aria-label="Cơ cấu công trình theo trạng thái">
            {snapshot.status_counts.map((item) => {
              const percentage = total > 0 ? (item.total / total) * 100 : 0;
              return percentage > 0 ? (
                <div
                  key={item.status}
                  className={statusBarClass(item.status)}
                  style={{ width: `${percentage}%` }}
                  title={`${statusLabel(item.status)}: ${decimalText(percentage)}%`}
                />
              ) : null;
            })}
          </div>
          <div className="mt-4 grid gap-x-6 gap-y-3 sm:grid-cols-2">
            {snapshot.status_counts.length ? snapshot.status_counts.map((item) => (
              <div key={item.status} className="flex items-center justify-between gap-3">
                <StatusBadge status={item.status} />
                <span className="text-xs font-semibold text-slate-600">
                  {integerText(item.total)} · {decimalText(total > 0 ? (item.total / total) * 100 : 0)}%
                </span>
              </div>
            )) : <p className="text-sm text-slate-500">Chưa có dữ liệu trạng thái.</p>}
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
          <AttentionMetric label="Công trình bị chặn" value={snapshot.blocked_projects} tone="rose" />
          <AttentionMetric label="Vấn đề đang mở" value={snapshot.open_issues} tone="amber" />
          <AttentionMetric label="Yêu cầu chờ duyệt" value={snapshot.pending_approvals} tone="blue" />
        </div>
      </div>
    </section>
  );
}

export function DashboardLoading() {
  return (
    <div className="space-y-5" role="status" aria-label="Đang tải Dashboard">
      {[0, 1].map((item) => (
        <div key={item} className="animate-pulse rounded-lg border border-line bg-white p-4 shadow-panel">
          <div className="h-4 w-48 rounded bg-slate-200" />
          <div className="mt-4 grid gap-3 sm:grid-cols-4">
            {[0, 1, 2, 3].map((metric) => <div key={metric} className="h-20 rounded bg-slate-100" />)}
          </div>
          <div className="mt-4 h-36 rounded bg-slate-100" />
        </div>
      ))}
    </div>
  );
}

export function DashboardError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="rounded-lg border border-rose-200 bg-rose-50 p-5 text-rose-800">
      <div className="flex items-start gap-3">
        <AlertCircle className="mt-0.5 h-5 w-5 shrink-0" />
        <div>
          <h2 className="font-semibold">Không tải được số liệu Dashboard</h2>
          <p className="mt-1 text-sm">Vui lòng thử lại hoặc kiểm tra kết nối API.</p>
          <button type="button" className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold hover:bg-rose-100" onClick={onRetry}>Thử lại</button>
        </div>
      </div>
    </div>
  );
}

function PanelHeader({ id, title, description }: { id: string; title: string; description: string }) {
  return (
    <div className="border-b border-line px-4 py-3">
      <h2 id={id} className="text-sm font-semibold text-ink">{title}</h2>
      <p className="mt-1 text-xs leading-5 text-slate-500">{description}</p>
    </div>
  );
}

function SnapshotMetric({ label, value, icon }: { label: string; value: number | string; icon: ReactNode }) {
  return (
    <div className="bg-white px-4 py-4">
      <div className="flex items-center gap-2 text-xs font-medium text-slate-500">{icon}{label}</div>
      <p className="mt-2 text-2xl font-semibold text-ink">{typeof value === 'number' ? integerText(value) : value}</p>
    </div>
  );
}

function AttentionMetric({ label, value, tone }: { label: string; value: number; tone: 'rose' | 'amber' | 'blue' }) {
  const classes = tone === 'rose' ? 'bg-rose-50 text-rose-700' : tone === 'amber' ? 'bg-amber-50 text-amber-800' : 'bg-blue-50 text-blue-700';
  const Icon = tone === 'blue' ? ClipboardCheck : AlertTriangle;
  return (
    <div className={`flex items-center justify-between gap-3 rounded-md px-3 py-3 ${classes}`}>
      <div><p className="text-xs font-medium">{label}</p><p className="mt-1 text-lg font-semibold">{integerText(value)}</p></div>
      <Icon className="h-5 w-5" />
    </div>
  );
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

function integerText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 0 }).format(value);
}

function decimalText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 }).format(value);
}

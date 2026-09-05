import type { ReactNode } from 'react';
import { AlertCircle, AlertTriangle, Building2, CheckCircle2, CircleDollarSign, ClipboardCheck, FolderKanban, TrendingDown, TrendingUp } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { statusLabel } from '../../lib/utils/status';
import { dateText as dateLabel } from '../../lib/utils/format';
import type { DashboardCashflowSeries, DashboardData, DashboardProjectSnapshot } from './dashboardTypes';

export function ProjectSnapshotPanel({ snapshot }: { snapshot: DashboardProjectSnapshot }) {
  const total = Math.max(snapshot.total_projects, 0);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-snapshot-title">
      <PanelHeader
        id="dashboard-snapshot-title"
        title="Toàn cảnh công trình"
        description="Trạng thái hiện tại trong phạm vi công trình được phép xem; không thay đổi theo kỳ dòng tiền."
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

export function CashflowPanel({ data }: { data: DashboardData }) {
  const cashflow = data.cashflow;
  if (!data.capabilities.finance_visible || !cashflow) return null;
  const warnings = cashflow.summary.pending_payment_count + cashflow.summary.unclassified_count + cashflow.summary.non_cash_count;

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-labelledby="dashboard-cashflow-title">
      <PanelHeader
        id="dashboard-cashflow-title"
        title="Dòng tiền theo kỳ"
        description={`Từ ${dateLabel(data.meta.from)} đến ${dateLabel(data.meta.to)} · chỉ tổng hợp các khoản thanh toán đã duyệt.`}
      />
      <div className="grid gap-px bg-line sm:grid-cols-2 xl:grid-cols-5">
        <MoneyMetric label="Số dư đầu kỳ" value={cashflow.summary.opening_balance} icon={<CircleDollarSign className="h-4 w-4" />} />
        <MoneyMetric label="Tổng thu" value={cashflow.summary.total_inflow} icon={<TrendingUp className="h-4 w-4" />} tone="positive" />
        <MoneyMetric label="Tổng chi" value={cashflow.summary.total_outflow} icon={<TrendingDown className="h-4 w-4" />} tone="negative" />
        <MoneyMetric label="Dòng tiền ròng" value={cashflow.summary.net_flow} icon={<CircleDollarSign className="h-4 w-4" />} signed />
        <MoneyMetric label="Số dư cuối kỳ" value={cashflow.summary.closing_balance} icon={<CircleDollarSign className="h-4 w-4" />} signed />
      </div>

      <div className="grid gap-5 p-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(18rem,0.6fr)]">
        <div>
          <h3 className="text-sm font-semibold text-ink">Diễn biến thu – chi</h3>
          <p className="mt-1 text-xs text-slate-500">Mỗi dòng biểu diễn tổng thu và tổng chi của một ngày hoặc một tháng trong kỳ.</p>
          <CashflowBars series={cashflow.series} />
        </div>
        <div className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold text-ink">Công trình theo dòng tiền ròng</h3>
            <div className="mt-3 divide-y divide-line rounded-md border border-line">
              {cashflow.projects.length ? cashflow.projects.slice(0, 6).map((project) => (
                <div key={project.project_id} className="flex items-center justify-between gap-4 px-3 py-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-slate-700">{project.project_name}</p>
                    <p className="mt-0.5 text-xs text-slate-500">{integerText(project.payment_count)} khoản đã duyệt</p>
                  </div>
                  <span className={moneyTone(project.net_amount)}><CurrencyText value={project.net_amount} /></span>
                </div>
              )) : <p className="px-3 py-5 text-sm text-slate-500">Chưa có dòng tiền đã duyệt trong kỳ.</p>}
            </div>
          </div>

          {warnings > 0 ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-amber-900">
              <div className="flex items-start gap-2">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="text-sm font-semibold">Dữ liệu cần đối soát</p>
                  <ul className="mt-2 space-y-1 text-xs leading-5">
                    {cashflow.summary.pending_payment_count > 0 ? <li>{integerText(cashflow.summary.pending_payment_count)} khoản chờ duyệt, tổng <CurrencyText value={cashflow.summary.pending_payment_total} />.</li> : null}
                    {cashflow.summary.unclassified_count > 0 ? <li>{integerText(cashflow.summary.unclassified_count)} khoản chưa phân loại được chiều thu/chi.</li> : null}
                    {cashflow.summary.non_cash_count > 0 ? <li>{integerText(cashflow.summary.non_cash_count)} khoản không dùng tiền mặt.</li> : null}
                  </ul>
                </div>
              </div>
            </div>
          ) : null}

          <p className="rounded-md bg-slate-50 p-3 text-xs leading-5 text-slate-500">
            Cơ sở số liệu: sổ thanh toán nội bộ. Dashboard chưa thay thế báo cáo kế toán, dữ liệu MISA hoặc yêu cầu chi chưa được duyệt.
          </p>
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

function CashflowBars({ series }: { series: DashboardCashflowSeries[] }) {
  const maxAmount = Math.max(1, ...series.flatMap((item) => [Math.abs(item.inflow_amount), Math.abs(item.outflow_amount)]));
  if (!series.length) return <p className="mt-4 rounded-md bg-slate-50 p-5 text-sm text-slate-500">Chưa có dòng tiền đã duyệt trong kỳ.</p>;

  return (
    <div className="mt-4 max-h-[26rem] space-y-3 overflow-y-auto pr-1">
      {series.map((item) => (
        <div key={item.period_start} className="grid grid-cols-[5.5rem_minmax(0,1fr)_6.5rem] items-center gap-3 text-xs">
          <span className="font-medium tabular-nums text-slate-500">{dateLabel(item.period_start)}</span>
          <div className="space-y-1.5">
            <div className="h-2 overflow-hidden rounded-full bg-emerald-50"><div className="h-full rounded-full bg-emerald-500" style={{ width: `${Math.max(1, (Math.abs(item.inflow_amount) / maxAmount) * 100)}%` }} /></div>
            <div className="h-2 overflow-hidden rounded-full bg-rose-50"><div className="h-full rounded-full bg-rose-500" style={{ width: `${Math.max(1, (Math.abs(item.outflow_amount) / maxAmount) * 100)}%` }} /></div>
          </div>
          <span className={`text-right font-semibold ${moneyTone(item.net_amount)}`}><CurrencyText value={item.net_amount} /></span>
        </div>
      ))}
      <div className="flex gap-4 border-t border-line pt-3 text-xs text-slate-500">
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-4 rounded bg-emerald-500" /> Thu</span>
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-4 rounded bg-rose-500" /> Chi</span>
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

function MoneyMetric({ label, value, icon, tone, signed = false }: { label: string; value: number; icon: ReactNode; tone?: 'positive' | 'negative'; signed?: boolean }) {
  const valueClass = tone === 'positive' ? 'text-emerald-700' : tone === 'negative' ? 'text-rose-700' : signed ? moneyTone(value) : 'text-ink';
  return (
    <div className="bg-white px-4 py-4">
      <div className="flex items-center gap-2 text-xs font-medium text-slate-500">{icon}{label}</div>
      <p className={`mt-2 text-base font-semibold ${valueClass}`}><CurrencyText value={value} /></p>
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

function moneyTone(value: number) {
  if (value < 0) return 'text-rose-700';
  if (value > 0) return 'text-emerald-700';
  return 'text-slate-600';
}

function integerText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 0 }).format(value);
}

function decimalText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 }).format(value);
}

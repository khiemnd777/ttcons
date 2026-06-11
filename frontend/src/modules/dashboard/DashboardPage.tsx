import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Building2, CheckCircle2, ClipboardCheck, PackageX, Receipt, WalletCards } from 'lucide-react';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { StatCard } from '../../components/StatCard';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { EnumChip } from '../../components/EnumChip';

export function DashboardPage() {
  const dashboard = useQuery({ queryKey: ['dashboard'], queryFn: () => get<Record<string, any>>('/reports/dashboard') });
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const approvals = useQuery({ queryKey: ['approvals', 'pending'], queryFn: () => get<Record<string, any>[]>('/approvals/pending') });

  if (dashboard.isLoading) return <LoadingSpinner />;
  const data = dashboard.data ?? {};
  const charts = data.charts ?? {};

  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Tổng quan công trình, pipeline, vật tư, nghiệm thu, công nợ và các điểm đang chờ quyết định."
      />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <StatCard label="Tổng công trình" value={data.total_projects ?? 0} icon={<Building2 className="h-5 w-5" />} />
        <StatCard label="Đang thi công" value={data.active_projects ?? 0} icon={<CheckCircle2 className="h-5 w-5" />} />
        <StatCard label="Bị block" value={data.blocked_projects ?? 0} icon={<AlertTriangle className="h-5 w-5" />} />
        <StatCard label="Chờ duyệt" value={data.pending_approvals ?? 0} icon={<ClipboardCheck className="h-5 w-5" />} />
        <StatCard label="Rework pending" value={data.pending_reworks ?? 0} icon={<PackageX className="h-5 w-5" />} />
      </div>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        <StatCard label="Công nợ phải thu" value={<CurrencyText value={data.receivable_remaining} />} icon={<Receipt className="h-5 w-5" />} />
        <StatCard label="Công nợ phải trả" value={<CurrencyText value={data.payable_remaining} />} icon={<WalletCards className="h-5 w-5" />} />
        <StatCard label="Dòng tiền tháng này" value={<CurrencyText value={data.cashflow_month} />} />
      </div>

      <div className="mt-5 grid gap-4 xl:grid-cols-2">
        <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
          <h2 className="text-sm font-semibold text-ink">Tiến độ công trình theo trạng thái</h2>
          <div className="mt-4 space-y-3">
            {(charts.project_status ?? []).map((row: Record<string, any>) => (
              <div key={row.status}>
                <div className="mb-1 flex justify-between text-xs text-slate-600">
                  <StatusBadge status={row.status} />
                  <span>{row.total}</span>
                </div>
                <div className="h-2 rounded-full bg-slate-100">
                  <div className="h-2 rounded-full bg-blue-600" style={{ width: `${Math.min(Number(row.total) * 24, 100)}%` }} />
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
          <h2 className="text-sm font-semibold text-ink">Công nợ đầu vào / đầu ra</h2>
          <div className="mt-4 grid gap-3">
            {(charts.debt ?? []).map((row: Record<string, any>) => (
              <div key={row.type} className="flex items-center justify-between rounded-md bg-slate-50 px-3 py-3">
                <span className="text-sm font-medium text-slate-700">{row.type}</span>
                <span className="text-sm font-semibold text-ink"><CurrencyText value={row.amount} /></span>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="mt-5 grid gap-4 xl:grid-cols-2">
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Công trình cần theo dõi</h2>
          <DataTable
            rows={projects.data ?? []}
            columns={[
              { key: 'name', header: 'Công trình', render: (row) => <span className="font-medium text-ink">{row.name}</span> },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              { key: 'progress_percent', header: 'Tiến độ', render: (row) => `${row.progress_percent ?? 0}%` },
            ]}
          />
        </section>
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Approval pending</h2>
          <DataTable
            rows={approvals.data ?? []}
            columns={[
              { key: 'target_type', header: 'Loại', render: (row) => <EnumChip group="target_type" value={row.target_type} /> },
              { key: 'project_name', header: 'Công trình' },
              { key: 'reason', header: 'Lý do' },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            ]}
          />
        </section>
      </div>
    </>
  );
}

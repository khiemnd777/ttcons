import { type ReactNode, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download, RefreshCw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { ProfitCostBreakdownRow, ProfitLedgerRow, ProfitPeriodRow, ProfitProjectRow, ProfitReport, ProfitReportWarning } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function ProfitReportPanel() {
  const [projectId, setProjectId] = useState('');
  const [from, setFrom] = useState(() => firstDayOfMonth());
  const [to, setTo] = useState(() => todayText());
  const [groupBy, setGroupBy] = useState<'month' | 'quarter'>('month');
  const [status, setStatus] = useState<'posted' | 'all'>('posted');
  const toast = useToast();

  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (projectId) params.set('project_id', projectId);
    params.set('from', from);
    params.set('to', to);
    params.set('group_by', groupBy);
    params.set('status', status);
    return params.toString();
  }, [from, groupBy, projectId, status, to]);

  const report = useQuery({
    queryKey: ['profit-report', projectId || 'all', from, to, groupBy, status],
    queryFn: () => get<ProfitReport>(`/reports/profit?${queryString}`),
  });

  async function exportCSV() {
    try {
      const blob = await downloadBlob(`/reports/profit/export?${queryString}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `profit-report-${from}-${to}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất CSV báo cáo lợi nhuận');
    } catch (error) {
      toast.error('Không xuất được CSV báo cáo lợi nhuận', error);
    }
  }

  const summary = report.data?.summary;
  const periods = report.data?.periods ?? [];
  const projects = report.data?.projects ?? [];
  const costBreakdown = report.data?.cost_breakdown ?? [];
  const ledger = report.data?.ledger ?? [];
  const warnings = report.data?.warnings ?? [];

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-ink">Báo cáo lợi nhuận / P&amp;L</h2>
          <p className="mt-1 text-xs text-slate-500">Accrual theo hoá đơn: revenue từ output issued, cost từ input verified, không tính VAT.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 xl:flex xl:flex-wrap xl:items-end xl:justify-end">
          <ProjectField value={projectId} onChange={setProjectId} />
          <DateField label="Từ ngày" value={from} onChange={setFrom} />
          <DateField label="Đến ngày" value={to} onChange={setTo} />
          <SelectField label="Kỳ" value={groupBy} onChange={(value) => setGroupBy(value as 'month' | 'quarter')} options={[['month', 'Tháng'], ['quarter', 'Quý']]} />
          <SelectField label="Status" value={status} onChange={(value) => setStatus(value as 'posted' | 'all')} options={[['posted', 'Posted'], ['all', 'All non-deleted']]} />
          <button
            type="button"
            className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            onClick={() => void report.refetch()}
            disabled={report.isFetching}
          >
            <RefreshCw className={report.isFetching ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
            Tải lại
          </button>
          <button
            type="button"
            className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            onClick={() => void exportCSV()}
            disabled={report.isLoading}
          >
            <Download className="h-4 w-4" />
            CSV
          </button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <MetricTile label="Revenue" value={<CurrencyText value={summary?.revenue_amount ?? 0} />} />
        <MetricTile label="Direct cost" value={<CurrencyText value={summary?.direct_cost_amount ?? 0} />} />
        <MetricTile label="Gross profit" value={<span className={profitClass(summary?.gross_profit)}><CurrencyText value={summary?.gross_profit ?? 0} /></span>} />
        <MetricTile label="Indirect cost" value={<CurrencyText value={summary?.indirect_cost_amount ?? 0} />} />
        <MetricTile label="Net profit" value={<span className={profitClass(summary?.net_profit)}><CurrencyText value={summary?.net_profit ?? 0} /></span>} />
        <MetricTile label="Net margin" value={marginText(summary?.net_margin_percent)} />
      </div>

      {warnings.length ? (
        <div className="grid gap-2 lg:grid-cols-3">
          {warnings.map((warning) => (
            <WarningItem key={warning.code} warning={warning} />
          ))}
        </div>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Theo công trình</h3>
          <DataTable<ProfitProjectRow>
            rows={projects}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có P&L theo công trình trong khoảng lọc"
            pageSize={6}
            defaultSort={{ key: 'net_profit', direction: 'desc' }}
            columns={[
              { key: 'project_name', header: 'Công trình', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.project_name}</span> },
              { key: 'revenue_amount', header: 'Revenue', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
              { key: 'direct_cost_amount', header: 'Direct', align: 'right', render: (row) => <CurrencyText value={row.direct_cost_amount} /> },
              { key: 'indirect_cost_amount', header: 'Indirect', align: 'right', render: (row) => <CurrencyText value={row.indirect_cost_amount} /> },
              { key: 'net_profit', header: 'Net', align: 'right', render: (row) => <span className={profitClass(row.net_profit)}><CurrencyText value={row.net_profit} /></span> },
              { key: 'net_margin_percent', header: 'Margin', align: 'right', render: (row) => marginText(row.net_margin_percent) },
            ]}
          />
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Theo kỳ</h3>
          <DataTable<ProfitPeriodRow>
            rows={periods}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có P&L trong khoảng lọc"
            pageSize={6}
            defaultSort={{ key: 'period_start', direction: 'asc' }}
            columns={[
              { key: 'period_start', header: 'Kỳ', sortable: true, render: (row) => dateText(row.period_start) },
              { key: 'revenue_amount', header: 'Revenue', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
              { key: 'direct_cost_amount', header: 'Direct', align: 'right', render: (row) => <CurrencyText value={row.direct_cost_amount} /> },
              { key: 'indirect_cost_amount', header: 'Indirect', align: 'right', render: (row) => <CurrencyText value={row.indirect_cost_amount} /> },
              { key: 'net_profit', header: 'Net', align: 'right', render: (row) => <span className={profitClass(row.net_profit)}><CurrencyText value={row.net_profit} /></span> },
            ]}
          />
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Cost breakdown</h3>
          <DataTable<ProfitCostBreakdownRow>
            rows={costBreakdown}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có cost trong khoảng lọc"
            pageSize={6}
            columns={[
              { key: 'cost_type', header: 'Loại', render: (row) => <CostTypePill costType={row.cost_type} /> },
              { key: 'cost_group', header: 'Nhóm', render: (row) => <span className="font-semibold text-ink">{row.cost_group}</span> },
              { key: 'classification_reason', header: 'Nguồn phân loại' },
              { key: 'invoice_count', header: 'HĐ', align: 'right' },
              { key: 'cost_amount', header: 'Cost', align: 'right', render: (row) => <CurrencyText value={row.cost_amount} /> },
            ]}
          />
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Nguồn chứng từ</h3>
          <DataTable<ProfitLedgerRow>
            rows={ledger}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có chứng từ P&L trong khoảng lọc"
            searchPlaceholder="Tìm số hoá đơn, công trình, đối tác..."
            pageSize={8}
            defaultSort={{ key: 'invoice_date', direction: 'desc' }}
            columns={[
              { key: 'invoice_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.invoice_date) },
              { key: 'invoice_number', header: 'Số HĐ', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.invoice_number}</span> },
              { key: 'direction', header: 'Dòng', render: (row) => <DirectionPill direction={row.direction} /> },
              { key: 'cost_type', header: 'P&L', render: (row) => <CostTypePill costType={row.cost_type} /> },
              { key: 'project_name', header: 'Công trình' },
              { key: 'counterparty_name', header: 'Đối tác' },
              { key: 'revenue_amount', header: 'Revenue', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
              { key: 'direct_cost_amount', header: 'Direct', align: 'right', render: (row) => <CurrencyText value={row.direct_cost_amount} /> },
              { key: 'indirect_cost_amount', header: 'Indirect', align: 'right', render: (row) => <CurrencyText value={row.indirect_cost_amount} /> },
              { key: 'status', header: 'Status', render: (row) => <StatusBadge status={row.status} /> },
              { key: 'source_code', header: 'Source' },
              { key: 'warning_flags', header: 'Flags', render: (row) => row.warning_flags || '-' },
            ]}
          />
        </div>
      </div>
    </section>
  );
}

function ProjectField({ value, onChange }: { value?: string; onChange: (value: string) => void }) {
  return (
    <label className="block min-w-44">
      <span className="text-xs font-semibold text-slate-600">Công trình</span>
      <div className="mt-1">
        <ProjectSelector value={value} onChange={onChange} placeholder="Tất cả công trình" />
      </div>
    </label>
  );
}

function DateField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="block min-w-36">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <input
        type="date"
        className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: [string, string][] }) {
  return (
    <label className="block min-w-32">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <select
        className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map(([optionValue, labelText]) => <option key={optionValue} value={optionValue}>{labelText}</option>)}
      </select>
    </label>
  );
}

function MetricTile({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold uppercase text-slate-500">{label}</p>
      <div className="mt-1 truncate text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function WarningItem({ warning }: { warning: ProfitReportWarning }) {
  const tone = warning.severity === 'info' ? 'border-blue-100 bg-blue-50 text-blue-700' : 'border-amber-100 bg-amber-50 text-amber-800';
  return (
    <div className={`rounded-md border px-3 py-2 text-xs ${tone}`}>
      <p className="font-semibold">{warning.message}</p>
      <p className="mt-1">{warning.count ? `${Number(warning.count)} dòng` : ''}{warning.amount ? ` / ${Number(warning.amount).toLocaleString('vi-VN')} VND` : ''}</p>
    </div>
  );
}

function DirectionPill({ direction }: { direction?: string }) {
  const base = 'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold';
  if (direction === 'revenue') return <span className={`${base} bg-blue-50 text-blue-700`}>Revenue</span>;
  if (direction === 'cost') return <span className={`${base} bg-rose-50 text-rose-700`}>Cost</span>;
  return <span className={`${base} bg-slate-100 text-slate-600`}>{direction || '-'}</span>;
}

function CostTypePill({ costType }: { costType?: string }) {
  const base = 'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold';
  if (costType === 'direct') return <span className={`${base} bg-emerald-50 text-emerald-700`}>Direct</span>;
  if (costType === 'indirect') return <span className={`${base} bg-amber-50 text-amber-800`}>Indirect</span>;
  if (costType === 'revenue') return <span className={`${base} bg-blue-50 text-blue-700`}>Revenue</span>;
  return <span className={`${base} bg-slate-100 text-slate-600`}>{costType || '-'}</span>;
}

function profitClass(value?: number) {
  const amount = Number(value ?? 0);
  if (amount > 0) return 'text-emerald-700';
  if (amount < 0) return 'text-rose-700';
  return 'text-ink';
}

function marginText(value?: number) {
  const amount = Number(value ?? 0);
  if (!Number.isFinite(amount)) return '0%';
  return `${amount.toLocaleString('vi-VN', { maximumFractionDigits: 1 })}%`;
}

function todayText() {
  return dateInputText(new Date());
}

function firstDayOfMonth() {
  const now = new Date();
  return dateInputText(new Date(now.getFullYear(), now.getMonth(), 1));
}

function dateInputText(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

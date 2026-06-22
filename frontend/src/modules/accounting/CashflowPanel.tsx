import { type ReactNode, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download, RefreshCw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { CashflowCategoryRow, CashflowLedgerRow, CashflowReport, CashflowSeriesRow } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';

type CashflowPanelProps = {
  projectId?: string;
};

export function CashflowPanel({ projectId }: CashflowPanelProps) {
  const [from, setFrom] = useState(() => firstDayOfMonth());
  const [to, setTo] = useState(() => todayText());
  const [groupBy, setGroupBy] = useState<'day' | 'week' | 'month'>('day');
  const [status, setStatus] = useState<'approved' | 'all'>('approved');
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
    queryKey: ['cashflow-report', projectId ?? 'all', from, to, groupBy, status],
    queryFn: () => get<CashflowReport>(`/reports/cashflow?${queryString}`),
  });

  async function exportCSV() {
    try {
      const blob = await downloadBlob(`/reports/cashflow/export?${queryString}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `cashflow-${from}-${to}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất CSV dòng tiền');
    } catch (error) {
      toast.error('Không xuất được CSV dòng tiền', error);
    }
  }

  const summary = report.data?.summary;
  const series = report.data?.series ?? [];
  const categories = report.data?.categories ?? [];
  const ledger = report.data?.ledger ?? [];

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-ink">Dòng tiền thu/chi</h2>
          <p className="mt-1 text-xs text-slate-500">Cash-basis theo payment đã duyệt; phiếu chờ duyệt được tách riêng trong summary.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 lg:flex lg:flex-wrap lg:items-end lg:justify-end">
          <DateField label="Từ ngày" value={from} onChange={setFrom} />
          <DateField label="Đến ngày" value={to} onChange={setTo} />
          <SelectField label="Nhóm" value={groupBy} onChange={(value) => setGroupBy(value as 'day' | 'week' | 'month')} options={[['day', 'Ngày'], ['week', 'Tuần'], ['month', 'Tháng']]} />
          <SelectField label="Status" value={status} onChange={(value) => setStatus(value as 'approved' | 'all')} options={[['approved', 'Approved'], ['all', 'All non-cancelled']]} />
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
        <MetricTile label="Đầu kỳ" value={<CurrencyText value={summary?.opening_balance ?? 0} />} />
        <MetricTile label="Tổng thu" value={<CurrencyText value={summary?.total_inflow ?? 0} />} />
        <MetricTile label="Tổng chi" value={<CurrencyText value={summary?.total_outflow ?? 0} />} />
        <MetricTile label="Net flow" value={<CurrencyText value={summary?.net_flow ?? 0} />} />
        <MetricTile label="Cuối kỳ" value={<CurrencyText value={summary?.closing_balance ?? 0} />} />
        <MetricTile label="Chờ duyệt" value={`${Number(summary?.pending_payment_count ?? 0)} phiếu`} />
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Diễn biến theo thời gian</h3>
          <DataTable<CashflowSeriesRow>
            rows={series}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có dòng tiền trong khoảng lọc"
            pageSize={6}
            defaultSort={{ key: 'period_start', direction: 'asc' }}
            columns={[
              { key: 'period_start', header: 'Kỳ', sortable: true, render: (row) => dateText(row.period_start) },
              { key: 'inflow_amount', header: 'Thu', align: 'right', render: (row) => <CurrencyText value={row.inflow_amount} /> },
              { key: 'outflow_amount', header: 'Chi', align: 'right', render: (row) => <CurrencyText value={row.outflow_amount} /> },
              { key: 'net_amount', header: 'Net', align: 'right', render: (row) => <CurrencyText value={row.net_amount} /> },
              { key: 'running_balance', header: 'Luỹ kế', align: 'right', render: (row) => <CurrencyText value={row.running_balance} /> },
            ]}
          />
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Theo category</h3>
          <DataTable<CashflowCategoryRow>
            rows={categories}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có category dòng tiền"
            pageSize={6}
            columns={[
              { key: 'category', header: 'Category', render: (row) => <span className="font-semibold text-ink">{row.category}</span> },
              { key: 'direction', header: 'Hướng', render: (row) => <span className={directionClass(row.direction)}>{directionLabel(row.direction)}</span> },
              { key: 'payment_count', header: 'Phiếu', align: 'right' },
              { key: 'inflow_amount', header: 'Thu', align: 'right', render: (row) => <CurrencyText value={row.inflow_amount} /> },
              { key: 'outflow_amount', header: 'Chi', align: 'right', render: (row) => <CurrencyText value={row.outflow_amount} /> },
            ]}
          />
        </div>
      </div>

      <div className="space-y-2">
        <h3 className="text-xs font-semibold uppercase text-slate-500">Ledger</h3>
        <DataTable<CashflowLedgerRow>
          rows={ledger}
          loading={report.isLoading}
          error={report.error}
          onRetry={() => void report.refetch()}
          empty="Chưa có dòng tiền trong khoảng lọc"
          searchPlaceholder="Tìm mã phiếu, công trình, category..."
          pageSize={10}
          defaultSort={{ key: 'occurred_at', direction: 'desc' }}
          columns={[
            { key: 'occurred_at', header: 'Ngày', sortable: true, render: (row) => dateText(row.occurred_at) },
            { key: 'payment_code', header: 'Mã phiếu', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.payment_code}</span> },
            { key: 'project_name', header: 'Công trình' },
            { key: 'category', header: 'Category' },
            { key: 'payment_type', header: 'Loại', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
            { key: 'payment_method', header: 'Method', render: (row) => <EnumChip group="payment_method" value={row.payment_method} /> },
            { key: 'status', header: 'Status', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'inflow_amount', header: 'Thu', align: 'right', render: (row) => <CurrencyText value={row.inflow_amount} /> },
            { key: 'outflow_amount', header: 'Chi', align: 'right', render: (row) => <CurrencyText value={row.outflow_amount} /> },
            { key: 'target_type', header: 'Target' },
          ]}
        />
      </div>
    </section>
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

function directionLabel(direction?: string) {
  if (direction === 'inflow') return 'Thu';
  if (direction === 'outflow') return 'Chi';
  if (direction === 'non_cash') return 'Không cash';
  return 'Unclassified';
}

function directionClass(direction?: string) {
  const base = 'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold';
  if (direction === 'inflow') return `${base} bg-emerald-50 text-emerald-700`;
  if (direction === 'outflow') return `${base} bg-rose-50 text-rose-700`;
  if (direction === 'non_cash') return `${base} bg-blue-50 text-blue-700`;
  return `${base} bg-slate-100 text-slate-600`;
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

import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { CashflowCategoryRow, CashflowLedgerRow, CashflowReport, CashflowSeriesRow, ReportProjectOption } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ReportDateField, ReportFilterActions, ReportProjectField, ReportSelectField } from '../reports/ReportFilters';

type CashflowPanelProps = {
  projectId?: string;
  initialFrom?: string;
  reportMode?: boolean;
  projectOptions?: ReportProjectOption[];
};

type CashflowFilters = {
  projectId: string;
  from: string;
  to: string;
  groupBy: 'day' | 'week' | 'month';
  status: 'approved' | 'all';
};

export function CashflowPanel({ projectId, initialFrom, reportMode = false, projectOptions = [] }: CashflowPanelProps) {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialFilters = useMemo(
    () => cashflowFiltersFromParams(reportMode ? searchParams : new URLSearchParams(), projectId, initialFrom),
    [],
  );
  const [filters, setFilters] = useState<CashflowFilters>(initialFilters);
  const [appliedFilters, setAppliedFilters] = useState<CashflowFilters>(initialFilters);
  const [exporting, setExporting] = useState(false);
  const toast = useToast();

  useEffect(() => {
    const nextFilters = cashflowFiltersFromParams(reportMode ? searchParams : new URLSearchParams(), projectId, initialFrom);
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
  }, [initialFrom, projectId, reportMode, searchParams]);

  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (appliedFilters.projectId) params.set('project_id', appliedFilters.projectId);
    params.set('from', appliedFilters.from);
    params.set('to', appliedFilters.to);
    params.set('group_by', appliedFilters.groupBy);
    params.set('status', appliedFilters.status);
    return params.toString();
  }, [appliedFilters]);

  const report = useQuery({
    queryKey: ['cashflow-report', appliedFilters],
    queryFn: () => get<CashflowReport>(`/reports/cashflow?${queryString}`),
  });

  function applyFilters() {
    setAppliedFilters(filters);
    if (reportMode) setSearchParams(cashflowFiltersToParams(filters), { replace: true });
  }

  function resetFilters() {
    const nextFilters = cashflowFiltersFromParams(new URLSearchParams(), projectId, initialFrom);
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
    if (reportMode) setSearchParams(cashflowFiltersToParams(nextFilters), { replace: true });
  }

  async function exportCSV() {
    setExporting(true);
    try {
      const blob = await downloadBlob(`/reports/cashflow/export?${queryString}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `dong-tien-${appliedFilters.from}-${appliedFilters.to}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất CSV dòng tiền');
    } catch (error) {
      toast.error('Không xuất được CSV dòng tiền', error);
    } finally {
      setExporting(false);
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
          <p className="mt-1 text-xs text-slate-500">Cơ sở tiền theo phiếu đã duyệt; phiếu chờ duyệt, chưa phân loại và không tiền mặt được tách riêng.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 lg:flex lg:flex-wrap lg:items-end lg:justify-end">
          {reportMode ? <ReportProjectField value={filters.projectId} options={projectOptions} onChange={(nextProjectId) => setFilters((current) => ({ ...current, projectId: nextProjectId }))} /> : null}
          <ReportDateField label="Từ ngày" value={filters.from} onChange={(from) => setFilters((current) => ({ ...current, from }))} />
          <ReportDateField label="Đến ngày" value={filters.to} onChange={(to) => setFilters((current) => ({ ...current, to }))} />
          <ReportSelectField label="Nhóm" value={filters.groupBy} onChange={(groupBy) => setFilters((current) => ({ ...current, groupBy: groupBy as CashflowFilters['groupBy'] }))} options={[['day', 'Ngày'], ['week', 'Tuần'], ['month', 'Tháng']]} />
          <ReportSelectField label="Trạng thái" value={filters.status} onChange={(status) => setFilters((current) => ({ ...current, status: status as CashflowFilters['status'] }))} options={[['approved', 'Đã duyệt'], ['all', 'Tất cả chưa hủy']]} />
          <ReportFilterActions applying={report.isFetching} exporting={exporting || report.isLoading} onApply={applyFilters} onReset={resetFilters} onExport={() => void exportCSV()} />
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <MetricTile label="Đầu kỳ" value={<CurrencyText value={summary?.opening_balance ?? 0} />} />
        <MetricTile label="Tổng thu" value={<CurrencyText value={summary?.total_inflow ?? 0} />} />
        <MetricTile label="Tổng chi" value={<CurrencyText value={summary?.total_outflow ?? 0} />} />
        <MetricTile label="Dòng tiền thuần" value={<CurrencyText value={summary?.net_flow ?? 0} />} />
        <MetricTile label="Cuối kỳ" value={<CurrencyText value={summary?.closing_balance ?? 0} />} />
        <MetricTile label="Tổng phiếu" value={`${Number(summary?.payment_count ?? 0)} phiếu`} />
        <MetricTile label="Chờ duyệt" value={<><span>{Number(summary?.pending_payment_count ?? 0)} phiếu · </span><CurrencyText value={summary?.pending_payment_total ?? 0} /></>} />
        <MetricTile label="Chưa phân loại" value={`${Number(summary?.unclassified_count ?? 0)} phiếu`} />
        <MetricTile label="Không tiền mặt" value={`${Number(summary?.non_cash_count ?? 0)} phiếu`} />
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
              { key: 'net_amount', header: 'Thuần', align: 'right', render: (row) => <CurrencyText value={row.net_amount} /> },
              { key: 'running_balance', header: 'Luỹ kế', align: 'right', render: (row) => <CurrencyText value={row.running_balance} /> },
            ]}
          />
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Theo nhóm</h3>
          <DataTable<CashflowCategoryRow>
            rows={categories}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có nhóm dòng tiền"
            pageSize={6}
            columns={[
              { key: 'category', header: 'Nhóm', render: (row) => <span className="font-semibold text-ink">{row.category}</span> },
              { key: 'direction', header: 'Hướng', render: (row) => <span className={directionClass(row.direction)}>{directionLabel(row.direction)}</span> },
              { key: 'payment_count', header: 'Phiếu', align: 'right' },
              { key: 'inflow_amount', header: 'Thu', align: 'right', render: (row) => <CurrencyText value={row.inflow_amount} /> },
              { key: 'outflow_amount', header: 'Chi', align: 'right', render: (row) => <CurrencyText value={row.outflow_amount} /> },
            ]}
          />
        </div>
      </div>

      <div className="space-y-2">
        <h3 className="text-xs font-semibold uppercase text-slate-500">Sổ dòng tiền</h3>
        <DataTable<CashflowLedgerRow>
          rows={ledger}
          loading={report.isLoading}
          error={report.error}
          onRetry={() => void report.refetch()}
          empty="Chưa có dòng tiền trong khoảng lọc"
          searchPlaceholder="Tìm mã phiếu, công trình, nhóm..."
          pageSize={10}
          defaultSort={{ key: 'occurred_at', direction: 'desc' }}
          columns={[
            { key: 'occurred_at', header: 'Ngày', sortable: true, render: (row) => dateText(row.occurred_at) },
            { key: 'payment_code', header: 'Mã phiếu', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.payment_code}</span> },
            { key: 'project_name', header: 'Công trình' },
            { key: 'category', header: 'Nhóm' },
            { key: 'payment_type', header: 'Loại', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
            { key: 'payment_method', header: 'Phương thức', render: (row) => <EnumChip group="payment_method" value={row.payment_method} /> },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'inflow_amount', header: 'Thu', align: 'right', render: (row) => <CurrencyText value={row.inflow_amount} /> },
            { key: 'outflow_amount', header: 'Chi', align: 'right', render: (row) => <CurrencyText value={row.outflow_amount} /> },
            { key: 'target_type', header: 'Nguồn liên kết' },
          ]}
        />
      </div>
    </section>
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
  if (direction === 'non_cash') return 'Không tiền mặt';
  return 'Chưa phân loại';
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

function cashflowFiltersFromParams(params: URLSearchParams, fixedProjectId?: string, initialFrom?: string): CashflowFilters {
  const groupBy = params.get('group_by');
  const status = params.get('status');
  return {
    projectId: fixedProjectId || params.get('project_id') || '',
    from: params.get('from') || initialFrom || firstDayOfMonth(),
    to: params.get('to') || todayText(),
    groupBy: groupBy === 'week' || groupBy === 'month' ? groupBy : 'day',
    status: status === 'all' ? 'all' : 'approved',
  };
}

function cashflowFiltersToParams(filters: CashflowFilters) {
  const params = new URLSearchParams();
  if (filters.projectId) params.set('project_id', filters.projectId);
  params.set('from', filters.from);
  params.set('to', filters.to);
  params.set('group_by', filters.groupBy);
  params.set('status', filters.status);
  return params;
}

function dateInputText(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

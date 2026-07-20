import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { ProfitCostBreakdownRow, ProfitLedgerRow, ProfitPeriodRow, ProfitProjectRow, ProfitReport, ProfitReportWarning, ReportProjectOption } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ReportDateField, ReportFilterActions, ReportProjectField, ReportSelectField } from './ReportFilters';

type ProfitFilters = {
  projectId: string;
  from: string;
  to: string;
  groupBy: 'month' | 'quarter';
  status: 'posted' | 'all';
};

export function ProfitReportPanel({ projectOptions = [] }: { projectOptions?: ReportProjectOption[] }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialFilters = useMemo(() => profitFiltersFromParams(searchParams), []);
  const [filters, setFilters] = useState<ProfitFilters>(initialFilters);
  const [appliedFilters, setAppliedFilters] = useState<ProfitFilters>(initialFilters);
  const [ledgerQuery, setLedgerQuery] = useState('');
  const [exporting, setExporting] = useState(false);
  const toast = useToast();

  useEffect(() => {
    const nextFilters = profitFiltersFromParams(searchParams);
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
  }, [searchParams]);

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
    queryKey: ['profit-report', appliedFilters],
    queryFn: () => get<ProfitReport>(`/reports/profit?${queryString}`),
  });

  function applyFilters() {
    setAppliedFilters(filters);
    setSearchParams(profitFiltersToParams(filters), { replace: true });
  }

  function resetFilters() {
    const nextFilters = defaultProfitFilters();
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
    setSearchParams(profitFiltersToParams(nextFilters), { replace: true });
  }

  async function exportCSV() {
    setExporting(true);
    try {
      const blob = await downloadBlob(`/reports/profit/export?${queryString}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `bao-cao-loi-nhuan-${appliedFilters.from}-${appliedFilters.to}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất CSV báo cáo lợi nhuận');
    } catch (error) {
      toast.error('Không xuất được CSV báo cáo lợi nhuận', error);
    } finally {
      setExporting(false);
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
          <p className="mt-1 text-xs text-slate-500">Dồn tích theo hóa đơn: doanh thu từ đầu ra đã phát hành, chi phí từ đầu vào đã xác minh, không gồm VAT.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 xl:flex xl:flex-wrap xl:items-end xl:justify-end">
          <ReportProjectField value={filters.projectId} options={projectOptions} onChange={(projectId) => setFilters((current) => ({ ...current, projectId }))} />
          <ReportDateField label="Từ ngày" value={filters.from} onChange={(from) => setFilters((current) => ({ ...current, from }))} />
          <ReportDateField label="Đến ngày" value={filters.to} onChange={(to) => setFilters((current) => ({ ...current, to }))} />
          <ReportSelectField label="Kỳ" value={filters.groupBy} onChange={(groupBy) => setFilters((current) => ({ ...current, groupBy: groupBy as ProfitFilters['groupBy'] }))} options={[['month', 'Tháng'], ['quarter', 'Quý']]} />
          <ReportSelectField label="Trạng thái" value={filters.status} onChange={(status) => setFilters((current) => ({ ...current, status: status as ProfitFilters['status'] }))} options={[['posted', 'Đã ghi nhận'], ['all', 'Tất cả chưa xóa']]} />
          <ReportFilterActions applying={report.isFetching} exporting={exporting || report.isLoading} onApply={applyFilters} onReset={resetFilters} onExport={() => void exportCSV()} />
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <MetricTile label="Doanh thu" value={<CurrencyText value={summary?.revenue_amount ?? 0} />} />
        <MetricTile label="Chi phí trực tiếp" value={<CurrencyText value={summary?.direct_cost_amount ?? 0} />} />
        <MetricTile label="Lợi nhuận gộp" value={<span className={profitClass(summary?.gross_profit)}><CurrencyText value={summary?.gross_profit ?? 0} /></span>} />
        <MetricTile label="Chi phí gián tiếp" value={<CurrencyText value={summary?.indirect_cost_amount ?? 0} />} />
        <MetricTile label="Lợi nhuận ròng" value={<span className={profitClass(summary?.net_profit)}><CurrencyText value={summary?.net_profit ?? 0} /></span>} />
        <MetricTile label="Biên lợi nhuận" value={marginText(summary?.net_margin_percent)} />
      </div>

      {warnings.length ? (
        <div className="grid gap-2 lg:grid-cols-3">
          {warnings.map((warning) => (
            <WarningItem
              key={warning.code}
              warning={warning}
              onFilter={warning.code === 'indirect_or_unclassified_cost' ? () => setLedgerQuery(warning.code) : undefined}
            />
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
              { key: 'revenue_amount', header: 'Doanh thu', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
              { key: 'direct_cost_amount', header: 'Chi phí trực tiếp', align: 'right', render: (row) => <CurrencyText value={row.direct_cost_amount} /> },
              { key: 'indirect_cost_amount', header: 'Chi phí gián tiếp', align: 'right', render: (row) => <CurrencyText value={row.indirect_cost_amount} /> },
              { key: 'net_profit', header: 'Lợi nhuận ròng', align: 'right', render: (row) => <span className={profitClass(row.net_profit)}><CurrencyText value={row.net_profit} /></span> },
              { key: 'net_margin_percent', header: 'Biên lợi nhuận', align: 'right', render: (row) => marginText(row.net_margin_percent) },
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
              { key: 'revenue_amount', header: 'Doanh thu', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
              { key: 'direct_cost_amount', header: 'Chi phí trực tiếp', align: 'right', render: (row) => <CurrencyText value={row.direct_cost_amount} /> },
              { key: 'indirect_cost_amount', header: 'Chi phí gián tiếp', align: 'right', render: (row) => <CurrencyText value={row.indirect_cost_amount} /> },
              { key: 'net_profit', header: 'Lợi nhuận ròng', align: 'right', render: (row) => <span className={profitClass(row.net_profit)}><CurrencyText value={row.net_profit} /></span> },
            ]}
          />
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Cơ cấu chi phí</h3>
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
              { key: 'cost_amount', header: 'Chi phí', align: 'right', render: (row) => <CurrencyText value={row.cost_amount} /> },
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
            searchQuery={ledgerQuery}
            onSearchQueryChange={setLedgerQuery}
            pageSize={8}
            defaultSort={{ key: 'invoice_date', direction: 'desc' }}
            columns={[
              { key: 'invoice_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.invoice_date) },
              { key: 'invoice_number', header: 'Số HĐ', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.invoice_number}</span> },
              { key: 'direction', header: 'Dòng', render: (row) => <DirectionPill direction={row.direction} /> },
              { key: 'cost_type', header: 'P&L', render: (row) => <CostTypePill costType={row.cost_type} /> },
              { key: 'project_name', header: 'Công trình' },
              { key: 'counterparty_name', header: 'Đối tác' },
              { key: 'revenue_amount', header: 'Doanh thu', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
              { key: 'direct_cost_amount', header: 'Chi phí trực tiếp', align: 'right', render: (row) => <CurrencyText value={row.direct_cost_amount} /> },
              { key: 'indirect_cost_amount', header: 'Chi phí gián tiếp', align: 'right', render: (row) => <CurrencyText value={row.indirect_cost_amount} /> },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              { key: 'source_code', header: 'Nguồn' },
              { key: 'warning_flags', header: 'Cảnh báo', render: (row) => row.warning_flags || '-' },
            ]}
          />
        </div>
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

function WarningItem({ warning, onFilter }: { warning: ProfitReportWarning; onFilter?: () => void }) {
  const tone = warning.severity === 'info' ? 'border-blue-100 bg-blue-50 text-blue-700' : 'border-amber-100 bg-amber-50 text-amber-800';
  const content = (
    <>
      <p className="font-semibold">{warning.message}</p>
      <p className="mt-1">{warning.count ? `${Number(warning.count)} dòng` : ''}{warning.amount ? ` / ${Number(warning.amount).toLocaleString('vi-VN')} VND` : ''}</p>
      {onFilter ? <p className="mt-1 font-semibold underline">Lọc sổ chứng từ</p> : null}
    </>
  );
  if (onFilter) {
    return (
      <button type="button" className={`rounded-md border px-3 py-2 text-left text-xs ${tone}`} onClick={onFilter}>
        {content}
      </button>
    );
  }
  return (
    <div className={`rounded-md border px-3 py-2 text-xs ${tone}`}>
      {content}
    </div>
  );
}

function DirectionPill({ direction }: { direction?: string }) {
  const base = 'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold';
  if (direction === 'revenue') return <span className={`${base} bg-blue-50 text-blue-700`}>Doanh thu</span>;
  if (direction === 'cost') return <span className={`${base} bg-rose-50 text-rose-700`}>Chi phí</span>;
  return <span className={`${base} bg-slate-100 text-slate-600`}>{direction || '-'}</span>;
}

function CostTypePill({ costType }: { costType?: string }) {
  const base = 'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold';
  if (costType === 'direct') return <span className={`${base} bg-emerald-50 text-emerald-700`}>Trực tiếp</span>;
  if (costType === 'indirect') return <span className={`${base} bg-amber-50 text-amber-800`}>Gián tiếp</span>;
  if (costType === 'revenue') return <span className={`${base} bg-blue-50 text-blue-700`}>Doanh thu</span>;
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

function defaultProfitFilters(): ProfitFilters {
  return {
    projectId: '',
    from: firstDayOfMonth(),
    to: todayText(),
    groupBy: 'month',
    status: 'posted',
  };
}

function profitFiltersFromParams(params: URLSearchParams): ProfitFilters {
  const defaults = defaultProfitFilters();
  const groupBy = params.get('group_by');
  const status = params.get('status');
  return {
    projectId: params.get('project_id') ?? '',
    from: params.get('from') ?? defaults.from,
    to: params.get('to') ?? defaults.to,
    groupBy: groupBy === 'quarter' ? 'quarter' : 'month',
    status: status === 'all' ? 'all' : 'posted',
  };
}

function profitFiltersToParams(filters: ProfitFilters) {
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

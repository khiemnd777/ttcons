import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { ReportProjectOption, TaxCounterpartyRow, TaxLedgerRow, TaxPeriodRow, TaxReport, TaxReportWarning } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ReportDateField, ReportFilterActions, ReportProjectField, ReportSelectField } from './ReportFilters';

type TaxFilters = {
  projectId: string;
  from: string;
  to: string;
  groupBy: 'month' | 'quarter';
  source: 'all' | 'input' | 'output';
  status: 'posted' | 'all';
};

export function TaxReportPanel({ projectOptions = [] }: { projectOptions?: ReportProjectOption[] }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialFilters = useMemo(() => taxFiltersFromParams(searchParams), []);
  const [filters, setFilters] = useState<TaxFilters>(initialFilters);
  const [appliedFilters, setAppliedFilters] = useState<TaxFilters>(initialFilters);
  const [ledgerQuery, setLedgerQuery] = useState('');
  const [exporting, setExporting] = useState(false);
  const toast = useToast();

  useEffect(() => {
    const nextFilters = taxFiltersFromParams(searchParams);
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
  }, [searchParams]);

  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (appliedFilters.projectId) params.set('project_id', appliedFilters.projectId);
    params.set('from', appliedFilters.from);
    params.set('to', appliedFilters.to);
    params.set('group_by', appliedFilters.groupBy);
    params.set('source', appliedFilters.source);
    params.set('status', appliedFilters.status);
    return params.toString();
  }, [appliedFilters]);

  const report = useQuery({
    queryKey: ['tax-report', appliedFilters],
    queryFn: () => get<TaxReport>(`/reports/tax?${queryString}`),
  });

  function applyFilters() {
    setAppliedFilters(filters);
    setSearchParams(taxFiltersToParams(filters), { replace: true });
  }

  function resetFilters() {
    const nextFilters = defaultTaxFilters();
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
    setSearchParams(taxFiltersToParams(nextFilters), { replace: true });
  }

  async function exportCSV() {
    setExporting(true);
    try {
      const blob = await downloadBlob(`/reports/tax/export?${queryString}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `bao-cao-thue-${appliedFilters.from}-${appliedFilters.to}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất CSV báo cáo thuế');
    } catch (error) {
      toast.error('Không xuất được CSV báo cáo thuế', error);
    } finally {
      setExporting(false);
    }
  }

  const summary = report.data?.summary;
  const periods = report.data?.periods ?? [];
  const counterparties = report.data?.counterparties ?? [];
  const ledger = report.data?.ledger ?? [];
  const warnings = report.data?.warnings ?? [];

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-ink">Báo cáo thuế / VAT</h2>
          <p className="mt-1 text-xs text-slate-500">Theo ngày và nguồn hóa đơn; mặc định chỉ tính đầu vào đã xác minh và đầu ra đã phát hành.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 xl:flex xl:flex-wrap xl:items-end xl:justify-end">
          <ReportProjectField value={filters.projectId} options={projectOptions} onChange={(projectId) => setFilters((current) => ({ ...current, projectId }))} />
          <ReportDateField label="Từ ngày" value={filters.from} onChange={(from) => setFilters((current) => ({ ...current, from }))} />
          <ReportDateField label="Đến ngày" value={filters.to} onChange={(to) => setFilters((current) => ({ ...current, to }))} />
          <ReportSelectField label="Kỳ" value={filters.groupBy} onChange={(groupBy) => setFilters((current) => ({ ...current, groupBy: groupBy as TaxFilters['groupBy'] }))} options={[['month', 'Tháng'], ['quarter', 'Quý']]} />
          <ReportSelectField label="Nguồn" value={filters.source} onChange={(source) => setFilters((current) => ({ ...current, source: source as TaxFilters['source'] }))} options={[['all', 'Tất cả'], ['input', 'Đầu vào'], ['output', 'Đầu ra']]} />
          <ReportSelectField label="Trạng thái" value={filters.status} onChange={(status) => setFilters((current) => ({ ...current, status: status as TaxFilters['status'] }))} options={[['posted', 'Đã ghi nhận'], ['all', 'Tất cả chưa xóa']]} />
          <ReportFilterActions applying={report.isFetching} exporting={exporting || report.isLoading} onApply={applyFilters} onReset={resetFilters} onExport={() => void exportCSV()} />
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <MetricTile label="VAT đầu vào" value={<CurrencyText value={summary?.input_vat_amount ?? 0} />} />
        <MetricTile label="VAT đầu ra" value={<CurrencyText value={summary?.output_vat_amount ?? 0} />} />
        <MetricTile label="VAT thuần" value={<span className={netVATClass(summary?.net_vat_amount)}><CurrencyText value={summary?.net_vat_amount ?? 0} /></span>} />
        <MetricTile label="HĐ đầu vào" value={Number(summary?.input_invoice_count ?? 0)} />
        <MetricTile label="HĐ đầu ra" value={Number(summary?.output_invoice_count ?? 0)} />
        <MetricTile label="Cảnh báo" value={Number(summary?.warning_count ?? 0)} />
      </div>

      {warnings.length ? (
        <div className="grid gap-2 lg:grid-cols-3">
          {warnings.map((warning) => (
            <WarningItem
              key={warning.code}
              warning={warning}
              onFilter={warning.code === 'missing_tax_code' || warning.code === 'vat_math_mismatch' ? () => setLedgerQuery(warning.code) : undefined}
            />
          ))}
        </div>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Theo kỳ</h3>
          <DataTable<TaxPeriodRow>
            rows={periods}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có hoá đơn VAT trong khoảng lọc"
            pageSize={6}
            defaultSort={{ key: 'period_start', direction: 'asc' }}
            columns={[
              { key: 'period_start', header: 'Kỳ', sortable: true, render: (row) => dateText(row.period_start) },
              { key: 'input_vat_amount', header: 'VAT vào', align: 'right', render: (row) => <CurrencyText value={row.input_vat_amount} /> },
              { key: 'output_vat_amount', header: 'VAT ra', align: 'right', render: (row) => <CurrencyText value={row.output_vat_amount} /> },
              { key: 'net_vat_amount', header: 'VAT thuần', align: 'right', render: (row) => <span className={netVATClass(row.net_vat_amount)}><CurrencyText value={row.net_vat_amount} /></span> },
              { key: 'invoice_count', header: 'Số HĐ', align: 'right' },
            ]}
          />
        </div>

        <div className="space-y-2">
          <h3 className="text-xs font-semibold uppercase text-slate-500">Theo MST / đối tác</h3>
          <DataTable<TaxCounterpartyRow>
            rows={counterparties}
            loading={report.isLoading}
            error={report.error}
            onRetry={() => void report.refetch()}
            empty="Chưa có đối tác trong khoảng lọc"
            pageSize={6}
            columns={[
              { key: 'direction', header: 'Loại', render: (row) => <DirectionPill direction={row.direction} /> },
              { key: 'tax_code', header: 'MST', render: (row) => row.tax_code || <span className="font-semibold text-amber-700">Thiếu MST</span> },
              { key: 'counterparty_name', header: 'Đối tác', render: (row) => <span className="font-semibold text-ink">{row.counterparty_name}</span> },
              { key: 'invoice_count', header: 'HĐ', align: 'right' },
              { key: 'vat_amount', header: 'VAT', align: 'right', render: (row) => <CurrencyText value={row.vat_amount} /> },
            ]}
          />
        </div>
      </div>

      <div className="space-y-2">
        <h3 className="text-xs font-semibold uppercase text-slate-500">Sổ hóa đơn</h3>
        <DataTable<TaxLedgerRow>
          rows={ledger}
          loading={report.isLoading}
          error={report.error}
          onRetry={() => void report.refetch()}
          empty="Chưa có hoá đơn VAT trong khoảng lọc"
          searchPlaceholder="Tìm số hoá đơn, công trình, đối tác, MST..."
          searchQuery={ledgerQuery}
          onSearchQueryChange={setLedgerQuery}
          pageSize={10}
          defaultSort={{ key: 'invoice_date', direction: 'desc' }}
          columns={[
            { key: 'invoice_date', header: 'Ngày HĐ', sortable: true, render: (row) => dateText(row.invoice_date) },
            { key: 'invoice_number', header: 'Số HĐ', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.invoice_number}</span> },
            { key: 'direction', header: 'Loại', render: (row) => <DirectionPill direction={row.direction} /> },
            { key: 'project_name', header: 'Công trình' },
            { key: 'counterparty_name', header: 'Đối tác' },
            { key: 'tax_code', header: 'MST', render: (row) => row.tax_code || <span className="font-semibold text-amber-700">Thiếu MST</span> },
            { key: 'amount_before_vat', header: 'Trước VAT', align: 'right', render: (row) => <CurrencyText value={row.amount_before_vat} /> },
            { key: 'vat_amount', header: 'VAT', align: 'right', render: (row) => <CurrencyText value={row.vat_amount} /> },
            { key: 'total_amount', header: 'Tổng', align: 'right', render: (row) => <CurrencyText value={row.total_amount} /> },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'source_code', header: 'Nguồn' },
            { key: 'warning_flags', header: 'Cảnh báo', render: (row) => row.warning_flags || '-' },
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

function WarningItem({ warning, onFilter }: { warning: TaxReportWarning; onFilter?: () => void }) {
  const tone = warning.severity === 'info' ? 'border-blue-100 bg-blue-50 text-blue-700' : 'border-amber-100 bg-amber-50 text-amber-800';
  const content = (
    <>
      <p className="font-semibold">{warning.message}</p>
      <p className="mt-1">{Number(warning.count ?? 0)} dòng{warning.amount ? ` / ${Number(warning.amount).toLocaleString('vi-VN')} VND` : ''}</p>
      {onFilter ? <p className="mt-1 font-semibold underline">Lọc sổ hóa đơn</p> : null}
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
  if (direction === 'input') return <span className={`${base} bg-emerald-50 text-emerald-700`}>Đầu vào</span>;
  if (direction === 'output') return <span className={`${base} bg-blue-50 text-blue-700`}>Đầu ra</span>;
  return <span className={`${base} bg-slate-100 text-slate-600`}>{direction || '-'}</span>;
}

function netVATClass(value?: number) {
  const amount = Number(value ?? 0);
  if (amount > 0) return 'text-rose-700';
  if (amount < 0) return 'text-emerald-700';
  return 'text-ink';
}

function todayText() {
  return dateInputText(new Date());
}

function firstDayOfMonth() {
  const now = new Date();
  return dateInputText(new Date(now.getFullYear(), now.getMonth(), 1));
}

function defaultTaxFilters(): TaxFilters {
  return {
    projectId: '',
    from: firstDayOfMonth(),
    to: todayText(),
    groupBy: 'month',
    source: 'all',
    status: 'posted',
  };
}

function taxFiltersFromParams(params: URLSearchParams): TaxFilters {
  const defaults = defaultTaxFilters();
  const groupBy = params.get('group_by');
  const source = params.get('source');
  const status = params.get('status');
  return {
    projectId: params.get('project_id') ?? '',
    from: params.get('from') ?? defaults.from,
    to: params.get('to') ?? defaults.to,
    groupBy: groupBy === 'quarter' ? 'quarter' : 'month',
    source: source === 'input' || source === 'output' ? source : 'all',
    status: status === 'all' ? 'all' : 'posted',
  };
}

function taxFiltersToParams(filters: TaxFilters) {
  const params = new URLSearchParams();
  if (filters.projectId) params.set('project_id', filters.projectId);
  params.set('from', filters.from);
  params.set('to', filters.to);
  params.set('group_by', filters.groupBy);
  params.set('source', filters.source);
  params.set('status', filters.status);
  return params;
}

function dateInputText(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

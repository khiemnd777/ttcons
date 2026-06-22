import { type ReactNode, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download, RefreshCw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { TaxCounterpartyRow, TaxLedgerRow, TaxPeriodRow, TaxReport, TaxReportWarning } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function TaxReportPanel() {
  const [projectId, setProjectId] = useState('');
  const [from, setFrom] = useState(() => firstDayOfMonth());
  const [to, setTo] = useState(() => todayText());
  const [groupBy, setGroupBy] = useState<'month' | 'quarter'>('month');
  const [source, setSource] = useState<'all' | 'input' | 'output'>('all');
  const [status, setStatus] = useState<'posted' | 'all'>('posted');
  const toast = useToast();

  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (projectId) params.set('project_id', projectId);
    params.set('from', from);
    params.set('to', to);
    params.set('group_by', groupBy);
    params.set('source', source);
    params.set('status', status);
    return params.toString();
  }, [from, groupBy, projectId, source, status, to]);

  const report = useQuery({
    queryKey: ['tax-report', projectId || 'all', from, to, groupBy, source, status],
    queryFn: () => get<TaxReport>(`/reports/tax?${queryString}`),
  });

  async function exportCSV() {
    try {
      const blob = await downloadBlob(`/reports/tax/export?${queryString}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `tax-report-${from}-${to}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất CSV báo cáo thuế');
    } catch (error) {
      toast.error('Không xuất được CSV báo cáo thuế', error);
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
          <p className="mt-1 text-xs text-slate-500">Theo kỳ hoá đơn và nguồn chứng từ; mặc định chỉ tính input verified và output issued.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 xl:flex xl:flex-wrap xl:items-end xl:justify-end">
          <ProjectField value={projectId} onChange={setProjectId} />
          <DateField label="Từ ngày" value={from} onChange={setFrom} />
          <DateField label="Đến ngày" value={to} onChange={setTo} />
          <SelectField label="Kỳ" value={groupBy} onChange={(value) => setGroupBy(value as 'month' | 'quarter')} options={[['month', 'Tháng'], ['quarter', 'Quý']]} />
          <SelectField label="Nguồn" value={source} onChange={(value) => setSource(value as 'all' | 'input' | 'output')} options={[['all', 'All'], ['input', 'Đầu vào'], ['output', 'Đầu ra']]} />
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
        <MetricTile label="VAT đầu vào" value={<CurrencyText value={summary?.input_vat_amount ?? 0} />} />
        <MetricTile label="VAT đầu ra" value={<CurrencyText value={summary?.output_vat_amount ?? 0} />} />
        <MetricTile label="Net VAT" value={<span className={netVATClass(summary?.net_vat_amount)}><CurrencyText value={summary?.net_vat_amount ?? 0} /></span>} />
        <MetricTile label="HĐ đầu vào" value={Number(summary?.input_invoice_count ?? 0)} />
        <MetricTile label="HĐ đầu ra" value={Number(summary?.output_invoice_count ?? 0)} />
        <MetricTile label="Cảnh báo" value={Number(summary?.warning_count ?? 0)} />
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
              { key: 'net_vat_amount', header: 'Net VAT', align: 'right', render: (row) => <span className={netVATClass(row.net_vat_amount)}><CurrencyText value={row.net_vat_amount} /></span> },
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
        <h3 className="text-xs font-semibold uppercase text-slate-500">Ledger hoá đơn</h3>
        <DataTable<TaxLedgerRow>
          rows={ledger}
          loading={report.isLoading}
          error={report.error}
          onRetry={() => void report.refetch()}
          empty="Chưa có hoá đơn VAT trong khoảng lọc"
          searchPlaceholder="Tìm số hoá đơn, công trình, đối tác, MST..."
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
            { key: 'status', header: 'Status', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'source_code', header: 'Source' },
            { key: 'warning_flags', header: 'Flags', render: (row) => row.warning_flags || '-' },
          ]}
        />
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

function WarningItem({ warning }: { warning: TaxReportWarning }) {
  const tone = warning.severity === 'info' ? 'border-blue-100 bg-blue-50 text-blue-700' : 'border-amber-100 bg-amber-50 text-amber-800';
  return (
    <div className={`rounded-md border px-3 py-2 text-xs ${tone}`}>
      <p className="font-semibold">{warning.message}</p>
      <p className="mt-1">{Number(warning.count ?? 0)} dòng{warning.amount ? ` / ${Number(warning.amount).toLocaleString('vi-VN')} VND` : ''}</p>
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

function dateInputText(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

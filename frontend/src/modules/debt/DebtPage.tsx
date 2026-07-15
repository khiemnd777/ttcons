import { type ReactNode, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Plus } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable, type Column } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { Partner, PayableRecord, ReceivableRecord } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { enumOptions } from '../../lib/utils/status';
import { partnerOptions, vendorPartnerTypes } from '../core-data/partnerUtils';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { InputInvoicePanel } from './InputInvoicePanel';
import { InputPaymentPlanPanel } from './InputPaymentPlanPanel';
import { OutputBillingPlanPanel } from './OutputBillingPlanPanel';
import { OutputInvoicePanel } from './OutputInvoicePanel';
import { PayableReconciliationPanel } from './PayableReconciliationPanel';
import { ReceivableReconciliationPanel } from './ReceivableReconciliationPanel';
import { agingBadgeClass, debtAgingLabel, reconciliationClass, reconciliationLabel } from './payableDebtUtils';

type DebtKind = 'payables' | 'receivables';
type Queue = 'all' | 'needs_review' | 'due_soon' | 'overdue' | 'settled';
type DetailTab = 'overview' | 'invoices' | 'plan' | 'payments' | 'reconciliation';
type Detail = { type: 'payable'; record: PayableRecord } | { type: 'receivable'; record: ReceivableRecord } | null;

const detailTabs: Array<{ value: DetailTab; label: string }> = [
  { value: 'overview', label: 'Tổng quan' },
  { value: 'invoices', label: 'Hóa đơn' },
  { value: 'plan', label: 'Lịch & tạm ứng' },
  { value: 'payments', label: 'Thu/chi' },
  { value: 'reconciliation', label: 'Đối soát' },
];

export function DebtPage({ kind }: { kind: DebtKind | 'both' }) {
  const { id } = useParams();
  const { hasAnyPermission } = useAuth();
  const canViewPayables = kind !== 'receivables' && hasAnyPermission(['accounting.payable_view']);
  const canViewReceivables = kind !== 'payables' && hasAnyPermission(['accounting.receivable_view']);
  const canCreatePayable = hasAnyPermission(['accounting.payable_approve']);
  const canCreateReceivable = hasAnyPermission(['accounting.receivable_approve']);
  const [activeKind, setActiveKind] = useState<DebtKind>(canViewPayables ? 'payables' : 'receivables');
  const [queue, setQueue] = useState<Queue>('needs_review');
  const [open, setOpen] = useState<'payable' | 'receivable' | null>(null);
  const [detail, setDetail] = useState<Detail>(null);
  const [detailTab, setDetailTab] = useState<DetailTab>('overview');
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const partners = useQuery({ queryKey: ['partners', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const scope = id ? `?project_id=${encodeURIComponent(id)}` : '';
  const payables = useQuery({ queryKey: ['payables', id ?? 'global'], queryFn: () => get<PayableRecord[]>(`/payables${scope}`), enabled: canViewPayables });
  const receivables = useQuery({ queryKey: ['receivables', id ?? 'global'], queryFn: () => get<ReceivableRecord[]>(`/receivables${scope}`), enabled: canViewReceivables });
  const payableRows = payables.data ?? [];
  const receivableRows = receivables.data ?? [];
  const payableSummary = useMemo(() => summarize(payableRows), [payableRows]);
  const receivableSummary = useMemo(() => summarize(receivableRows), [receivableRows]);
  const currentKind = activeKind === 'payables' && canViewPayables ? 'payables' : 'receivables';
  const currentRows = currentKind === 'payables' ? payableRows : receivableRows;
  const queueOptions = useMemo(() => buildQueueOptions(currentRows), [currentRows]);
  const filteredRows = useMemo(() => currentRows.filter((row) => matchesQueue(row, queue)), [currentRows, queue]);

  const invalidatePayables = (payableID?: string) => {
    queryClient.invalidateQueries({ queryKey: ['payables'] });
    if (payableID) queryClient.invalidateQueries({ queryKey: ['payable-reconciliation', payableID] });
  };
  const invalidateReceivables = (receivableID?: string) => {
    queryClient.invalidateQueries({ queryKey: ['receivables'] });
    if (receivableID) queryClient.invalidateQueries({ queryKey: ['receivable-reconciliation', receivableID] });
  };
  const createPayable = useMutation({
    mutationFn: (values: Record<string, any>) => post('/payables', values),
    onSuccess: () => { invalidatePayables(); setOpen(null); toast.success('Đã ghi nhận phải trả'); },
    onError: (error) => toast.error('Không ghi nhận được phải trả', error),
  });
  const createReceivable = useMutation({
    mutationFn: (values: Record<string, any>) => post('/receivables', values),
    onSuccess: () => { invalidateReceivables(); setOpen(null); toast.success('Đã ghi nhận phải thu'); },
    onError: (error) => toast.error('Không ghi nhận được phải thu', error),
  });
  const openDetail = (next: Detail) => { setDetail(next); setDetailTab('overview'); };
  const setKind = (next: DebtKind) => { setActiveKind(next); setQueue('needs_review'); };
  const activeSummary = currentKind === 'payables' ? payableSummary : receivableSummary;

  return (
    <>
      <PageHeader
        title="Công nợ"
        description="Xử lý phải trả, phải thu, lịch đến hạn, hóa đơn và đối soát theo từng nghĩa vụ."
        actions={<>{id ? <ProjectBackLink projectId={id} /> : null}{currentKind === 'payables' && canCreatePayable ? <PrimaryButton label="Ghi nhận phải trả" onClick={() => setOpen('payable')} /> : null}{currentKind === 'receivables' && canCreateReceivable ? <PrimaryButton label="Ghi nhận phải thu" onClick={() => setOpen('receivable')} /> : null}</>}
      />

      {kind === 'both' ? (
        <div className="mb-4 flex flex-wrap gap-2 border-b border-line">
          {canViewPayables ? <KindTab active={currentKind === 'payables'} label="Phải trả" count={payableRows.length} amount={payableSummary.remaining} onClick={() => setKind('payables')} /> : null}
          {canViewReceivables ? <KindTab active={currentKind === 'receivables'} label="Phải thu" count={receivableRows.length} amount={receivableSummary.remaining} onClick={() => setKind('receivables')} /> : null}
        </div>
      ) : null}

      <section className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7">
          <DebtMetric label={currentKind === 'payables' ? 'Còn phải trả' : 'Còn phải thu'} value={<CurrencyText value={activeSummary.remaining} />} />
          <DebtMetric label="Chưa đến hạn" value={<CurrencyText value={activeSummary.notDue} />} />
          <DebtMetric label="1–30 ngày" value={<CurrencyText value={activeSummary.bucket1To30} />} />
          <DebtMetric label="31–60 ngày" value={<CurrencyText value={activeSummary.bucket31To60} />} />
          <DebtMetric label="61–90 ngày" value={<CurrencyText value={activeSummary.bucket61To90} />} />
          <DebtMetric label="Trên 90 ngày" value={<CurrencyText value={activeSummary.bucket90Plus} />} />
          <DebtMetric label="Cần đối soát" value={<span className={activeSummary.needsReview ? 'text-amber-700' : 'text-emerald-700'}>{activeSummary.needsReview}</span>} />
        </div>

        <FilterBar label="Hàng đợi" value={queue} options={queueOptions} onChange={(value) => setQueue(value as Queue)} />
        {queue === 'needs_review' && filteredRows.length ? <div className="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800"><AlertTriangle className="h-4 w-4" />Ưu tiên mở từng dòng để xử lý chênh lệch hóa đơn, lịch và phân bổ thu/chi.</div> : null}

        {currentKind === 'payables' ? (
          <DataTable rows={filteredRows as PayableRecord[]} loading={payables.isLoading} error={payables.error} onRetry={() => void payables.refetch()} empty={queue === 'all' ? 'Chưa có phải trả' : 'Không có phải trả trong hàng đợi này'} searchPlaceholder="Tìm mã, nhà cung cấp, công trình..." pageSize={10} defaultSort={{ key: 'effective_due_date', direction: 'asc' }} columns={payableColumns(id, (row) => openDetail({ type: 'payable', record: row }))} />
        ) : (
          <DataTable rows={filteredRows as ReceivableRecord[]} loading={receivables.isLoading} error={receivables.error} onRetry={() => void receivables.refetch()} empty={queue === 'all' ? 'Chưa có phải thu' : 'Không có phải thu trong hàng đợi này'} searchPlaceholder="Tìm mã, khách hàng, công trình..." pageSize={10} defaultSort={{ key: 'effective_due_date', direction: 'asc' }} columns={receivableColumns(id, (row) => openDetail({ type: 'receivable', record: row }))} />
        )}
      </section>

      <RecordFormDialog open={open === 'payable'} title="Ghi nhận phải trả" submitLabel="Tạo bản nháp" onClose={() => setOpen(null)} fields={[
        { name: 'project_id', label: 'Công trình', type: 'select', defaultValue: id ?? '', required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
        { name: 'payable_code', label: 'Mã phải trả', defaultValue: `PAY-${Date.now().toString().slice(-5)}`, required: true },
        { name: 'vendor_type', label: 'Loại đối tác', type: 'select', defaultValue: 'supplier', required: true, options: enumOptions('vendor_type', ['supplier', 'subcontractor', 'labor_team', 'transport', 'equipment_rental', 'service_provider', 'other']) },
        { name: 'vendor_partner_id', label: 'Đối tác trong danh mục', type: 'select', options: partnerOptions(partners.data ?? [], vendorPartnerTypes), helperText: 'Tên đối tác được chụp lại vào công nợ.' },
        { name: 'vendor_name', label: 'Tên đối tác', required: true, hiddenWhen: (values: Record<string, any>) => Boolean(values.vendor_partner_id) },
        { name: 'gross_amount', label: 'Giá trị ghi nhận', type: 'number', defaultValue: 10000000, required: true, min: 1, helperText: 'Hệ thống tạo nháp và tự tính số dư.' },
        { name: 'due_date', label: 'Hạn thanh toán', type: 'date' },
        { name: 'advance_amount', label: 'Tạm ứng đã khấu trừ', type: 'number', defaultValue: 0, min: 0 },
        { name: 'retention_amount', label: 'Giữ lại', type: 'number', defaultValue: 0, min: 0 },
        { name: 'deduction_amount', label: 'Khấu trừ khác', type: 'number', defaultValue: 0, min: 0 },
      ]} onSubmit={(values) => createPayable.mutateAsync(values)} />

      <RecordFormDialog open={open === 'receivable'} title="Ghi nhận phải thu" submitLabel="Tạo bản nháp" onClose={() => setOpen(null)} fields={[
        { name: 'project_id', label: 'Công trình', type: 'select', defaultValue: id ?? '', required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
        { name: 'receivable_code', label: 'Mã phải thu', defaultValue: `REC-${Date.now().toString().slice(-5)}`, required: true },
        { name: 'client_partner_id', label: 'Khách hàng trong danh mục', type: 'select', options: partnerOptions(partners.data ?? [], ['client']), helperText: 'Tên khách hàng được chụp lại vào công nợ.' },
        { name: 'client_name', label: 'Tên khách hàng', required: true, hiddenWhen: (values: Record<string, any>) => Boolean(values.client_partner_id) },
        { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number', min: 0 },
        { name: 'milestone_name', label: 'Đợt thanh toán', required: true },
        { name: 'gross_amount', label: 'Giá trị ghi nhận', type: 'number', defaultValue: 30000000, required: true, min: 1, helperText: 'Hệ thống tạo nháp và tự tính số dư.' },
        { name: 'due_date', label: 'Hạn thu', type: 'date' },
      ]} onSubmit={(values) => createReceivable.mutateAsync(values)} />

      <Dialog open={Boolean(detail)} title={detail ? `${detail.type === 'payable' ? detail.record.payable_code : detail.record.receivable_code} · ${detail.type === 'payable' ? 'Phải trả' : 'Phải thu'}` : 'Chi tiết công nợ'} description="Mỗi tab sở hữu một phần nghiệp vụ; thu/chi gắn công nợ luôn được tạo tại đây để giữ đúng phân bổ." onClose={() => setDetail(null)} size="workspace">
        {detail ? <>
          <div className="mb-4 flex gap-1 overflow-x-auto border-b border-line">
            {detailTabs.map((tab) => <button key={tab.value} type="button" className={`whitespace-nowrap border-b-2 px-3 py-2 text-sm font-semibold ${detailTab === tab.value ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink'}`} onClick={() => setDetailTab(tab.value)}>{tab.label}</button>)}
          </div>
          {detail.type === 'payable' ? <PayableDetail tab={detailTab} record={detail.record} onChanged={() => invalidatePayables(detail.record.id)} /> : <ReceivableDetail tab={detailTab} record={detail.record} onChanged={() => invalidateReceivables(detail.record.id)} />}
        </> : null}
      </Dialog>
    </>
  );
}

function PayableDetail({ tab, record, onChanged }: { tab: DetailTab; record: PayableRecord; onChanged: () => void }) {
  if (tab === 'invoices') return <InputInvoicePanel payable={record} onChanged={onChanged} />;
  if (tab === 'plan') return <InputPaymentPlanPanel payable={record} onChanged={onChanged} view="plan" />;
  if (tab === 'payments') return <InputPaymentPlanPanel payable={record} onChanged={onChanged} view="payments" />;
  if (tab === 'reconciliation') return <PayableReconciliationPanel payable={record} />;
  return <DebtOverview record={record} direction="payable" />;
}

function ReceivableDetail({ tab, record, onChanged }: { tab: DetailTab; record: ReceivableRecord; onChanged: () => void }) {
  if (tab === 'invoices') return <OutputInvoicePanel receivable={record} onChanged={onChanged} />;
  if (tab === 'plan') return <OutputBillingPlanPanel receivable={record} onChanged={onChanged} view="plan" />;
  if (tab === 'payments') return <OutputBillingPlanPanel receivable={record} onChanged={onChanged} view="payments" />;
  if (tab === 'reconciliation') return <ReceivableReconciliationPanel receivable={record} />;
  return <DebtOverview record={record} direction="receivable" />;
}

function DebtOverview({ record, direction }: { record: PayableRecord | ReceivableRecord; direction: 'payable' | 'receivable' }) {
  const payable = direction === 'payable';
  const party = payable ? (record as PayableRecord).vendor_name : (record as ReceivableRecord).client_name;
  const paid = payable ? (record as PayableRecord).paid_amount : (record as ReceivableRecord).received_amount;
  const advance = payable ? (record as PayableRecord).advance_approved_total : (record as ReceivableRecord).advance_approved_total;
  return <div className="space-y-4">
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <DebtMetric label={payable ? 'Nhà cung cấp/đối tác' : 'Khách hàng'} value={party || '-'} />
      <DebtMetric label="Giá trị ghi nhận" value={<CurrencyText value={record.gross_amount} />} />
      <DebtMetric label={payable ? 'Còn phải trả' : 'Còn phải thu'} value={<CurrencyText value={record.remaining_amount ?? 0} />} />
      <DebtMetric label="Trạng thái" value={<StatusBadge status={record.status} />} />
      <DebtMetric label={payable ? 'Đã trả' : 'Đã thu'} value={<CurrencyText value={paid ?? 0} />} />
      <DebtMetric label="Ứng trước đã duyệt" value={<CurrencyText value={advance ?? 0} />} />
      <DebtMetric label="Hạn" value={dateText(record.effective_due_date ?? record.due_date)} />
      <DebtMetric label="Tuổi nợ" value={<span className={agingBadgeClass(record.aging_bucket)}>{debtAgingLabel(record.aging_bucket, record.overdue_days)}</span>} />
    </div>
    <div className="rounded-md border border-line bg-slate-50 p-4 text-sm text-slate-600">Hóa đơn, lịch/tạm ứng, thu/chi và đối soát được tách thành các tab để người xử lý luôn ở đúng ngữ cảnh.</div>
  </div>;
}

function payableColumns(projectId: string | undefined, onView: (row: PayableRecord) => void): Column<PayableRecord>[] {
  return [
    { key: 'payable_code', header: 'Mã', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.payable_code}</span> },
    { key: 'vendor_name', header: 'Nhà cung cấp/đối tác' },
    ...(!projectId ? [{ key: 'project_name', header: 'Công trình' } as Column<PayableRecord>] : []),
    { key: 'gross_amount', header: 'Giá trị', align: 'right', render: (row) => <CurrencyText value={row.gross_amount} /> },
    { key: 'remaining_amount', header: 'Còn phải trả', align: 'right', render: (row) => <CurrencyText value={row.remaining_amount ?? 0} /> },
    { key: 'effective_due_date', header: 'Hạn', sortable: true, render: (row) => dateText(row.effective_due_date ?? row.due_date) },
    { key: 'aging_bucket', header: 'Tuổi nợ', sortable: true, render: (row) => <span className={agingBadgeClass(row.aging_bucket)}>{debtAgingLabel(row.aging_bucket, row.overdue_days)}</span> },
    { key: 'invoice_count', header: 'Hóa đơn', render: (row) => <div><div className="font-semibold text-ink">{Number(row.invoice_count ?? 0)} PDF</div><div className="text-xs text-slate-500"><CurrencyText value={row.verified_invoice_total ?? 0} /> đã xác thực</div></div> },
    { key: 'reconciliation_status', header: 'Đối soát', render: (row) => <span className={reconciliationClass(row.reconciliation_status, row.reconciliation_issue_count)}>{reconciliationLabel(row.reconciliation_status, row.reconciliation_issue_count)}</span> },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
    { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => onView(row)} /> },
  ];
}

function receivableColumns(projectId: string | undefined, onView: (row: ReceivableRecord) => void): Column<ReceivableRecord>[] {
  return [
    { key: 'receivable_code', header: 'Mã', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.receivable_code}</span> },
    { key: 'client_name', header: 'Khách hàng' },
    ...(!projectId ? [{ key: 'project_name', header: 'Công trình' } as Column<ReceivableRecord>] : []),
    { key: 'milestone_name', header: 'Đợt thanh toán' },
    { key: 'gross_amount', header: 'Giá trị', align: 'right', render: (row) => <CurrencyText value={row.gross_amount} /> },
    { key: 'remaining_amount', header: 'Còn phải thu', align: 'right', render: (row) => <CurrencyText value={row.remaining_amount ?? 0} /> },
    { key: 'effective_due_date', header: 'Hạn', sortable: true, render: (row) => dateText(row.effective_due_date ?? row.due_date) },
    { key: 'aging_bucket', header: 'Tuổi nợ', sortable: true, render: (row) => <span className={agingBadgeClass(row.aging_bucket)}>{debtAgingLabel(row.aging_bucket, row.overdue_days)}</span> },
    { key: 'invoice_count', header: 'Hóa đơn', render: (row) => <div><div className="font-semibold text-ink">{Number(row.invoice_count ?? 0)} PDF</div><div className="text-xs text-slate-500"><CurrencyText value={row.issued_invoice_total ?? 0} /> đã phát hành</div></div> },
    { key: 'reconciliation_status', header: 'Đối soát', render: (row) => <span className={reconciliationClass(row.reconciliation_status, row.reconciliation_issue_count)}>{reconciliationLabel(row.reconciliation_status, row.reconciliation_issue_count)}</span> },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
    { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => onView(row)} /> },
  ];
}

function summarize(rows: Array<PayableRecord | ReceivableRecord>) {
  return rows.reduce((summary, row) => {
    const remaining = Number(row.remaining_amount ?? 0);
    summary.remaining += remaining;
    if (row.aging_bucket === 'not_due') summary.notDue += remaining;
    if (row.aging_bucket === '1_30') summary.bucket1To30 += remaining;
    if (row.aging_bucket === '31_60') summary.bucket31To60 += remaining;
    if (row.aging_bucket === '61_90') summary.bucket61To90 += remaining;
    if (row.aging_bucket === '90_plus') summary.bucket90Plus += remaining;
    if (needsReview(row)) summary.needsReview += 1;
    return summary;
  }, { remaining: 0, notDue: 0, bucket1To30: 0, bucket31To60: 0, bucket61To90: 0, bucket90Plus: 0, needsReview: 0 });
}

function buildQueueOptions(rows: Array<PayableRecord | ReceivableRecord>): FilterOption[] {
  const values: Array<{ value: Queue; label: string }> = [
    { value: 'needs_review', label: 'Cần xử lý' },
    { value: 'due_soon', label: 'Sắp đến hạn' },
    { value: 'overdue', label: 'Quá hạn' },
    { value: 'settled', label: 'Đã tất toán' },
    { value: 'all', label: 'Tất cả' },
  ];
  return values.map((item) => ({ ...item, count: rows.filter((row) => matchesQueue(row, item.value)).length }));
}

function matchesQueue(row: PayableRecord | ReceivableRecord, queue: Queue) {
  if (queue === 'all') return true;
  if (queue === 'needs_review') return needsReview(row);
  if (queue === 'settled') return row.aging_bucket === 'settled' || Number(row.remaining_amount ?? 0) <= 0;
  if (queue === 'overdue') return ['1_30', '31_60', '61_90', '90_plus'].includes(row.aging_bucket ?? '');
  if (row.aging_bucket !== 'not_due') return false;
  const due = new Date(row.effective_due_date ?? row.due_date ?? '').getTime();
  return Number.isFinite(due) && due - Date.now() <= 30 * 24 * 60 * 60 * 1000;
}

function needsReview(row: PayableRecord | ReceivableRecord) {
  return row.reconciliation_status === 'needs_review' || Number(row.reconciliation_issue_count ?? 0) > 0;
}

function KindTab({ active, label, count, amount, onClick }: { active: boolean; label: string; count: number; amount: number; onClick: () => void }) {
  return <button type="button" className={`min-w-44 border-b-2 px-4 py-3 text-left ${active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink'}`} onClick={onClick}><span className="block text-sm font-semibold">{label} · {count}</span><span className="mt-0.5 block text-xs"><CurrencyText value={amount} /> còn lại</span></button>;
}

function PrimaryButton({ label, onClick }: { label: string; onClick: () => void }) {
  return <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={onClick}><Plus className="h-4 w-4" />{label}</button>;
}

function DebtMetric({ label, value }: { label: string; value: ReactNode }) {
  return <div className="rounded-md border border-line bg-slate-50 px-3 py-2"><p className="text-xs font-semibold uppercase text-slate-500">{label}</p><div className="mt-1 truncate text-sm font-semibold text-ink">{value}</div></div>;
}

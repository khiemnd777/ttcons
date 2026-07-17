import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Plus } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable, type Column } from '../../components/DataTable';
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
import { DebtCreateDialog, type DebtCreateMode } from './DebtCreateDialog';
import { DebtDetailDialog, type DebtDetail, type DebtDetailTab } from './DebtDetailDialog';
import { agingBadgeClass, debtAgingLabel, reconciliationClass, reconciliationLabel } from './payableDebtUtils';

type DebtKind = 'payables' | 'receivables';
type Queue = 'all' | 'needs_review' | 'due_soon' | 'overdue' | 'settled';

export function DebtPage({ kind }: { kind: DebtKind | 'both' }) {
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const { hasAnyPermission } = useAuth();
  const canViewPayables = kind !== 'receivables' && hasAnyPermission(['accounting.payable_view']);
  const canViewReceivables = kind !== 'payables' && hasAnyPermission(['accounting.receivable_view']);
  const canCreatePayable = hasAnyPermission(['accounting.payable_approve']);
  const canCreateReceivable = hasAnyPermission(['accounting.receivable_approve']);
  const [activeKind, setActiveKind] = useState<DebtKind>(canViewPayables ? 'payables' : 'receivables');
  const [queue, setQueue] = useState<Queue>('needs_review');
  const appliedSourceQuery = useRef('');
  const [open, setOpen] = useState<DebtCreateMode | null>(null);
  const [detail, setDetail] = useState<DebtDetail | null>(null);
  const [detailTab, setDetailTab] = useState<DebtDetailTab>('overview');
  const queryClient = useQueryClient();
  const toast = useToast();
  const requestedKind = debtKindFromQuery(searchParams.get('kind'));
  const requestedQueue = debtQueueFromQuery(searchParams.get('queue'));
  useEffect(() => {
    const queryKey = `${requestedKind ?? ''}:${requestedQueue ?? ''}`;
    if (queryKey === ':' || appliedSourceQuery.current === queryKey) return;
    if (requestedKind === 'payables' && !canViewPayables) return;
    if (requestedKind === 'receivables' && !canViewReceivables) return;
    appliedSourceQuery.current = queryKey;
    if (requestedKind === 'payables' && canViewPayables) setActiveKind('payables');
    if (requestedKind === 'receivables' && canViewReceivables) setActiveKind('receivables');
    if (requestedQueue) setQueue(requestedQueue);
  }, [canViewPayables, canViewReceivables, requestedKind, requestedQueue]);
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const partners = useQuery({ queryKey: ['partners', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const scope = id ? `?project_id=${encodeURIComponent(id)}` : '';
  const payables = useQuery({ queryKey: ['payables', id ?? 'global'], queryFn: () => get<PayableRecord[]>(`/payables${scope}`), enabled: canViewPayables });
  const receivables = useQuery({ queryKey: ['receivables', id ?? 'global'], queryFn: () => get<ReceivableRecord[]>(`/receivables${scope}`), enabled: canViewReceivables });
  const payableRows = payables.data ?? [];
  const receivableRows = receivables.data ?? [];
  const projectOptions = useMemo(() => (projects.data ?? []).map((project) => ({ id: project.id, name: project.name })), [projects.data]);
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
  const approveDebt = useMutation({
    mutationFn: (current: DebtDetail) => post<PayableRecord | ReceivableRecord>(`/${current.type === 'payable' ? 'payables' : 'receivables'}/${current.record.id}/approve`, { comment: 'Duyệt công nợ từ workspace chi tiết' }),
    onSuccess: (row, current) => {
      if (current.type === 'payable') {
        queryClient.setQueriesData<PayableRecord[]>({ queryKey: ['payables'] }, (rows) => rows?.map((item) => item.id === row.id ? { ...item, ...row } as PayableRecord : item));
        invalidatePayables(current.record.id);
      } else {
        queryClient.setQueriesData<ReceivableRecord[]>({ queryKey: ['receivables'] }, (rows) => rows?.map((item) => item.id === row.id ? { ...item, ...row } as ReceivableRecord : item));
        invalidateReceivables(current.record.id);
      }
      setDetail((active) => active && active.type === current.type && active.record.id === current.record.id ? { ...active, record: { ...active.record, ...row } } as DebtDetail : active);
      toast.success(`Đã duyệt ${current.type === 'payable' ? 'phải trả' : 'phải thu'}`);
    },
    onError: (error) => toast.error('Không duyệt được công nợ', error),
  });
  const openDetail = (next: DebtDetail) => { setDetail(next); setDetailTab(Number(next.record.reconciliation_issue_count ?? 0) > 0 ? 'reconciliation' : 'overview'); };
  const setKind = (next: DebtKind) => { setActiveKind(next); setQueue('needs_review'); };
  const activeSummary = currentKind === 'payables' ? payableSummary : receivableSummary;

  useEffect(() => {
    if (!detail) return;
    const rows = detail.type === 'payable' ? payableRows : receivableRows;
    const refreshed = rows.find((row) => row.id === detail.record.id);
    if (refreshed && refreshed !== detail.record) setDetail((current) => current ? { ...current, record: refreshed } as DebtDetail : current);
  }, [detail?.record.id, detail?.type, payableRows, receivableRows]);

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

      <DebtCreateDialog
        open={Boolean(open)}
        initialMode={open ?? (currentKind === 'payables' ? 'payable' : 'receivable')}
        projectId={id}
        projects={projectOptions}
        partners={partners.data ?? []}
        canCreatePayable={canCreatePayable}
        canCreateReceivable={canCreateReceivable}
        onClose={() => setOpen(null)}
        onCreatePayable={(values) => createPayable.mutateAsync(values)}
        onCreateReceivable={(values) => createReceivable.mutateAsync(values)}
      />

      <DebtDetailDialog
        detail={detail}
        tab={detailTab}
        canApprove={detail?.type === 'payable' ? canCreatePayable : canCreateReceivable}
        approving={approveDebt.isPending}
        onTab={setDetailTab}
        onClose={() => setDetail(null)}
        onChanged={() => detail?.type === 'payable' ? invalidatePayables(detail.record.id) : detail ? invalidateReceivables(detail.record.id) : undefined}
        onApprove={() => { if (detail) approveDebt.mutate(detail); }}
      />
    </>
  );
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

function debtKindFromQuery(value: string | null): DebtKind | null {
  return value === 'payables' || value === 'receivables' ? value : null;
}

function debtQueueFromQuery(value: string | null): Queue | null {
  return value === 'all' || value === 'needs_review' || value === 'due_soon' || value === 'overdue' || value === 'settled' ? value : null;
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

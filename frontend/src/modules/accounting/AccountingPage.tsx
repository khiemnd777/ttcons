import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { ListSection } from '../../components/ListSection';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';
import { CashflowPanel } from './CashflowPanel';

export function AccountingPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [budgetStatus, setBudgetStatus] = useState('all');
  const [paymentStatus, setPaymentStatus] = useState('all');
  const [paymentType, setPaymentType] = useState('all');
  const [open, setOpen] = useState<'budget' | 'payment' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'budget' | 'payment'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const budgets = useQuery({ queryKey: ['budget', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/budget`), enabled: Boolean(projectId) });
  const payments = useQuery({ queryKey: ['payments'], queryFn: () => get<Record<string, any>[]>('/payments') });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['budget', projectId] });
    queryClient.invalidateQueries({ queryKey: ['payments'] });
    queryClient.invalidateQueries({ queryKey: ['cashflow-report'] });
  };
  const createBudget = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/budget`, values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo ngân sách');
    },
    onError: (error) => toast.error('Không tạo được ngân sách', error),
  });
  const createPayment = useMutation({
    mutationFn: (values: Record<string, any>) => post('/payments', values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo payment / receipt');
    },
    onError: (error) => toast.error('Không tạo được payment / receipt', error),
  });
  const approveBudget = useMutation({
    mutationFn: (budgetId: string) => post(`/budgets/${budgetId}/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt ngân sách');
    },
    onError: (error) => toast.error('Không duyệt được ngân sách', error),
  });
  const approvePayment = useMutation({
    mutationFn: (paymentId: string) => post(`/payments/${paymentId}/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt payment / receipt');
    },
    onError: (error) => toast.error('Không duyệt được payment / receipt', error),
  });
  const budgetRows = budgets.data ?? [];
  const projectPaymentRows = (payments.data ?? []).filter((row) => !projectId || row.project_id === projectId);
  const filteredBudgets = useMemo(() => budgetStatus === 'all' ? budgetRows : budgetRows.filter((row) => row.status === budgetStatus), [budgetRows, budgetStatus]);
  const filteredPayments = useMemo(() => projectPaymentRows.filter((row) => {
    const statusMatch = paymentStatus === 'all' || row.status === paymentStatus;
    const typeMatch = paymentType === 'all' || row.payment_type === paymentType;
    return statusMatch && typeMatch;
  }), [paymentStatus, paymentType, projectPaymentRows]);
  const budgetFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: budgetRows.length },
    { value: 'draft', label: 'Draft', count: countBy(budgetRows, 'status', 'draft') },
    { value: 'submitted', label: 'Submitted', count: countBy(budgetRows, 'status', 'submitted') },
    { value: 'approved', label: 'Approved', count: countBy(budgetRows, 'status', 'approved') },
  ], [budgetRows]);
  const paymentStatusOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: projectPaymentRows.length },
    { value: 'draft', label: 'Draft', count: countBy(projectPaymentRows, 'status', 'draft') },
    { value: 'pending', label: 'Pending', count: countBy(projectPaymentRows, 'status', 'pending') },
    { value: 'waiting_approval', label: 'Waiting', count: countBy(projectPaymentRows, 'status', 'waiting_approval') },
    { value: 'approved', label: 'Approved', count: countBy(projectPaymentRows, 'status', 'approved') },
    { value: 'paid', label: 'Paid', count: countBy(projectPaymentRows, 'status', 'paid') },
    { value: 'received', label: 'Received', count: countBy(projectPaymentRows, 'status', 'received') },
  ], [projectPaymentRows]);
  const paymentTypeOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Mọi loại', count: projectPaymentRows.length },
    { value: 'supplier_payment', label: 'Supplier', count: countBy(projectPaymentRows, 'payment_type', 'supplier_payment') },
    { value: 'subcontractor_payment', label: 'Subcontractor', count: countBy(projectPaymentRows, 'payment_type', 'subcontractor_payment') },
    { value: 'client_receipt', label: 'Client receipt', count: countBy(projectPaymentRows, 'payment_type', 'client_receipt') },
    { value: 'outbound', label: 'Outbound', count: countBy(projectPaymentRows, 'payment_type', 'outbound') },
    { value: 'inbound', label: 'Inbound', count: countBy(projectPaymentRows, 'payment_type', 'inbound') },
    { value: 'advance', label: 'Advance', count: countBy(projectPaymentRows, 'payment_type', 'advance') },
    { value: 'other', label: 'Other', count: countBy(projectPaymentRows, 'payment_type', 'other') },
  ], [projectPaymentRows]);

  return (
    <>
      <PageHeader
        title="Kế toán"
        description="Ngân sách, thanh toán, tạm ứng/hoàn ứng, dòng tiền và kiểm soát tài chính công trình."
        actions={
          <>
            {!global ? <ProjectBackLink projectId={projectId} /> : null}
            {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('payment')}><Plus className="h-4 w-4" />Payment</button>
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('budget')}><Plus className="h-4 w-4" />Budget</button>
          </>
        }
      />
      <div className="mb-4">
        <CashflowPanel projectId={projectId} />
      </div>
      <div className="mb-4 grid gap-3 xl:grid-cols-3">
        <FilterBar label="Budget status" value={budgetStatus} options={budgetFilterOptions} onChange={setBudgetStatus} disabled={!projectId} />
        <FilterBar label="Payment status" value={paymentStatus} options={paymentStatusOptions} onChange={setPaymentStatus} disabled={!projectId} />
        <FilterBar label="Payment type" value={paymentType} options={paymentTypeOptions} onChange={setPaymentType} disabled={!projectId} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <ListSection title="Ngân sách công trình">
          <DataTable rows={filteredBudgets} loading={budgets.isLoading} error={budgets.error} onRetry={() => budgets.refetch()} searchPlaceholder="Tìm mã, ngân sách, trạng thái..." pageSize={10} defaultSort={{ key: 'budget_code', direction: 'asc' }} columns={[
            { key: 'budget_code', header: 'Mã', sortable: true },
            { key: 'name', header: 'Ngân sách', sortable: true },
            { key: 'approved_amount', header: 'Đã duyệt', align: 'right', sortable: true, sortValue: (row) => Number(row.approved_amount ?? 0), render: (row) => <CurrencyText value={row.approved_amount} /> },
            { key: 'used_amount', header: 'Đã dùng', align: 'right', sortable: true, sortValue: (row) => Number(row.used_amount ?? 0), render: (row) => <CurrencyText value={row.used_amount} /> },
            { key: 'remaining_amount', header: 'Còn lại', align: 'right', sortable: true, sortValue: (row) => Number(row.remaining_amount ?? 0), render: (row) => <CurrencyText value={row.remaining_amount} /> },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'budget', record: row })}><button disabled={row.status === 'approved'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => approveBudget.mutate(row.id)}>Approve</button></RowActions> },
          ]} empty={projectId ? 'Không có ngân sách trong bộ lọc này' : 'Chọn công trình để xem ngân sách'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('budget')}>
              <Plus className="h-4 w-4" />Budget
            </button>
          ) : undefined} />
        </ListSection>
        <ListSection title="Payments / Receipts">
          <DataTable rows={filteredPayments} loading={payments.isLoading} error={payments.error} onRetry={() => payments.refetch()} searchPlaceholder="Tìm mã phiếu, loại, công trình..." pageSize={10} defaultSort={{ key: 'payment_code', direction: 'desc' }} columns={[
            { key: 'payment_code', header: 'Mã', sortable: true },
            { key: 'payment_type', header: 'Loại', sortable: true, render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
            { key: 'project_name', header: 'Công trình', sortable: true },
            { key: 'amount', header: 'Số tiền', align: 'right', sortable: true, sortValue: (row) => Number(row.amount ?? 0), render: (row) => <CurrencyText value={row.amount} /> },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'payment', record: row })}><button disabled={!['draft', 'submitted', 'pending'].includes(row.status)} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => approvePayment.mutate(row.id)}><Check className="inline h-4 w-4" /> Approve</button></RowActions> },
          ]} empty={projectId ? 'Không có payment / receipt trong bộ lọc này' : 'Chọn công trình để xem payment / receipt'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpen('payment')}>
              <Plus className="h-4 w-4" />Payment
            </button>
          ) : undefined} />
        </ListSection>
      </div>
      <RecordFormDialog
        open={open === 'budget'}
        title="Tạo ngân sách"
        submitLabel="Tạo ngân sách"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'budget_code', label: 'Mã ngân sách', defaultValue: `BUD-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'name', label: 'Tên ngân sách', defaultValue: 'Ngân sách công trình', required: true },
          { name: 'planned_amount', label: 'Kế hoạch', type: 'number', defaultValue: 100000000, required: true, min: 0 },
          { name: 'approved_amount', label: 'Đã duyệt', type: 'number', defaultValue: 0, min: 0 },
          { name: 'used_amount', label: 'Đã dùng', type: 'number', defaultValue: 0, min: 0 },
          { name: 'remaining_amount', label: 'Còn lại', type: 'number', defaultValue: 100000000, min: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'submitted', 'approved']) },
        ]}
        onSubmit={(values) => createBudget.mutateAsync(values)}
      />
      <RecordFormDialog
        open={open === 'payment'}
        title="Tạo payment / receipt"
        submitLabel="Tạo payment / receipt"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'project_id', label: 'Công trình', type: 'select', defaultValue: projectId, required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
          { name: 'payment_code', label: 'Mã', defaultValue: `PMT-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'payment_type', label: 'Loại', type: 'select', defaultValue: 'supplier_payment', required: true, options: enumOptions('payment_type', ['supplier_payment', 'subcontractor_payment', 'client_receipt', 'advance', 'advance_settlement', 'other']) },
          { name: 'amount', label: 'Số tiền', type: 'number', defaultValue: 10000000, required: true, min: 0 },
          { name: 'payment_method', label: 'Phương thức', type: 'select', defaultValue: 'bank_transfer', required: true, options: enumOptions('payment_method', ['bank_transfer', 'cash']) },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'submitted', 'pending', 'approved', 'paid', 'received']) },
        ]}
        onSubmit={(values) => createPayment.mutateAsync(values)}
      />
      <RecordDetailDialog
        open={dialog?.type === 'budget'}
        title="Chi tiết ngân sách"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'budget_code', label: 'Mã' },
          { key: 'name', label: 'Ngân sách' },
          { key: 'planned_amount', label: 'Kế hoạch', render: (row) => <CurrencyText value={row.planned_amount} /> },
          { key: 'approved_amount', label: 'Đã duyệt', render: (row) => <CurrencyText value={row.approved_amount} /> },
          { key: 'used_amount', label: 'Đã dùng', render: (row) => <CurrencyText value={row.used_amount} /> },
          { key: 'remaining_amount', label: 'Còn lại', render: (row) => <CurrencyText value={row.remaining_amount} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <RecordDetailDialog
        open={dialog?.type === 'payment'}
        title="Chi tiết payment / receipt"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'payment_code', label: 'Mã' },
          { key: 'payment_type', label: 'Loại', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
          { key: 'project_name', label: 'Công trình' },
          { key: 'amount', label: 'Số tiền', render: (row) => <CurrencyText value={row.amount} /> },
          { key: 'payment_method', label: 'Phương thức', render: (row) => <EnumChip group="payment_method" value={row.payment_method} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
    </>
  );
}

function countBy(rows: Record<string, any>[], key: string, value: string) {
  return rows.filter((row) => row[key] === value).length;
}

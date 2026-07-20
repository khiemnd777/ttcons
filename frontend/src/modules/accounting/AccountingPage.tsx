import { lazy, type ReactNode, Suspense, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Check, Plus } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { ListSection } from '../../components/ListSection';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { AccountingCreateDialog, type AccountingCreateMode } from './AccountingCreateDialog';
import { AccountingDetailDialog, type AccountingDetail } from './AccountingDetailDialog';

const CashflowPanel = lazy(() => import('./CashflowPanel').then((module) => ({ default: module.CashflowPanel })));

type AccountingTab = 'overview' | 'budgets' | 'payments' | 'cashflow';
type BudgetTotals = { planned: number; approved: number; used: number; remaining: number };

const tabs: Array<{ value: AccountingTab; label: string }> = [
  { value: 'overview', label: 'Tổng quan' },
  { value: 'budgets', label: 'Ngân sách' },
  { value: 'payments', label: 'Phiếu thu/chi' },
  { value: 'cashflow', label: 'Dòng tiền' },
];

export function AccountingPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const { hasAnyPermission } = useAuth();
  const [projectId, setProjectId] = useState(id ?? '');
  const [tab, setTab] = useState<AccountingTab>('overview');
  const [budgetStatus, setBudgetStatus] = useState('all');
  const [paymentStatus, setPaymentStatus] = useState('all');
  const [paymentType, setPaymentType] = useState('all');
  const [open, setOpen] = useState<AccountingCreateMode | null>(null);
  const [dialog, setDialog] = useState<AccountingDetail | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const canViewBudget = hasAnyPermission(['accounting.budget_view']);
  const canManageBudget = hasAnyPermission(['accounting.budget_approve']);
  const canCreatePayment = hasAnyPermission(['accounting.payment_create']);
  const canApprovePayment = hasAnyPermission(['accounting.payment_approve']);
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const selectedProject = (projects.data ?? []).find((project) => project.id === projectId);
  const projectOptions = useMemo(() => (projects.data ?? []).map((project) => ({ id: project.id, name: project.name })), [projects.data]);
  const budgets = useQuery({ queryKey: ['budget', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/budget`), enabled: Boolean(projectId) && canViewBudget });
  const payments = useQuery({ queryKey: ['payments'], queryFn: () => get<Record<string, any>[]>('/payments') });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['budget', projectId] });
    queryClient.invalidateQueries({ queryKey: ['payments'] });
    queryClient.invalidateQueries({ queryKey: ['cashflow-report'] });
  };
  const createBudget = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/budget`, values),
    onSuccess: () => { invalidate(); setOpen(null); toast.success('Đã tạo ngân sách nháp'); },
    onError: (error) => toast.error('Không tạo được ngân sách', error),
  });
  const createPayment = useMutation({
    mutationFn: (values: Record<string, any>) => post('/payments', values),
    onSuccess: () => { invalidate(); setOpen(null); toast.success('Đã tạo phiếu thu/chi nháp'); },
    onError: (error) => toast.error('Không tạo được phiếu thu/chi', error),
  });
  const approveBudget = useMutation({
    mutationFn: (budgetId: string) => post<Record<string, any>>(`/budgets/${budgetId}/approve`, {}),
    onSuccess: (row) => { invalidate(); setDialog((current) => current?.type === 'budget' && current.record.id === row.id ? { ...current, record: { ...current.record, ...row } } : current); toast.success('Đã duyệt ngân sách'); },
    onError: (error) => toast.error('Không duyệt được ngân sách', error),
  });
  const approvePayment = useMutation({
    mutationFn: (paymentId: string) => post<Record<string, any>>(`/payments/${paymentId}/approve`, {}),
    onSuccess: (row) => { invalidate(); setDialog((current) => current?.type === 'payment' && current.record.id === row.id ? { ...current, record: { ...current.record, ...row } } : current); toast.success('Đã duyệt phiếu thu/chi'); },
    onError: (error) => toast.error('Không duyệt được phiếu thu/chi', error),
  });

  const budgetRows = budgets.data ?? [];
  const projectPaymentRows = (payments.data ?? []).filter((row) => !projectId || row.project_id === projectId);
  const pendingPayments = projectPaymentRows.filter((row) => ['draft', 'submitted', 'pending', 'waiting_approval'].includes(row.status));
  const filteredBudgets = useMemo(() => budgetStatus === 'all' ? budgetRows : budgetRows.filter((row) => row.status === budgetStatus), [budgetRows, budgetStatus]);
  const filteredPayments = useMemo(() => projectPaymentRows.filter((row) => (paymentStatus === 'all' || row.status === paymentStatus) && (paymentType === 'all' || row.payment_type === paymentType)), [paymentStatus, paymentType, projectPaymentRows]);
  const totals = useMemo<BudgetTotals>(() => budgetRows.reduce<BudgetTotals>((result, row) => ({
    planned: result.planned + Number(row.planned_amount ?? 0),
    approved: result.approved + Number(row.approved_amount ?? 0),
    used: result.used + Number(row.used_amount ?? 0),
    remaining: result.remaining + Number(row.remaining_amount ?? 0),
  }), { planned: 0, approved: 0, used: 0, remaining: 0 }), [budgetRows]);
  const pendingAmount = pendingPayments.reduce((sum, row) => sum + Number(row.amount ?? 0), 0);

  const budgetFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: budgetRows.length },
    { value: 'draft', label: 'Nháp', count: countBy(budgetRows, 'status', 'draft') },
    { value: 'submitted', label: 'Đã gửi', count: countBy(budgetRows, 'status', 'submitted') },
    { value: 'approved', label: 'Đã duyệt', count: countBy(budgetRows, 'status', 'approved') },
  ], [budgetRows]);
  const paymentStatusOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: projectPaymentRows.length },
    { value: 'draft', label: 'Nháp', count: countBy(projectPaymentRows, 'status', 'draft') },
    { value: 'pending', label: 'Chờ xử lý', count: countBy(projectPaymentRows, 'status', 'pending') },
    { value: 'waiting_approval', label: 'Chờ duyệt', count: countBy(projectPaymentRows, 'status', 'waiting_approval') },
    { value: 'approved', label: 'Đã duyệt', count: countBy(projectPaymentRows, 'status', 'approved') },
    { value: 'paid', label: 'Đã chi', count: countBy(projectPaymentRows, 'status', 'paid') },
    { value: 'received', label: 'Đã thu', count: countBy(projectPaymentRows, 'status', 'received') },
  ], [projectPaymentRows]);
  const paymentTypeOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Mọi loại', count: projectPaymentRows.length },
    { value: 'supplier_payment', label: 'Trả nhà cung cấp', count: countBy(projectPaymentRows, 'payment_type', 'supplier_payment') },
    { value: 'subcontractor_payment', label: 'Trả thầu phụ', count: countBy(projectPaymentRows, 'payment_type', 'subcontractor_payment') },
    { value: 'client_receipt', label: 'Thu khách hàng', count: countBy(projectPaymentRows, 'payment_type', 'client_receipt') },
    { value: 'outbound', label: 'Chi khác', count: countBy(projectPaymentRows, 'payment_type', 'outbound') },
    { value: 'inbound', label: 'Thu khác', count: countBy(projectPaymentRows, 'payment_type', 'inbound') },
    { value: 'other', label: 'Điều chỉnh khác', count: countBy(projectPaymentRows, 'payment_type', 'other') },
  ], [projectPaymentRows]);

  return (
    <>
      <PageHeader
        title="Kế toán"
        description="Kiểm soát ngân sách, duyệt phiếu và theo dõi dòng tiền; công nợ phải trả và phải thu được xử lý tại màn Công nợ."
        actions={<>{!global ? <ProjectBackLink projectId={projectId} /> : null}{global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}{canApprovePayment && pendingPayments.length ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => { setPaymentStatus('all'); setTab('payments'); }}><Check className="h-4 w-4" />Duyệt {pendingPayments.length} phiếu</button> : canCreatePayment ? <button disabled={!projectId} type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => setOpen('payment')}><Plus className="h-4 w-4" />Phiếu thu/chi khác</button> : null}</>}
      />

      <div className="mb-4 flex gap-1 overflow-x-auto border-b border-line">
        {tabs.map((item) => <button key={item.value} type="button" className={`whitespace-nowrap border-b-2 px-4 py-3 text-sm font-semibold ${tab === item.value ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink'}`} onClick={() => setTab(item.value)}>{item.label}{item.value === 'payments' && pendingPayments.length ? ` · ${pendingPayments.length}` : ''}</button>)}
      </div>

      {tab === 'overview' ? <AccountingOverview projectId={projectId} totals={totals} pendingCount={pendingPayments.length} pendingAmount={pendingAmount} paymentCount={projectPaymentRows.length} canCreatePayment={canCreatePayment} canManageBudget={canManageBudget} onOpenBudget={() => setOpen('budget')} onOpenPayment={() => setOpen('payment')} onTab={setTab} /> : null}

      {tab === 'budgets' ? <div className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><FilterBar label="Trạng thái ngân sách" value={budgetStatus} options={budgetFilterOptions} onChange={setBudgetStatus} disabled={!projectId || !canViewBudget} />{canManageBudget ? <button disabled={!projectId} type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => setOpen('budget')}><Plus className="h-4 w-4" />Ngân sách</button> : null}</div>
        <ListSection title="Ngân sách công trình"><DataTable rows={filteredBudgets} loading={budgets.isLoading} error={budgets.error} onRetry={() => budgets.refetch()} searchPlaceholder="Tìm mã, tên ngân sách, trạng thái..." pageSize={10} defaultSort={{ key: 'budget_code', direction: 'asc' }} columns={[
          { key: 'budget_code', header: 'Mã', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.budget_code}</span> },
          { key: 'name', header: 'Tên ngân sách', sortable: true },
          { key: 'planned_amount', header: 'Kế hoạch', align: 'right', render: (row) => <CurrencyText value={row.planned_amount} /> },
          { key: 'approved_amount', header: 'Đã duyệt', align: 'right', render: (row) => <CurrencyText value={row.approved_amount} /> },
          { key: 'used_amount', header: 'Đã dùng', align: 'right', render: (row) => <CurrencyText value={row.used_amount} /> },
          { key: 'remaining_amount', header: 'Còn lại', align: 'right', render: (row) => <CurrencyText value={row.remaining_amount} /> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'budget', record: row })}>{canManageBudget && ['draft', 'submitted'].includes(row.status) ? <button className="text-sm font-semibold text-blue-700" onClick={() => approveBudget.mutate(row.id)}>Duyệt</button> : null}</RowActions> },
        ]} empty={canViewBudget ? 'Chưa có ngân sách trong bộ lọc này' : 'Bạn không có quyền xem ngân sách'} /></ListSection>
      </div> : null}

      {tab === 'payments' ? <div className="space-y-4">
        <div className="grid gap-3 xl:grid-cols-2"><FilterBar label="Trạng thái phiếu" value={paymentStatus} options={paymentStatusOptions} onChange={setPaymentStatus} disabled={!projectId} /><FilterBar label="Loại phiếu" value={paymentType} options={paymentTypeOptions} onChange={setPaymentType} disabled={!projectId} /></div>
        <div className="flex justify-end">{canCreatePayment ? <button disabled={!projectId} type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={() => setOpen('payment')}><Plus className="h-4 w-4" />Phiếu thu/chi khác</button> : null}</div>
        <ListSection title="Phiếu thu/chi"><DataTable rows={filteredPayments} loading={payments.isLoading} error={payments.error} onRetry={() => payments.refetch()} searchPlaceholder="Tìm mã phiếu, loại, công trình..." pageSize={10} defaultSort={{ key: 'created_at', direction: 'desc' }} columns={[
          { key: 'payment_code', header: 'Mã', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.payment_code}</span> },
          { key: 'payment_type', header: 'Loại', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
          ...(!id ? [{ key: 'project_name', header: 'Công trình' }] : []),
          { key: 'amount', header: 'Số tiền', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
          { key: 'payment_method', header: 'Phương thức', render: (row) => <EnumChip group="payment_method" value={row.payment_method} /> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'payment', record: row })}>{canApprovePayment && ['draft', 'submitted', 'pending', 'waiting_approval'].includes(row.status) ? <button className="text-sm font-semibold text-blue-700" onClick={() => approvePayment.mutate(row.id)}><Check className="mr-1 inline h-4 w-4" />Duyệt</button> : null}</RowActions> },
        ]} empty="Chưa có phiếu thu/chi trong bộ lọc này" /></ListSection>
      </div> : null}

      {tab === 'cashflow' ? (
        <Suspense fallback={<div className="rounded-lg border border-line bg-white p-8"><LoadingSpinner /></div>}>
          <CashflowPanel projectId={projectId} initialFrom={!global ? selectedProject?.start_date ?? '2000-01-01' : undefined} />
        </Suspense>
      ) : null}

      <AccountingCreateDialog
        open={Boolean(open)}
        initialMode={open ?? 'payment'}
        projectId={projectId}
        projects={projectOptions}
        canCreateBudget={canManageBudget}
        canCreatePayment={canCreatePayment}
        onClose={() => setOpen(null)}
        onCreateBudget={(values) => createBudget.mutateAsync(values)}
        onCreatePayment={(values) => createPayment.mutateAsync(values)}
      />

      <AccountingDetailDialog
        detail={dialog}
        canApprove={dialog?.type === 'budget' ? canManageBudget : canApprovePayment}
        approving={dialog?.type === 'budget' ? approveBudget.isPending : approvePayment.isPending}
        onClose={() => setDialog(null)}
        onApprove={() => {
          if (!dialog) return;
          if (dialog.type === 'budget') approveBudget.mutate(dialog.record.id);
          else approvePayment.mutate(dialog.record.id);
        }}
      />
    </>
  );
}

function AccountingOverview({ projectId, totals, pendingCount, pendingAmount, paymentCount, canCreatePayment, canManageBudget, onOpenBudget, onOpenPayment, onTab }: { projectId: string; totals: { planned: number; approved: number; used: number; remaining: number }; pendingCount: number; pendingAmount: number; paymentCount: number; canCreatePayment: boolean; canManageBudget: boolean; onOpenBudget: () => void; onOpenPayment: () => void; onTab: (tab: AccountingTab) => void }) {
  return <div className="space-y-4">
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"><Metric label="Ngân sách đã duyệt" value={<CurrencyText value={totals.approved} />} /><Metric label="Đã sử dụng" value={<CurrencyText value={totals.used} />} /><Metric label="Còn lại" value={<CurrencyText value={totals.remaining} />} /><Metric label="Phiếu chờ xử lý" value={`${pendingCount} · ${moneyLabel(pendingAmount)}`} tone={pendingCount ? 'amber' : 'slate'} /></div>
    <div className="grid gap-4 lg:grid-cols-2">
      <ActionCard title={pendingCount ? `${pendingCount} phiếu cần duyệt/xử lý` : 'Không có phiếu đang chờ'} description={pendingCount ? `Tổng giá trị đang chờ: ${moneyLabel(pendingAmount)}.` : `Đã ghi nhận ${paymentCount} phiếu trong công trình.`} actionLabel="Mở hàng đợi phiếu" onClick={() => onTab('payments')} primary={pendingCount > 0} />
      <ActionCard title="Theo dõi dòng tiền" description="Đối chiếu thu, chi, số dư, phiếu chưa phân loại và dữ liệu không tiền mặt." actionLabel="Mở báo cáo dòng tiền" onClick={() => onTab('cashflow')} />
    </div>
    <div className="flex flex-wrap justify-end gap-2">{canCreatePayment ? <button disabled={!projectId} type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-50" onClick={onOpenPayment}><Plus className="mr-1 inline h-4 w-4" />Phiếu thu/chi khác</button> : null}{canManageBudget ? <button disabled={!projectId} type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-50" onClick={onOpenBudget}><Plus className="mr-1 inline h-4 w-4" />Ngân sách</button> : null}</div>
  </div>;
}

function ActionCard({ title, description, actionLabel, onClick, primary = false }: { title: string; description: string; actionLabel: string; onClick: () => void; primary?: boolean }) {
  return <div className={`rounded-lg border p-4 ${primary ? 'border-amber-200 bg-amber-50' : 'border-line bg-white'}`}><h3 className="font-semibold text-ink">{title}</h3><p className="mt-1 text-sm text-slate-600">{description}</p><button type="button" className="mt-4 inline-flex items-center gap-2 text-sm font-semibold text-blue-700" onClick={onClick}>{actionLabel}<ArrowRight className="h-4 w-4" /></button></div>;
}

function Metric({ label, value, tone = 'slate' }: { label: string; value: ReactNode; tone?: 'slate' | 'amber' }) {
  return <div className={`rounded-md border px-3 py-2 ${tone === 'amber' ? 'border-amber-200 bg-amber-50' : 'border-line bg-slate-50'}`}><div className="text-xs font-semibold uppercase text-slate-500">{label}</div><div className="mt-1 text-base font-semibold text-ink">{value}</div></div>;
}

function moneyLabel(value: number) { return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND', maximumFractionDigits: 0 }).format(value); }
function countBy(rows: Record<string, any>[], key: string, value: string) { return rows.filter((row) => row[key] === value).length; }

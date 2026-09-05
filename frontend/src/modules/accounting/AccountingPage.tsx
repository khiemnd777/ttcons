import { lazy, type ReactNode, Suspense, useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { dateText } from '../../lib/utils/format';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Check, ClipboardCheck, Plus, RotateCcw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
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
import { SimpleForm } from '../_shared/SimpleForm';
import { AccountingBooksPanel, type AccountingBookView } from './AccountingBooksPanel';
import { AccountingCreateDialog, type AccountingCreateMode } from './AccountingCreateDialog';
import { AccountingDetailDialog, type AccountingDetail } from './AccountingDetailDialog';

const CashflowPanel = lazy(() => import('./CashflowPanel').then((module) => ({ default: module.CashflowPanel })));

type AccountingTab = 'overview' | 'tasks' | 'budgets' | 'payments' | AccountingBookView;
type BudgetTotals = { planned: number; approved: number; committed: number; actual: number; paid: number; available: number };

const tabs: Array<{ value: AccountingTab; label: string }> = [
  { value: 'overview', label: 'Tổng quan' },
	{ value: 'tasks', label: 'Việc cần xử lý' },
  { value: 'budgets', label: 'Ngân sách' },
	{ value: 'payments', label: 'Thu/Chi' },
	{ value: 'journal', label: 'Sổ Nhật ký chung' },
	{ value: 'ledger', label: 'Sổ Cái' },
	{ value: 'tax', label: 'Thuế' },
];

export function AccountingPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const { hasAnyPermission } = useAuth();
  const [projectId, setProjectId] = useState(id ?? '');
  const [tab, setTab] = useState<AccountingTab>('overview');
  const [budgetStatus, setBudgetStatus] = useState('all');
  const [paymentStatus, setPaymentStatus] = useState('all');
  const [paymentType, setPaymentType] = useState('all');
	const [paymentView, setPaymentView] = useState<'vouchers' | 'cashflow'>('vouchers');
  const [open, setOpen] = useState<AccountingCreateMode | null>(null);
  const [dialog, setDialog] = useState<AccountingDetail | null>(null);
	const [reversingPayment, setReversingPayment] = useState<Record<string, any> | null>(null);
  const [reconciliationIssue, setReconciliationIssue] = useState<Record<string, any> | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const canViewBudget = hasAnyPermission(['accounting.budget_view']);
  const canManageBudget = hasAnyPermission(['accounting.budget_approve']);
  const canCreatePayment = hasAnyPermission(['accounting.payment_create']);
  const canApprovePayment = hasAnyPermission(['accounting.payment_approve']);
	const canViewJournal = hasAnyPermission(['accounting.journal_view']);
	const canPostJournal = hasAnyPermission(['accounting.journal_post']);
	const canViewTax = hasAnyPermission(['accounting.tax_view']);
	const canReverseJournal = hasAnyPermission(['accounting.journal_reverse']);
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const selectedProject = (projects.data ?? []).find((project) => project.id === projectId);
  const projectOptions = useMemo(() => (projects.data ?? []).map((project) => ({ id: project.id, name: project.name })), [projects.data]);
  const budgets = useQuery({ queryKey: ['budget', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/budget`), enabled: Boolean(projectId) && canViewBudget });
	const accounts = useQuery({ queryKey: ['accounting-accounts', projectId], queryFn: () => get<Record<string, any>[]>(`/accounting/accounts?project_id=${projectId}`), enabled: Boolean(projectId) && (canViewJournal || canViewTax) });
  const reconciliationIssues = useQuery({ queryKey: ['accounting-reconciliation-issues', projectId], queryFn: () => get<Record<string, any>[]>(`/accounting/reconciliation-issues?project_id=${encodeURIComponent(projectId)}`), enabled: Boolean(projectId) && canViewJournal });
  const journalEntries = useQuery({ queryKey: ['accounting-journal-options', projectId], queryFn: () => get<Record<string, any>[]>(`/accounting/journal-entries?project_id=${encodeURIComponent(projectId)}`), enabled: Boolean(projectId) && canViewJournal });
  const payments = useQuery({ queryKey: ['payments'], queryFn: () => get<Record<string, any>[]>('/payments') });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['budget', projectId] });
    queryClient.invalidateQueries({ queryKey: ['payments'] });
    queryClient.invalidateQueries({ queryKey: ['cashflow-report'] });
	queryClient.invalidateQueries({ queryKey: ['accounting-journal'] });
	queryClient.invalidateQueries({ queryKey: ['accounting-trial-balance'] });
	queryClient.invalidateQueries({ queryKey: ['accounting-general-ledger'] });
	queryClient.invalidateQueries({ queryKey: ['accounting-profit-loss'] });
	queryClient.invalidateQueries({ queryKey: ['accounting-vat-ledger'] });
    queryClient.invalidateQueries({ queryKey: ['accounting-reconciliation-issues'] });
    queryClient.invalidateQueries({ queryKey: ['accounting-journal-options'] });
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
	const reversePayment = useMutation({
	  mutationFn: ({ paymentId, reason }: { paymentId: string; reason: string }) => post<Record<string, any>>(`/payments/${paymentId}/reverse`, { reason }),
	  onSuccess: () => { invalidate(); setDialog(null); setReversingPayment(null); toast.success('Đã lập chứng từ đảo và hoàn nguyên phân bổ'); },
	  onError: (error) => toast.error('Không đảo được Phiếu thu/Phiếu chi', error),
	});
  const resolveReconciliation = useMutation({
    mutationFn: ({ issueId, values }: { issueId: string; values: Record<string, any> }) => post(`/accounting/reconciliation-issues/${issueId}/resolve`, values),
    onSuccess: () => { invalidate(); setReconciliationIssue(null); toast.success('Đã hoàn tất đối chiếu chuyển đổi sổ kế toán'); },
    onError: (error) => toast.error('Không xử lý được đối chiếu chuyển đổi', error),
  });

  const budgetRows = budgets.data ?? [];
  const projectPaymentRows = (payments.data ?? []).filter((row) => !projectId || row.project_id === projectId);
  const pendingPayments = projectPaymentRows.filter((row) => ['draft', 'submitted', 'pending', 'waiting_approval'].includes(row.status));
  const openReconciliationIssues = reconciliationIssues.data ?? [];
  const pendingTaskCount = pendingPayments.length + openReconciliationIssues.length;
  const filteredBudgets = useMemo(() => budgetStatus === 'all' ? budgetRows : budgetRows.filter((row) => row.status === budgetStatus), [budgetRows, budgetStatus]);
  const filteredPayments = useMemo(() => projectPaymentRows.filter((row) => (paymentStatus === 'all' || row.status === paymentStatus) && (paymentType === 'all' || row.payment_type === paymentType)), [paymentStatus, paymentType, projectPaymentRows]);
	const totals = useMemo<BudgetTotals>(() => budgetRows.reduce<BudgetTotals>((result, row) => ({
    planned: result.planned + Number(row.planned_amount ?? 0),
    approved: result.approved + Number(row.approved_amount ?? 0),
	committed: result.committed + Number(row.committed_amount ?? 0),
	actual: result.actual + Number(row.actual_amount ?? row.used_amount ?? 0),
	paid: result.paid + Number(row.paid_amount ?? 0),
	available: result.available + Number(row.available_amount ?? row.remaining_amount ?? 0),
	}), { planned: 0, approved: 0, committed: 0, actual: 0, paid: 0, available: 0 }), [budgetRows]);
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
  ], [projectPaymentRows]);
  const paymentTypeOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Mọi loại', count: projectPaymentRows.length },
    { value: 'supplier_payment', label: 'Trả nhà cung cấp', count: countBy(projectPaymentRows, 'payment_type', 'supplier_payment') },
    { value: 'subcontractor_payment', label: 'Trả thầu phụ', count: countBy(projectPaymentRows, 'payment_type', 'subcontractor_payment') },
    { value: 'client_receipt', label: 'Thu khách hàng', count: countBy(projectPaymentRows, 'payment_type', 'client_receipt') },
    { value: 'outbound', label: 'Chi khác', count: countBy(projectPaymentRows, 'payment_type', 'outbound') },
    { value: 'inbound', label: 'Thu khác', count: countBy(projectPaymentRows, 'payment_type', 'inbound') },
    { value: 'tax_payment', label: 'Nộp thuế GTGT', count: countBy(projectPaymentRows, 'payment_type', 'tax_payment') },
    { value: 'other', label: 'Dữ liệu cũ cần đối chiếu', count: countBy(projectPaymentRows, 'payment_type', 'other') },
  ], [projectPaymentRows]);

  return (
    <>
      <PageHeader
        title="Kế toán"
		description="Ghi nhận chứng từ, kiểm soát ngân sách, đối chiếu công nợ và lập sổ kế toán theo nguyên tắc ghi Nợ – ghi Có."
        actions={<>{!global ? <ProjectBackLink projectId={projectId} /> : null}{global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}{pendingTaskCount ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => { setPaymentStatus('all'); setTab('tasks'); }}><Check className="h-4 w-4" />Xử lý {pendingTaskCount} việc</button> : canCreatePayment ? <button disabled={!projectId} type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => setOpen('payment')}><Plus className="h-4 w-4" />Lập Phiếu thu/Phiếu chi</button> : null}</>}
      />

      <div className="mb-4 flex gap-1 overflow-x-auto border-b border-line">
		{tabs.map((item) => <button key={item.value} type="button" className={`whitespace-nowrap border-b-2 px-4 py-3 text-sm font-semibold ${tab === item.value ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink'}`} onClick={() => setTab(item.value)}>{item.label}{item.value === 'tasks' && pendingTaskCount ? ` · ${pendingTaskCount}` : ''}</button>)}
      </div>

	  {tab === 'overview' ? <AccountingOverview projectId={projectId} totals={totals} pendingCount={pendingPayments.length} pendingAmount={pendingAmount} paymentCount={projectPaymentRows.length} canCreatePayment={canCreatePayment} canManageBudget={canManageBudget} onOpenBudget={() => setOpen('budget')} onOpenPayment={() => setOpen('payment')} onTab={setTab} /> : null}

	  {tab === 'tasks' ? <AccountingTasks rows={pendingPayments} issues={openReconciliationIssues} loading={payments.isLoading} issuesLoading={reconciliationIssues.isLoading} error={payments.error} issuesError={reconciliationIssues.error} canApprove={canApprovePayment} canResolve={canPostJournal} onRetry={() => payments.refetch()} onRetryIssues={() => reconciliationIssues.refetch()} onView={(record) => setDialog({ type: 'payment', record })} onApprove={(paymentId) => approvePayment.mutate(paymentId)} onResolve={setReconciliationIssue} /> : null}

      {tab === 'budgets' ? <div className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><FilterBar label="Trạng thái ngân sách" value={budgetStatus} options={budgetFilterOptions} onChange={setBudgetStatus} disabled={!projectId || !canViewBudget} />{canManageBudget ? <button disabled={!projectId} type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => setOpen('budget')}><Plus className="h-4 w-4" />Ngân sách</button> : null}</div>
        <ListSection title="Ngân sách công trình"><DataTable rows={filteredBudgets} loading={budgets.isLoading} error={budgets.error} onRetry={() => budgets.refetch()} searchPlaceholder="Tìm mã, tên ngân sách, trạng thái..." pageSize={10} defaultSort={{ key: 'budget_code', direction: 'asc' }} columns={[
          { key: 'budget_code', header: 'Mã', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.budget_code}</span> },
          { key: 'name', header: 'Tên ngân sách', sortable: true },
          { key: 'planned_amount', header: 'Kế hoạch', align: 'right', render: (row) => <CurrencyText value={row.planned_amount} /> },
          { key: 'approved_amount', header: 'Đã duyệt', align: 'right', render: (row) => <CurrencyText value={row.approved_amount} /> },
		  { key: 'committed_amount', header: 'Đã cam kết', align: 'right', render: (row) => <CurrencyText value={row.committed_amount} /> },
		  { key: 'actual_amount', header: 'Thực chi ghi nhận', align: 'right', render: (row) => <CurrencyText value={row.actual_amount} /> },
		  { key: 'paid_amount', header: 'Đã thanh toán', align: 'right', render: (row) => <CurrencyText value={row.paid_amount} /> },
		  { key: 'available_amount', header: 'Khả dụng', align: 'right', render: (row) => <span className={Number(row.available_amount ?? 0) < 0 ? 'font-semibold text-rose-700' : ''}><CurrencyText value={row.available_amount} /></span> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'budget', record: row })}>{canManageBudget && ['draft', 'submitted'].includes(row.status) ? <button className="text-sm font-semibold text-blue-700" onClick={() => approveBudget.mutate(row.id)}>Duyệt</button> : null}</RowActions> },
        ]} empty={canViewBudget ? 'Chưa có ngân sách trong bộ lọc này' : 'Bạn không có quyền xem ngân sách'} /></ListSection>
      </div> : null}

	  {tab === 'payments' ? <div className="space-y-4">
		<div className="flex w-fit rounded-lg border border-line bg-slate-50 p-1"><SubViewButton active={paymentView === 'vouchers'} label="Chứng từ Thu/Chi" onClick={() => setPaymentView('vouchers')} /><SubViewButton active={paymentView === 'cashflow'} label="Sổ quỹ và dòng tiền" onClick={() => setPaymentView('cashflow')} /></div>
		{paymentView === 'vouchers' ? <>
        <div className="grid gap-3 xl:grid-cols-2"><FilterBar label="Trạng thái phiếu" value={paymentStatus} options={paymentStatusOptions} onChange={setPaymentStatus} disabled={!projectId} /><FilterBar label="Loại phiếu" value={paymentType} options={paymentTypeOptions} onChange={setPaymentType} disabled={!projectId} /></div>
		<div className="flex justify-end">{canCreatePayment ? <button disabled={!projectId} type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={() => setOpen('payment')}><Plus className="h-4 w-4" />Lập Phiếu thu/Phiếu chi khác</button> : null}</div>
		<ListSection title="Chứng từ Thu/Chi"><DataTable rows={filteredPayments} loading={payments.isLoading} error={payments.error} onRetry={() => payments.refetch()} searchPlaceholder="Tìm số phiếu, loại chứng từ, công trình..." pageSize={10} defaultSort={{ key: 'created_at', direction: 'desc' }} columns={[
		  { key: 'payment_code', header: 'Số phiếu', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.payment_code}</span> },
		  { key: 'payment_type', header: 'Loại chứng từ', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
          ...(!id ? [{ key: 'project_name', header: 'Công trình' }] : []),
          { key: 'amount', header: 'Số tiền', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
          { key: 'payment_method', header: 'Phương thức', render: (row) => <EnumChip group="payment_method" value={row.payment_method} /> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
		  { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'payment', record: row })}>{canApprovePayment && ['draft', 'submitted', 'pending', 'waiting_approval'].includes(row.status) ? <button className="text-sm font-semibold text-blue-700" onClick={() => approvePayment.mutate(row.id)}><Check className="mr-1 inline h-4 w-4" />Duyệt</button> : null}{canReverseJournal && row.status === 'approved' ? <button className="text-sm font-semibold text-rose-700" onClick={() => setReversingPayment(row)}><RotateCcw className="mr-1 inline h-4 w-4" />Lập chứng từ đảo</button> : null}</RowActions> },
		]} empty="Chưa có chứng từ Thu/Chi trong bộ lọc này" /></ListSection>
		</> : <Suspense fallback={<div className="rounded-lg border border-line bg-white p-8"><LoadingSpinner /></div>}><CashflowPanel projectId={projectId} initialFrom={!global ? selectedProject?.start_date ?? currentFiscalYearStart() : undefined} /></Suspense>}
      </div> : null}

	  {tab === 'journal' || tab === 'ledger' || tab === 'tax' ? <AccountingBooksPanel view={tab} projectId={projectId} enabled={tab === 'tax' ? canViewTax : canViewJournal} accounts={accounts.data ?? []} /> : null}

      <AccountingCreateDialog
        open={Boolean(open)}
        initialMode={open ?? 'payment'}
        projectId={projectId}
        projects={projectOptions}
		budgets={budgetRows}
		accounts={accounts.data ?? []}
        canCreateBudget={canManageBudget}
        canCreatePayment={canCreatePayment}
        onClose={() => setOpen(null)}
        onCreateBudget={(values) => createBudget.mutateAsync(values)}
        onCreatePayment={(values) => createPayment.mutateAsync(values)}
      />

      <AccountingDetailDialog
        detail={dialog}
        canApprove={dialog?.type === 'budget' ? canManageBudget : canApprovePayment}
		canReverse={canReverseJournal}
        approving={dialog?.type === 'budget' ? approveBudget.isPending : approvePayment.isPending}
        onClose={() => setDialog(null)}
        onApprove={() => {
          if (!dialog) return;
          if (dialog.type === 'budget') approveBudget.mutate(dialog.record.id);
          else approvePayment.mutate(dialog.record.id);
        }}
		onReverse={() => { if (dialog?.type === 'payment') setReversingPayment(dialog.record); }}
      />

	  <PaymentReversalDialog payment={reversingPayment} saving={reversePayment.isPending} onClose={() => setReversingPayment(null)} onSubmit={(reason) => reversePayment.mutateAsync({ paymentId: reversingPayment!.id, reason })} />
      <ReconciliationDialog issue={reconciliationIssue} journals={journalEntries.data ?? []} saving={resolveReconciliation.isPending} onClose={() => setReconciliationIssue(null)} onSubmit={(values) => resolveReconciliation.mutateAsync({ issueId: reconciliationIssue!.id, values })} />
    </>
  );
}

function AccountingOverview({ projectId, totals, pendingCount, pendingAmount, paymentCount, canCreatePayment, canManageBudget, onOpenBudget, onOpenPayment, onTab }: { projectId: string; totals: BudgetTotals; pendingCount: number; pendingAmount: number; paymentCount: number; canCreatePayment: boolean; canManageBudget: boolean; onOpenBudget: () => void; onOpenPayment: () => void; onTab: (tab: AccountingTab) => void }) {
  return <div className="space-y-4">
	<div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-6"><Metric label="Ngân sách đã duyệt" value={<CurrencyText value={totals.approved} />} /><Metric label="Đã cam kết" value={<CurrencyText value={totals.committed} />} /><Metric label="Thực chi ghi nhận" value={<CurrencyText value={totals.actual} />} /><Metric label="Đã thanh toán" value={<CurrencyText value={totals.paid} />} /><Metric label="Ngân sách khả dụng" value={<CurrencyText value={totals.available} />} tone={totals.available < 0 ? 'amber' : 'slate'} /><Metric label="Chứng từ chờ xử lý" value={`${pendingCount} · ${moneyLabel(pendingAmount)}`} tone={pendingCount ? 'amber' : 'slate'} /></div>
    <div className="grid gap-4 lg:grid-cols-2">
	  <ActionCard title={pendingCount ? `${pendingCount} chứng từ cần duyệt` : 'Không có chứng từ đang chờ'} description={pendingCount ? `Tổng giá trị chờ duyệt: ${moneyLabel(pendingAmount)}.` : `Đã ghi nhận ${paymentCount} chứng từ Thu/Chi trong công trình.`} actionLabel="Mở việc cần xử lý" onClick={() => onTab('tasks')} primary={pendingCount > 0} />
	  <ActionCard title="Đối chiếu sổ kế toán" description="Kiểm tra Sổ Nhật ký chung, Bảng cân đối số phát sinh và kết quả hoạt động kinh doanh." actionLabel="Mở Sổ Cái" onClick={() => onTab('ledger')} />
    </div>
	<div className="flex flex-wrap justify-end gap-2">{canCreatePayment ? <button disabled={!projectId} type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-50" onClick={onOpenPayment}><Plus className="mr-1 inline h-4 w-4" />Lập Phiếu thu/Phiếu chi</button> : null}{canManageBudget ? <button disabled={!projectId} type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-50" onClick={onOpenBudget}><Plus className="mr-1 inline h-4 w-4" />Lập ngân sách</button> : null}</div>
  </div>;
}

function AccountingTasks({ rows, issues, loading, issuesLoading, error, issuesError, canApprove, canResolve, onRetry, onRetryIssues, onView, onApprove, onResolve }: { rows: Record<string, any>[]; issues: Record<string, any>[]; loading: boolean; issuesLoading: boolean; error: unknown; issuesError: unknown; canApprove: boolean; canResolve: boolean; onRetry: () => void; onRetryIssues: () => void; onView: (row: Record<string, any>) => void; onApprove: (id: string) => void; onResolve: (row: Record<string, any>) => void }) {
  return <div className="space-y-4"><ListSection title="Chứng từ chờ duyệt" description="Ưu tiên kiểm tra số phiếu, loại nghiệp vụ, tài khoản tiền và khoản mục ngân sách trước khi ghi sổ."><DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={15} columns={[
	{ key: 'payment_code', header: 'Số phiếu', render: (row) => <span className="font-semibold text-ink">{row.payment_code}</span> },
	{ key: 'payment_type', header: 'Nghiệp vụ', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
	{ key: 'project_name', header: 'Công trình' },
	{ key: 'amount', header: 'Số tiền', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
	{ key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
	{ key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => onView(row)}>{canApprove ? <button className="text-sm font-semibold text-blue-700" onClick={() => onApprove(row.id)}><Check className="mr-1 inline h-4 w-4" />Duyệt và ghi sổ</button> : null}</RowActions> },
	]} empty="Không có chứng từ chờ xử lý" /></ListSection>
  <ListSection title="Đối chiếu dữ liệu chuyển đổi" description="Các chứng từ lịch sử chưa đủ căn cứ định khoản phải được lập CTGS bổ sung hoặc chấp nhận không chuyển sổ có giải trình."><DataTable rows={issues} loading={issuesLoading} error={issuesError} onRetry={onRetryIssues} pageSize={10} columns={[
    { key: 'source_code', header: 'Số chứng từ', render: (row) => <span className="font-semibold text-ink">{row.source_code || '-'}</span> },
    { key: 'source_type', header: 'Nguồn', render: (row) => sourceTypeLabel(row.source_type) },
    { key: 'description', header: 'Nội dung cần đối chiếu' },
    { key: 'created_at', header: 'Ngày ghi nhận', render: (row) => dateText(row.created_at) },
    { key: 'action', header: 'Thao tác', render: (row) => canResolve ? <button type="button" className="inline-flex items-center gap-1 text-sm font-semibold text-blue-700" onClick={() => onResolve(row)}><ClipboardCheck className="h-4 w-4" />Đối chiếu</button> : null },
  ]} empty="Không còn chứng từ lịch sử cần đối chiếu" /></ListSection></div>;
}

function SubViewButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
  return <button type="button" className={`rounded-md px-3 py-2 text-sm font-semibold ${active ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-500 hover:text-ink'}`} onClick={onClick}>{label}</button>;
}

function PaymentReversalDialog({ payment, saving, onClose, onSubmit }: { payment: Record<string, any> | null; saving: boolean; onClose: () => void; onSubmit: (reason: string) => Promise<unknown> }) {
  return <Dialog open={Boolean(payment)} title={`Lập chứng từ đảo · ${payment?.payment_code ?? ''}`} description="Hệ thống sẽ lập bút toán đảo, hoàn nguyên phân bổ công nợ/ngân sách và giữ nguyên chứng từ gốc để phục vụ kiểm toán." onClose={onClose} size="md">
	<SimpleForm key={payment?.id ?? 'payment-reversal'} fields={[{ name: 'reason', label: 'Lý do sửa sai', type: 'textarea', required: true, fullWidth: true, placeholder: 'Nêu rõ sai sót và chứng từ thay thế (nếu có)' }]} submitLabel={saving ? 'Đang lập chứng từ đảo...' : 'Xác nhận lập chứng từ đảo'} onCancel={onClose} onSubmit={(values) => onSubmit(String(values.reason ?? '').trim())} />
  </Dialog>;
}

function ReconciliationDialog({ issue, journals, saving, onClose, onSubmit }: { issue: Record<string, any> | null; journals: Record<string, any>[]; saving: boolean; onClose: () => void; onSubmit: (values: Record<string, any>) => Promise<unknown> }) {
  return <Dialog open={Boolean(issue)} title={`Đối chiếu chuyển đổi · ${issue?.source_code ?? ''}`} description="Chọn CTGS bổ sung nếu nghiệp vụ phải được phản ánh vào sổ; chỉ chấp nhận không chuyển sổ khi có căn cứ nghiệp vụ rõ ràng." onClose={onClose} size="md">
    <SimpleForm key={issue?.id ?? 'reconciliation'} fields={[
      { name: 'action', label: 'Kết quả đối chiếu', type: 'select', required: true, defaultValue: 'resolve', options: [{ value: 'resolve', label: 'Đã lập CTGS bổ sung' }, { value: 'accept', label: 'Chấp nhận không chuyển sổ' }] },
      { name: 'journal_entry_id', label: 'Chứng từ ghi sổ bổ sung', type: 'select', required: true, hiddenWhen: (values) => values.action !== 'resolve', options: journals.map((journal) => ({ value: journal.id, label: `${journal.entry_no} · ${journal.entry_date} · ${journal.description}` })) },
      { name: 'resolution_note', label: 'Căn cứ xử lý', type: 'textarea', required: true, fullWidth: true, placeholder: 'Nêu căn cứ định khoản, hồ sơ đối chiếu và kết luận xử lý' },
    ]} submitLabel={saving ? 'Đang hoàn tất...' : 'Hoàn tất đối chiếu'} onCancel={onClose} onSubmit={onSubmit} />
  </Dialog>;
}

function sourceTypeLabel(value: unknown) {
  return ({ payment: 'Phiếu thu/chi', input_invoice: 'Hóa đơn đầu vào', output_invoice: 'Hóa đơn đầu ra' } as Record<string, string>)[String(value)] ?? String(value ?? '-');
}

function ActionCard({ title, description, actionLabel, onClick, primary = false }: { title: string; description: string; actionLabel: string; onClick: () => void; primary?: boolean }) {
  return <div className={`rounded-lg border p-4 ${primary ? 'border-amber-200 bg-amber-50' : 'border-line bg-white'}`}><h3 className="font-semibold text-ink">{title}</h3><p className="mt-1 text-sm text-slate-600">{description}</p><button type="button" className="mt-4 inline-flex items-center gap-2 text-sm font-semibold text-blue-700" onClick={onClick}>{actionLabel}<ArrowRight className="h-4 w-4" /></button></div>;
}

function Metric({ label, value, tone = 'slate' }: { label: string; value: ReactNode; tone?: 'slate' | 'amber' }) {
  return <div className={`rounded-md border px-3 py-2 ${tone === 'amber' ? 'border-amber-200 bg-amber-50' : 'border-line bg-slate-50'}`}><div className="text-xs font-semibold uppercase text-slate-500">{label}</div><div className="mt-1 text-base font-semibold text-ink">{value}</div></div>;
}

function moneyLabel(value: number) { return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND', maximumFractionDigits: 0 }).format(value); }
function countBy(rows: Record<string, any>[], key: string, value: string) { return rows.filter((row) => row[key] === value).length; }
function currentFiscalYearStart() { return `${new Date().getFullYear()}-01-01`; }

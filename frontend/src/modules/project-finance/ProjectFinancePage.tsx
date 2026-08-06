import { type FormEvent, type ReactElement, useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, Plus, WalletCards } from 'lucide-react';
import clsx from 'clsx';
import { CurrencyText } from '../../components/CurrencyText';
import { CurrencyInput } from '../../components/CurrencyInput';
import { DataTable, type Column } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { ListSection } from '../../components/ListSection';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ProjectFinanceOptions, ProjectFinanceWorkspace } from '../../lib/types/domain';
import { currency, dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { ProjectScopeActions } from '../_shared/ProjectScopeActions';
import { useParams, useSearchParams } from 'react-router-dom';

type FinanceTab = 'overview' | 'tasks' | 'costs' | 'requests' | 'report';
type FinanceRow = Record<string, any>;

const tabs: Array<{ value: FinanceTab; label: string }> = [
  { value: 'overview', label: 'Tổng quan' },
  { value: 'tasks', label: 'Việc cần xử lý' },
  { value: 'costs', label: 'Chi phí vật tư' },
  { value: 'requests', label: 'Đề nghị thu/chi' },
  { value: 'report', label: 'Báo cáo quản trị' },
];

export function ProjectFinancePage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab') as FinanceTab | null;
  const requestedProjectId = searchParams.get('project') ?? '';
  const [tab, setTab] = useState<FinanceTab>(tabs.some((item) => item.value === requestedTab) ? requestedTab! : 'overview');
  const [draftProjectId, setDraftProjectId] = useState(requestedProjectId);
  const [dialog, setDialog] = useState<'cost' | 'request' | null>(null);
  const [detail, setDetail] = useState<{ kind: 'cost' | 'request'; row: FinanceRow } | null>(null);
  const [actionPrompt, setActionPrompt] = useState<{ kind: 'cost' | 'request'; row: FinanceRow; actionName: string } | null>(null);
  const { hasAnyPermission } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const projectId = id ?? (global ? requestedProjectId : '');

  const workspace = useQuery({
    queryKey: ['project-finance', projectId],
    queryFn: () => get<ProjectFinanceWorkspace>(`/projects/${projectId}/project-finance`),
    enabled: Boolean(projectId),
  });
  const options = useQuery({
    queryKey: ['project-finance-options', projectId],
    queryFn: () => get<ProjectFinanceOptions>(`/projects/${projectId}/project-finance/options`),
    enabled: Boolean(projectId) && (hasAnyPermission(['project_finance.cost_create']) || hasAnyPermission(['project_finance.request_create'])),
  });
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['project-finance', projectId] });
  const action = useMutation({
    mutationFn: ({ kind, row, actionName, comment }: { kind: 'cost' | 'request'; row: FinanceRow; actionName: string; comment: string }) =>
      post(`/project-finance/${kind === 'cost' ? 'material-costs' : 'cash-requests'}/${row.id}/${actionName}`, { comment }),
    onSuccess: async (_, variables) => {
      await refresh();
      setDetail(null);
      setActionPrompt(null);
      toast.success(actionSuccessText(variables.kind, variables.actionName));
    },
    onError: (error) => toast.error('Không cập nhật được hồ sơ tài chính công trình', error),
  });

  const canCreateCost = hasAnyPermission(['project_finance.cost_create']);
  const canCreateRequest = hasAnyPermission(['project_finance.request_create']);
  const canReviewCost = hasAnyPermission(['project_finance.cost_review']);
  const canApproveRequest = hasAnyPermission(['project_finance.request_approve']);
  const canProcessRequest = hasAnyPermission(['project_finance.process']);
  const canViewReport = hasAnyPermission(['project_finance.report_view']);
  const data = workspace.data;
  const reportVisible = canViewReport && data?.can_view_report !== false;
  const visibleTabs = tabs.filter((item) => item.value !== 'report' || reportVisible);

  useEffect(() => {
    if (global) setDraftProjectId(requestedProjectId);
  }, [global, requestedProjectId]);

  useEffect(() => {
    if (tab === 'report' && !reportVisible) selectTab('overview');
  // selectTab is intentionally excluded because this effect follows permission/data changes only.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, reportVisible]);

  function selectTab(next: FinanceTab) {
    setTab(next);
    const params = new URLSearchParams(searchParams);
    params.set('tab', next);
    setSearchParams(params, { replace: true });
  }

  function applyProjectScope() {
    if (!draftProjectId) return;
    const params = new URLSearchParams(searchParams);
    params.set('project', draftProjectId);
    setSearchParams(params, { replace: true });
    setDialog(null);
    setDetail(null);
    setActionPrompt(null);
  }

  function resetProjectScope() {
    const params = new URLSearchParams(searchParams);
    params.delete('project');
    setSearchParams(params, { replace: true });
    setDraftProjectId('');
    setDialog(null);
    setDetail(null);
    setActionPrompt(null);
  }

  async function exportReport() {
    if (!projectId) return;
    try {
      const blob = await downloadBlob(`/projects/${projectId}/project-finance/export`);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'project-finance-material-costs.csv';
      anchor.click();
      URL.revokeObjectURL(url);
      toast.success('Đã xuất báo cáo chi phí vật tư');
    } catch (error) {
      toast.error('Không xuất được báo cáo', error);
    }
  }

  return (
    <div className="space-y-5">
      {id ? <ProjectBackLink /> : null}
      <header className="border-b border-line pb-5">
        <div className={clsx('grid gap-4', global && 'xl:grid-cols-[minmax(0,1fr)_36rem] xl:items-end xl:gap-8')}>
          <div className="min-w-0">
            <h1 className="text-xl font-semibold tracking-normal text-ink md:text-2xl">Tài chính công trình</h1>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">Ghi nhận chi phí mua vật tư, theo dõi hồ sơ đề nghị thu/chi và cung cấp số liệu quản trị cho Ban giám đốc. Không thay thế MISA, sổ kế toán hoặc kê khai thuế.</p>
          </div>

          {global ? <section className="min-w-0 border-t border-line pt-4 xl:border-0 xl:pt-0" aria-label="Bộ lọc công trình tài chính">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <AsyncProjectPicker
                value={draftProjectId}
                onChange={setDraftProjectId}
                endpoint="/project-finance/projects"
                queryKey="project-finance-project-options"
                emptyLabel="Chọn công trình"
                allowEmptySelection={false}
              />
              <ProjectScopeActions
                applying={workspace.isFetching}
                applyDisabled={!draftProjectId || draftProjectId === projectId}
                resetDisabled={workspace.isFetching || (!draftProjectId && !projectId)}
                onApply={applyProjectScope}
                onReset={resetProjectScope}
              />
            </div>
          </section> : null}
        </div>
      </header>

      {!projectId ? <EmptyState title="Chọn công trình để xem tài chính công trình." /> : <>
        <div className="flex flex-col gap-3 border-b border-line sm:flex-row sm:items-end sm:justify-between">
          <nav className="order-2 flex min-w-0 gap-1 overflow-x-auto sm:order-1" aria-label="Nội dung tài chính công trình">
            {visibleTabs.map((item) => <button key={item.value} className={clsx('whitespace-nowrap border-b-2 px-3 py-2.5 text-sm font-semibold', tab === item.value ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink')} onClick={() => selectTab(item.value)}>{item.label}{item.value === 'tasks' && data?.tasks?.length ? <span className="ml-2 rounded-full bg-amber-100 px-1.5 py-0.5 text-xs text-amber-800">{data.tasks.length}</span> : null}</button>)}
          </nav>
          {canCreateCost || canCreateRequest ? <div className="order-1 flex flex-wrap gap-2 pb-0 sm:order-2 sm:pb-2">
            {canCreateCost ? <button className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60" disabled={workspace.isFetching} onClick={() => setDialog('cost')}><Plus className="h-4 w-4" /> Ghi nhận chi phí vật tư</button> : null}
            {canCreateRequest ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={workspace.isFetching} onClick={() => setDialog('request')}><WalletCards className="h-4 w-4" /> Lập đề nghị thu/chi</button> : null}
          </div> : null}
        </div>

        {workspace.isLoading ? <div className="rounded-lg border border-line bg-white p-8 text-sm text-slate-600">Đang tải số liệu tài chính công trình...</div> : null}
        {workspace.isError ? <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">Không tải được số liệu. Vui lòng kiểm tra quyền và thử lại.</div> : null}
        {data && tab === 'overview' ? <Overview data={data} canViewReport={reportVisible} onOpenTasks={() => selectTab('tasks')} /> : null}
        {data && tab === 'tasks' ? <Tasks rows={data.tasks} onOpen={(row) => {
          const kind = row.task_type === 'material_cost' ? 'cost' : 'request';
          const fullRow = kind === 'cost' ? data.material_costs.find((item) => item.id === row.id) : data.cash_requests.find((item) => item.id === row.id);
          setDetail({ kind, row: fullRow ?? row });
        }} /> : null}
        {data && tab === 'costs' ? <Costs rows={data.material_costs} onOpen={(row) => setDetail({ kind: 'cost', row })} /> : null}
        {data && tab === 'requests' ? <Requests rows={data.cash_requests} onOpen={(row) => setDetail({ kind: 'request', row })} /> : null}
        {data && tab === 'report' && reportVisible ? <ManagementReport data={data} canExport={canViewReport} onExport={exportReport} /> : null}
      </>}

      <MaterialCostDialog open={dialog === 'cost'} projectId={projectId} options={options.data} onClose={() => setDialog(null)} onSaved={async () => { setDialog(null); await refresh(); }} />
      <CashRequestDialog open={dialog === 'request'} projectId={projectId} onClose={() => setDialog(null)} onSaved={async () => { setDialog(null); await refresh(); }} />
      <FinanceDetailDialog detail={detail} onClose={() => setDetail(null)} onAction={(kind, row, actionName) => setActionPrompt({ kind, row, actionName })} pending={action.isPending} permissions={{ canCreateCost, canCreateRequest, canReviewCost, canApproveRequest, canProcessRequest }} />
      <FinanceActionConfirmDialog prompt={actionPrompt} pending={action.isPending} onClose={() => setActionPrompt(null)} onConfirm={(comment) => actionPrompt && action.mutate({ ...actionPrompt, comment })} />
    </div>
  );
}

function Overview({ data, canViewReport, onOpenTasks }: { data: ProjectFinanceWorkspace; canViewReport: boolean; onOpenTasks: () => void }) {
  const summary = data.summary;
  return <div className="space-y-5">
    {canViewReport ? <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Metric label="Giá trị hợp đồng" value={currency(summary.contract_value)} note="Hợp đồng chính đang hiệu lực" />
      <Metric label="Doanh thu đủ điều kiện" value={currency(summary.eligible_revenue)} note="Theo mốc hợp đồng đã đủ điều kiện" />
      <Metric label="Chi phí vật tư đã xác nhận" value={currency(summary.confirmed_material_cost)} note={`Đã đặt/dự kiến: ${currency(summary.committed_material_cost)}`} />
      <Metric label="Lợi nhuận quản trị tạm tính" value={currency(summary.provisional_profit)} note="Doanh thu đủ điều kiện − chi phí vật tư đã xác nhận" emphasized />
    </div> : null}
    {canViewReport ? <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900"><strong>Phạm vi số liệu:</strong> {summary.scope_warning}</div> : null}
    <div className="grid gap-4 lg:grid-cols-2">
      <section className="rounded-lg border border-line bg-white p-4 shadow-panel"><h2 className="font-semibold text-ink">Hồ sơ cần xử lý</h2><p className="mt-1 text-sm text-slate-600">{data.tasks.length} hồ sơ chưa hoàn tất.</p><button className="mt-4 text-sm font-semibold text-blue-700 hover:underline" onClick={onOpenTasks}>Mở danh sách việc cần xử lý →</button></section>
      {canViewReport ? <section className="rounded-lg border border-line bg-white p-4 shadow-panel"><h2 className="font-semibold text-ink">Đề nghị thu/chi đang mở</h2><div className="mt-3 grid grid-cols-2 gap-3"><Metric label="Đề nghị thu" value={currency(summary.open_inbound_amount)} compact /><Metric label="Đề nghị chi" value={currency(summary.open_outbound_amount)} compact /></div></section> : <section className="rounded-lg border border-line bg-white p-4 shadow-panel"><h2 className="font-semibold text-ink">Phạm vi thao tác</h2><p className="mt-1 text-sm leading-6 text-slate-600">Anh/chị có thể cập nhật hồ sơ được phân công. Số liệu doanh thu và lợi nhuận quản trị chỉ hiển thị cho vai trò được cấp quyền báo cáo.</p></section>}
    </div>
  </div>;
}

function Metric({ label, value, note, emphasized = false, compact = false }: { label: string; value: string; note?: string; emphasized?: boolean; compact?: boolean }) {
  return <div className={clsx('rounded-lg border p-4', emphasized ? 'border-emerald-200 bg-emerald-50' : 'border-line bg-white', !compact && 'shadow-panel')}><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><p className={clsx('mt-2 font-semibold tabular-nums text-ink', compact ? 'text-lg' : 'text-xl')}>{value}</p>{note ? <p className="mt-1 text-xs leading-5 text-slate-500">{note}</p> : null}</div>;
}

function Tasks({ rows, onOpen }: { rows: FinanceRow[]; onOpen: (row: FinanceRow) => void }) {
  const columns: Column<FinanceRow>[] = [
    { key: 'task_type', header: 'Loại', render: (row) => row.task_type === 'material_cost' ? 'Chi phí vật tư' : 'Đề nghị thu/chi' },
    { key: 'title', header: 'Nội dung', render: (row) => <button className="text-left font-semibold text-blue-700 hover:underline" onClick={() => onOpen(row)}>{row.title}</button> },
    { key: 'counterparty', header: 'Đối tượng' },
    { key: 'amount', header: 'Giá trị', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
  ];
  return <ListSection title="Việc cần xử lý" description="Ưu tiên hồ sơ cần bổ sung, đang chờ xác nhận hoặc chưa bàn giao Kế toán."><DataTable rows={rows} columns={columns} empty="Không còn hồ sơ tài chính cần xử lý." searchPlaceholder="Tìm nội dung, đối tượng..." pageSize={15} /></ListSection>;
}

function Costs({ rows, onOpen }: { rows: FinanceRow[]; onOpen: (row: FinanceRow) => void }) {
  const columns: Column<FinanceRow>[] = [
    { key: 'invoice_date', header: 'Ngày HĐ', sortable: true, render: (row) => dateText(row.invoice_date) },
    { key: 'invoice_number', header: 'Số hóa đơn', render: (row) => row.invoice_number || '-' },
    { key: 'vendor_name', header: 'Nhà cung cấp' },
    { key: 'description', header: 'Nội dung', render: (row) => <button className="text-left font-semibold text-blue-700 hover:underline" onClick={() => onOpen(row)}>{row.description}</button> },
    { key: 'total_amount', header: 'Tổng thanh toán', align: 'right', render: (row) => <CurrencyText value={row.total_amount} /> },
    { key: 'recognized_cost_amount', header: 'Chi phí xác nhận', align: 'right', render: (row) => <CurrencyText value={row.recognized_cost_amount} /> },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
  ];
  return <ListSection title="Chi phí mua vật tư" description="Chỉ hồ sơ đã xác nhận mới được cộng vào chi phí và lợi nhuận quản trị tạm tính."><DataTable rows={rows} columns={columns} empty="Chưa có chi phí vật tư." searchPlaceholder="Tìm hóa đơn, nhà cung cấp, nội dung..." pageSize={15} /></ListSection>;
}

function Requests({ rows, onOpen }: { rows: FinanceRow[]; onOpen: (row: FinanceRow) => void }) {
  const columns: Column<FinanceRow>[] = [
    { key: 'request_code', header: 'Số đề nghị', render: (row) => <button className="font-semibold text-blue-700 hover:underline" onClick={() => onOpen(row)}>{row.request_code}</button> },
    { key: 'direction', header: 'Loại', render: (row) => row.direction === 'inbound' ? 'Đề nghị thu' : 'Đề nghị chi' },
    { key: 'title', header: 'Nội dung' },
    { key: 'counterparty_name', header: 'Đối tượng' },
    { key: 'amount', header: 'Số tiền', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
    { key: 'due_date', header: 'Hạn xử lý', render: (row) => dateText(row.due_date) },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
  ];
  return <ListSection title="Đề nghị thu/chi nội bộ" description="Theo dõi hồ sơ từ lúc lập đến khi bàn giao và được Kế toán xử lý; không ghi nhận bút toán hay thanh toán trên Consflow."><DataTable rows={rows} columns={columns} empty="Chưa có đề nghị thu/chi." searchPlaceholder="Tìm số đề nghị, đối tượng, nội dung..." pageSize={15} /></ListSection>;
}

function ManagementReport({ data, canExport, onExport }: { data: ProjectFinanceWorkspace; canExport: boolean; onExport: () => void }) {
  const summary = data.summary;
  return <div className="space-y-4">
    <ListSection title="Báo cáo quản trị công trình" description="Số liệu điều hành, không phải báo cáo tài chính, báo cáo thuế hay sổ kế toán." actions={canExport ? <button className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onExport}><Download className="h-4 w-4" /> Xuất CSV chi phí vật tư</button> : null}>
      <div className="overflow-hidden rounded-lg border border-line bg-white shadow-panel"><dl className="divide-y divide-line">{[
        ['Giá trị hợp đồng đang hiệu lực', summary.contract_value],['Doanh thu đủ điều kiện',summary.eligible_revenue],['Giá trị mua vật tư đã cam kết/dự kiến',summary.committed_material_cost],['Chi phí vật tư đã xác nhận',summary.confirmed_material_cost],['Lợi nhuận quản trị tạm tính',summary.provisional_profit],
      ].map(([label, value]) => <div key={String(label)} className="flex items-center justify-between gap-4 px-4 py-3"><dt className="text-sm text-slate-600">{label}</dt><dd className="font-semibold tabular-nums text-ink">{currency(value)}</dd></div>)}</dl></div>
    </ListSection>
    <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-900"><strong>Cảnh báo phạm vi:</strong> {summary.scope_warning} Còn {summary.unconfirmed_cost_count} hồ sơ chi phí trị giá {currency(summary.unconfirmed_cost_amount)} chưa được xác nhận.</div>
  </div>;
}

function MaterialCostDialog({ open, projectId, options, onClose, onSaved }: { open: boolean; projectId: string; options?: ProjectFinanceOptions; onClose: () => void; onSaved: () => void }) {
  const toast = useToast();
  const [values, setValues] = useState({ source_key: '', vendor_name: '', invoice_number: '', invoice_date: '', description: '', amount_before_vat: '', vat_amount: '0', notes: '' });
  const [errors, setErrors] = useState<Record<string, boolean>>({});
  const create = useMutation({ mutationFn: (payload: Record<string, any>) => post(`/projects/${projectId}/project-finance/material-costs`, payload), onSuccess: () => { toast.success('Đã lưu nháp chi phí vật tư'); onSaved(); }, onError: (error) => toast.error('Không lưu được chi phí vật tư', error) });
  const total = Number(values.amount_before_vat || 0) + Number(values.vat_amount || 0);
  useEffect(() => { if (open) { setValues({ source_key: '', vendor_name: '', invoice_number: '', invoice_date: '', description: '', amount_before_vat: '', vat_amount: '0', notes: '' }); setErrors({}); } }, [open, projectId]);
  function submit(event: FormEvent) { event.preventDefault(); const next = { source_key: !values.source_key, vendor_name: !values.vendor_name.trim(), description: !values.description.trim(), amount_before_vat: Number(values.amount_before_vat) <= 0 }; setErrors(next); if (Object.values(next).some(Boolean)) { toast.warn({ title: 'Chưa đủ thông tin', description: 'Cần chọn hồ sơ nguồn, nhà cung cấp, nội dung và giá trị trước VAT.' }); return; } const [sourceKind, sourceId] = values.source_key.split(':'); create.mutate({ ...values, source_key: undefined, procurement_request_id: sourceKind === 'procurement' ? sourceId : '', incoming_acceptance_id: sourceKind === 'acceptance' ? sourceId : '', stock_document_id: sourceKind === 'stock' ? sourceId : '', amount_before_vat: Number(values.amount_before_vat), vat_amount: Number(values.vat_amount), total_amount: total }); }
  return <Dialog open={open} title="Ghi nhận chi phí mua vật tư" description="Liên kết hồ sơ mua sắm nguồn. Số liệu chỉ lên báo cáo sau khi được xác nhận." onClose={onClose} size="lg" footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Hủy</button><button className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={create.isPending} onClick={() => document.getElementById('material-cost-form-submit')?.click()}>Lưu nháp</button></div>}><form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
    <Field label="Hồ sơ mua sắm nguồn *" error={errors.source_key}><select value={values.source_key} onChange={(event) => setValues({ ...values, source_key: event.target.value })}><option value="">{options ? 'Chọn hồ sơ nguồn' : 'Đang tải hồ sơ nguồn...'}</option><optgroup label="Yêu cầu mua">{options?.procurements?.map((row) => <option key={row.id} value={`procurement:${row.id}`}>{row.request_code} · {row.title}</option>)}</optgroup><optgroup label="Nghiệm thu đầu vào">{options?.incoming_acceptances?.map((row) => <option key={row.id} value={`acceptance:${row.id}`}>{row.acceptance_code} · {row.supplier_name}</option>)}</optgroup><optgroup label="Phiếu kho đã ghi nhận">{options?.stock_documents?.map((row) => <option key={row.id} value={`stock:${row.id}`}>{row.document_code} · {dateText(row.document_date)}</option>)}</optgroup></select></Field>
    <Field label="Nhà cung cấp *" error={errors.vendor_name}><input value={values.vendor_name} onChange={(event) => setValues({ ...values, vendor_name: event.target.value })} /></Field>
    <Field label="Số hóa đơn"><input value={values.invoice_number} onChange={(event) => setValues({ ...values, invoice_number: event.target.value })} /></Field>
    <Field label="Ngày hóa đơn"><input type="date" value={values.invoice_date} onChange={(event) => setValues({ ...values, invoice_date: event.target.value })} /></Field>
    <Field label="Nội dung chi phí *" error={errors.description} wide><input value={values.description} onChange={(event) => setValues({ ...values, description: event.target.value })} /></Field>
    <Field label="Tiền trước VAT *" error={errors.amount_before_vat}><CurrencyInput min={0} invalid={Boolean(errors.amount_before_vat)} value={values.amount_before_vat} onValueChange={({ numericString }) => setValues({ ...values, amount_before_vat: numericString })} /></Field>
    <Field label="Tiền VAT"><CurrencyInput min={0} value={values.vat_amount} onValueChange={({ numericString }) => setValues({ ...values, vat_amount: numericString })} /></Field>
    <div className="rounded-md bg-slate-50 p-3 sm:col-span-2"><p className="text-xs font-semibold uppercase text-slate-500">Tổng thanh toán</p><p className="mt-1 text-lg font-semibold">{currency(total)}</p></div>
    <Field label="Ghi chú" wide><textarea rows={3} value={values.notes} onChange={(event) => setValues({ ...values, notes: event.target.value })} /></Field>
    <button id="material-cost-form-submit" className="hidden" type="submit">Lưu</button>
  </form></Dialog>;
}

function CashRequestDialog({ open, projectId, onClose, onSaved }: { open: boolean; projectId: string; onClose: () => void; onSaved: () => void }) {
  const toast = useToast();
  const [values, setValues] = useState({ direction: 'outbound', title: '', counterparty_name: '', amount: '', due_date: '', notes: '' });
  const [errors, setErrors] = useState<Record<string, boolean>>({});
  const create = useMutation({ mutationFn: (payload: Record<string, any>) => post(`/projects/${projectId}/project-finance/cash-requests`, payload), onSuccess: () => { toast.success('Đã lưu nháp đề nghị thu/chi'); onSaved(); }, onError: (error) => toast.error('Không lưu được đề nghị thu/chi', error) });
  useEffect(() => { if (open) { setValues({ direction: 'outbound', title: '', counterparty_name: '', amount: '', due_date: '', notes: '' }); setErrors({}); } }, [open, projectId]);
  function submit(event: FormEvent) { event.preventDefault(); const next = { title: !values.title.trim(), counterparty_name: !values.counterparty_name.trim(), amount: Number(values.amount) <= 0 }; setErrors(next); if (Object.values(next).some(Boolean)) { toast.warn({ title: 'Chưa đủ thông tin', description: 'Cần nhập nội dung, đối tượng và số tiền.' }); return; } create.mutate({ ...values, amount: Number(values.amount) }); }
  return <Dialog open={open} title="Lập đề nghị thu/chi nội bộ" description="Hồ sơ được duyệt và bàn giao Kế toán; Consflow không tự ghi sổ hoặc ghi nhận đã thanh toán." onClose={onClose} size="lg" footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Hủy</button><button className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={create.isPending} onClick={() => document.getElementById('cash-request-form-submit')?.click()}>Lưu nháp</button></div>}><form className="grid gap-4 sm:grid-cols-2" onSubmit={submit}>
    <Field label="Loại đề nghị *"><select value={values.direction} onChange={(event) => setValues({ ...values, direction: event.target.value })}><option value="outbound">Đề nghị chi</option><option value="inbound">Đề nghị thu</option></select></Field>
    <Field label="Hạn xử lý"><input type="date" value={values.due_date} onChange={(event) => setValues({ ...values, due_date: event.target.value })} /></Field>
    <Field label="Nội dung *" error={errors.title} wide><input value={values.title} onChange={(event) => setValues({ ...values, title: event.target.value })} /></Field>
    <Field label="Đối tượng thu/chi *" error={errors.counterparty_name}><input value={values.counterparty_name} onChange={(event) => setValues({ ...values, counterparty_name: event.target.value })} /></Field>
    <Field label="Số tiền *" error={errors.amount}><CurrencyInput min={1} invalid={Boolean(errors.amount)} value={values.amount} onValueChange={({ numericString }) => setValues({ ...values, amount: numericString })} /></Field>
    <Field label="Ghi chú / hồ sơ cần bàn giao" wide><textarea rows={3} value={values.notes} onChange={(event) => setValues({ ...values, notes: event.target.value })} /></Field>
    <button id="cash-request-form-submit" className="hidden" type="submit">Lưu</button>
  </form></Dialog>;
}

function Field({ label, error, wide = false, children }: { label: string; error?: boolean; wide?: boolean; children: ReactElement }) {
  return <label className={clsx('block text-sm font-medium text-slate-700 [&_input]:mt-1 [&_input]:w-full [&_input]:rounded-md [&_input]:border [&_input]:px-3 [&_input]:py-2 [&_select]:mt-1 [&_select]:w-full [&_select]:rounded-md [&_select]:border [&_select]:px-3 [&_select]:py-2 [&_textarea]:mt-1 [&_textarea]:w-full [&_textarea]:rounded-md [&_textarea]:border [&_textarea]:px-3 [&_textarea]:py-2', wide && 'sm:col-span-2', error ? '[&_input]:border-rose-400 [&_select]:border-rose-400 [&_textarea]:border-rose-400' : '[&_input]:border-line [&_select]:border-line [&_textarea]:border-line')}>{label}{children}</label>;
}

function FinanceDetailDialog({ detail, onClose, onAction, pending, permissions }: { detail: { kind: 'cost' | 'request'; row: FinanceRow } | null; onClose: () => void; onAction: (kind: 'cost' | 'request', row: FinanceRow, action: string) => void; pending: boolean; permissions: { canCreateCost: boolean; canCreateRequest: boolean; canReviewCost: boolean; canApproveRequest: boolean; canProcessRequest: boolean } }) {
  if (!detail) return null;
  const { kind, row } = detail;
  const actions: Array<{ key: string; label: string }> = [];
  if (kind === 'cost' && permissions.canCreateCost && ['draft','needs_changes'].includes(row.status)) actions.push({ key: 'submit', label: 'Gửi xác nhận' });
  if (kind === 'cost' && permissions.canReviewCost && row.status === 'submitted') actions.push({ key: 'confirm', label: 'Xác nhận chi phí' });
  if (kind === 'cost' && permissions.canReviewCost && row.status === 'submitted') actions.push({ key: 'request-changes', label: 'Yêu cầu bổ sung' });
  if (kind === 'request' && ['draft','needs_changes'].includes(row.status) && permissions.canCreateRequest) actions.push({ key: 'submit', label: 'Gửi duyệt' });
  if (kind === 'request' && row.status === 'submitted' && permissions.canApproveRequest) actions.push({ key: 'approve', label: 'Duyệt đề nghị' });
  if (kind === 'request' && ['submitted','approved'].includes(row.status) && permissions.canApproveRequest) actions.push({ key: 'request-changes', label: 'Yêu cầu bổ sung' });
  if (kind === 'request' && row.status === 'approved' && permissions.canProcessRequest) actions.push({ key: 'handoff', label: 'Xác nhận bàn giao Kế toán' });
  if (kind === 'request' && row.status === 'handed_to_accounting' && permissions.canProcessRequest) actions.push({ key: 'process', label: 'Xác nhận đã xử lý' });
  return <Dialog open title={kind === 'cost' ? 'Chi tiết chi phí vật tư' : 'Chi tiết đề nghị thu/chi'} description={kind === 'cost' ? row.description : `${row.request_code} · ${row.title}`} onClose={onClose} footer={<div className="flex flex-wrap justify-end gap-2"><button className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Đóng</button>{actions.map((item) => <button key={item.key} className={clsx('rounded-md px-4 py-2 text-sm font-semibold disabled:opacity-60', item.key === 'request-changes' ? 'border border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100' : 'bg-blue-600 text-white hover:bg-blue-700')} disabled={pending} onClick={() => onAction(kind,row,item.key)}>{item.label}</button>)}</div>}><div className="grid gap-3 sm:grid-cols-2">{(kind === 'cost' ? [['Trạng thái',<StatusBadge status={row.status} />],['Nhà cung cấp',row.vendor_name],['Số hóa đơn',row.invoice_number||'-'],['Ngày hóa đơn',dateText(row.invoice_date)],['Tổng thanh toán',currency(row.total_amount)],['Chi phí đã xác nhận',currency(row.recognized_cost_amount)],['Hồ sơ nguồn',row.request_code||row.acceptance_code||row.document_code||'-'],['Ghi chú',row.notes||'-'],['Phản hồi',row.review_comment||'-']] : [['Trạng thái',<StatusBadge status={row.status} />],['Loại',row.direction==='inbound'?'Đề nghị thu':'Đề nghị chi'],['Đối tượng',row.counterparty_name],['Số tiền',currency(row.amount)],['Hạn xử lý',dateText(row.due_date)],['Người lập',row.created_by_name||'-'],['Ghi chú',row.notes||'-'],['Phản hồi',row.review_comment||'-']]).map(([label,value], index) => <div key={`${label}-${index}`} className="rounded-md bg-slate-50 p-3"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><div className="mt-1 text-sm font-semibold text-ink">{value}</div></div>)}</div></Dialog>;
}

function FinanceActionConfirmDialog({ prompt, pending, onClose, onConfirm }: { prompt: { kind: 'cost' | 'request'; row: FinanceRow; actionName: string } | null; pending: boolean; onClose: () => void; onConfirm: (comment: string) => void }) {
  const [comment, setComment] = useState('');
  useEffect(() => { setComment(''); }, [prompt]);
  if (!prompt) return null;
  const requiresComment = prompt.actionName === 'request-changes';
  const presentation = actionPresentation(prompt.kind, prompt.actionName);
  return <Dialog open title={presentation.title} description={presentation.description} onClose={onClose} size="sm" footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Quay lại</button><button className={clsx('rounded-md px-4 py-2 text-sm font-semibold text-white disabled:opacity-60', requiresComment ? 'bg-amber-600 hover:bg-amber-700' : 'bg-blue-600 hover:bg-blue-700')} disabled={pending || (requiresComment && !comment.trim())} onClick={() => onConfirm(comment.trim())}>{presentation.confirmLabel}</button></div>}>
    <label className="block text-sm font-medium text-slate-700">{requiresComment ? 'Nội dung cần bổ sung *' : 'Ghi chú (không bắt buộc)'}<textarea className={clsx('mt-1 w-full rounded-md border px-3 py-2', requiresComment && !comment.trim() ? 'border-amber-400' : 'border-line')} rows={4} value={comment} onChange={(event) => setComment(event.target.value)} placeholder={requiresComment ? 'Nêu rõ hồ sơ hoặc số liệu cần bổ sung' : 'Ghi chú cho lần xử lý này'} /></label>
    {prompt.actionName === 'process' ? <p className="mt-3 rounded-md bg-blue-50 p-3 text-sm text-blue-800">Xác nhận này chỉ ghi nhận hồ sơ đã được Kế toán xử lý trên hệ thống chuyên môn; không đồng nghĩa Consflow đã thanh toán hoặc ghi sổ.</p> : null}
  </Dialog>;
}

function actionPresentation(kind: 'cost' | 'request', actionName: string) {
  const presentations: Record<string, { title: string; description: string; confirmLabel: string }> = {
    'cost-submit': { title: 'Gửi xác nhận chi phí?', description: 'Hồ sơ sẽ chuyển sang trạng thái chờ Kế toán kiểm tra.', confirmLabel: 'Gửi xác nhận' },
    'cost-confirm': { title: 'Xác nhận chi phí vật tư?', description: 'Toàn bộ giá trị hồ sơ sẽ được cộng vào chi phí vật tư và lợi nhuận quản trị tạm tính.', confirmLabel: 'Xác nhận chi phí' },
    'cost-request-changes': { title: 'Yêu cầu bổ sung hồ sơ?', description: 'Người lập cần chỉnh lại hồ sơ trước khi gửi xác nhận lần nữa.', confirmLabel: 'Gửi yêu cầu' },
    'request-submit': { title: 'Gửi duyệt đề nghị?', description: 'Đề nghị sẽ chuyển đến cấp có thẩm quyền để kiểm tra.', confirmLabel: 'Gửi duyệt' },
    'request-approve': { title: 'Duyệt đề nghị thu/chi?', description: 'Hồ sơ đủ điều kiện để bàn giao cho Kế toán xử lý trên hệ thống chuyên môn.', confirmLabel: 'Duyệt đề nghị' },
    'request-request-changes': { title: 'Yêu cầu bổ sung đề nghị?', description: 'Người lập cần chỉnh lại hồ sơ trước khi gửi duyệt lần nữa.', confirmLabel: 'Gửi yêu cầu' },
    'request-handoff': { title: 'Xác nhận đã bàn giao Kế toán?', description: 'Consflow chỉ theo dõi việc bàn giao hồ sơ, không tự tạo bút toán hoặc thanh toán.', confirmLabel: 'Xác nhận bàn giao' },
    'request-process': { title: 'Xác nhận hồ sơ đã được xử lý?', description: 'Hồ sơ sẽ được coi là hoàn tất trong workflow tài chính công trình.', confirmLabel: 'Xác nhận đã xử lý' },
  };
  return presentations[`${kind}-${actionName}`] ?? { title: 'Xác nhận thao tác?', description: 'Vui lòng kiểm tra hồ sơ trước khi tiếp tục.', confirmLabel: 'Xác nhận' };
}

function actionSuccessText(kind: 'cost' | 'request', actionName: string) {
  const labels: Record<string, string> = { submit: 'Đã gửi hồ sơ', confirm: 'Đã xác nhận chi phí vật tư', approve: 'Đã duyệt đề nghị', 'request-changes': 'Đã gửi yêu cầu bổ sung', handoff: 'Đã xác nhận bàn giao Kế toán', process: 'Đã xác nhận hồ sơ được xử lý' };
  return labels[actionName] ?? (kind === 'cost' ? 'Đã cập nhật chi phí vật tư' : 'Đã cập nhật đề nghị thu/chi');
}

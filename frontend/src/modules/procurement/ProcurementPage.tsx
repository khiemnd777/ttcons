import { useMemo, useState } from 'react';
import { Link, useLocation, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Check, ClipboardList, Clock3, Eye, Loader2, PackageCheck, Pencil, Search, Send, Truck } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type { Partner } from '../../lib/types/domain';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { StatCard } from '../../components/StatCard';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { partnerOptions } from '../core-data/partnerUtils';
import { useAuth } from '../../lib/auth/AuthContext';
import { workItemDetailPath } from '../work-items/workItemNavigation';
import {
  syncWorkItemMutationTransition,
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
  type WorkItemMutationResult,
} from '../work-items/workItemMutationContext';

type ProcurementRow = Record<string, any>;
type QueueView = 'attention' | 'delivery' | 'all';

const procurementActionLabels: Record<string, string> = {
  submit: 'Đã gửi yêu cầu mua để duyệt',
  approve: 'Đã duyệt yêu cầu mua',
  order: 'Đã ghi nhận đặt hàng',
  receive: 'Đã đối soát khối lượng nghiệm thu đầu vào',
};

const procurementStatusLabels: Record<string, string> = {
  draft: 'Nháp', waiting_approval: 'Chờ duyệt', submitted: 'Chờ duyệt', approved: 'Chờ đặt hàng',
  ordered: 'Đang giao / nghiệm thu', partially_received: 'Đạt nghiệm thu một phần', received: 'Đã nghiệm thu đủ',
  changes_requested: 'Cần chỉnh sửa',
};

export function ProcurementPage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const location = useLocation();
  const taskId = new URLSearchParams(location.search).get('task')?.trim() ?? '';
  const { hasPermission } = useAuth();
  const [projectId, setProjectId] = useState(id ?? '');
  const [view, setView] = useState<QueueView>('attention');
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [supplierFilter, setSupplierFilter] = useState('all');
  const [viewId, setViewId] = useState('');
  const [editRecord, setEditRecord] = useState<ProcurementRow | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const listPath = projectId ? `/procurement-requests?project_id=${encodeURIComponent(projectId)}` : '/procurement-requests';
  const requests = useQuery({ queryKey: ['procurement', projectId], queryFn: () => get<ProcurementRow[]>(listPath) });
  const detail = useQuery({ queryKey: ['procurement-detail', viewId], queryFn: () => get<ProcurementRow>(`/procurement-requests/${viewId}`), enabled: Boolean(viewId) });
  const partners = useQuery({ queryKey: ['partners', 'supplier', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const invalidate = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ['procurement'] }),
    queryClient.invalidateQueries({ queryKey: ['procurement-detail'] }),
  ]);
  const update = useMutation({
    mutationFn: ({ requestId, values }: { requestId: string; values: ProcurementRow }) => put(`/procurement-requests/${requestId}`, values),
    onSuccess: async () => { await invalidate(); setEditRecord(null); toast.success('Đã cập nhật thông tin thương mại yêu cầu mua'); },
    onError: (error) => toast.error('Không cập nhật được yêu cầu mua', error),
  });
  const action = useMutation({
    mutationFn: ({ requestId, name }: { requestId: string; name: string }) => post<WorkItemMutationResult>(`/procurement-requests/${requestId}/${name}`, withWorkItemMutationContext({}, taskId)),
    onSuccess: async (result, variables) => { await invalidate(); syncWorkItemMutationTransition(queryClient, result); toast.success(`${procurementActionLabels[variables.name] ?? 'Đã cập nhật yêu cầu mua'}.${workItemAutoStartedNotice(result)}`); },
    onError: (error) => toast.error('Không cập nhật được yêu cầu mua', error),
  });
  const rows = requests.data ?? [];
  const filteredRows = useMemo(() => rows.filter((row) => {
    const text = `${row.request_code ?? ''} ${row.title ?? ''} ${row.supplier_name ?? ''} ${row.project_name ?? ''}`.toLocaleLowerCase('vi-VN');
    const matchesQuery = !query.trim() || text.includes(query.trim().toLocaleLowerCase('vi-VN'));
    const matchesStatus = statusFilter === 'all' || row.status === statusFilter;
    const matchesSupplier = supplierFilter === 'all' || row.supplier_partner_id === supplierFilter;
    const awaitingStock = numeric(row.accepted_quantity) > numeric(row.stock_received_quantity) + 0.000001;
    const matchesView = view === 'all'
      || (view === 'delivery' && ['ordered', 'partially_received', 'received'].includes(row.status))
      || (view === 'attention' && (!['received'].includes(row.status) || awaitingStock));
    return matchesQuery && matchesStatus && matchesSupplier && matchesView;
  }), [query, rows, statusFilter, supplierFilter, view]);
  const supplierOptions = partnerOptions(partners.data ?? [], ['supplier', 'service_provider', 'other']);
  const waitingApproval = rows.filter((row) => ['waiting_approval', 'submitted'].includes(row.status)).length;
  const waitingOrder = rows.filter((row) => row.status === 'approved').length;
  const overdue = rows.filter(isOverdue).length;
  const waitingStock = rows.filter((row) => numeric(row.accepted_quantity) > numeric(row.stock_received_quantity) + 0.000001).length;

  const fields = (record: ProcurementRow) => [
    { name: 'title', label: 'Tiêu đề', defaultValue: record.title ?? '', required: true },
    { name: 'supplier_partner_id', label: 'Nhà cung cấp', type: 'select' as const, defaultValue: record.supplier_partner_id ?? '', options: supplierOptions, helperText: 'Yêu cầu sinh từ BOQ đã được tách theo nhà cung cấp được chọn.' },
    { name: 'order_reference', label: 'Số đơn đặt hàng / PO', defaultValue: record.order_reference ?? '', helperText: 'Bắt buộc trước khi ghi nhận đã đặt hàng.' },
    { name: 'expected_delivery_date', label: 'Ngày giao dự kiến', type: 'date' as const, defaultValue: record.expected_delivery_date?.slice?.(0, 10) ?? '', helperText: 'Bắt buộc trước khi ghi nhận đã đặt hàng.' },
  ];

  return <>
    <PageHeader
      title="Mua sắm vật tư"
      description="Theo dõi từ yêu cầu mua, đặt hàng, nghiệm thu đầu vào đến phiếu nhập kho."
      actions={<>
        {scoped ? <ProjectBackLink projectId={projectId} /> : null}
        {!scoped ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
        {projectId ? <Link className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" to={`/projects/${projectId}/boq`}><ClipboardList className="h-4 w-4" />Mở BOQ & báo giá</Link> : null}
      </>}
    />
    <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard label="Cần duyệt" value={waitingApproval} icon={<Check className="h-4 w-4" />} />
      <StatCard label="Chờ đặt hàng" value={waitingOrder} icon={<PackageCheck className="h-4 w-4" />} />
      <StatCard label="Trễ giao" value={overdue} icon={<Clock3 className="h-4 w-4" />} />
      <StatCard label="Chờ nhập kho" value={waitingStock} icon={<Truck className="h-4 w-4" />} sub="Đã đạt nghiệm thu nhưng chưa ghi đủ phiếu nhập" />
    </div>
    <div className="mb-4 rounded-lg border border-line bg-white p-3 shadow-panel">
      <div className="flex flex-wrap gap-2 border-b border-line pb-3">
        <ViewButton active={view === 'attention'} onClick={() => setView('attention')}>Cần xử lý</ViewButton>
        <ViewButton active={view === 'delivery'} onClick={() => setView('delivery')}>Đang giao & nghiệm thu</ViewButton>
        <ViewButton active={view === 'all'} onClick={() => setView('all')}>Tất cả</ViewButton>
      </div>
      <div className="grid gap-2 pt-3 md:grid-cols-[minmax(220px,1fr)_180px_180px]">
        <label className="relative"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm" placeholder="Tìm mã, yêu cầu, nhà cung cấp..." value={query} onChange={(event) => setQuery(event.target.value)} /></label>
        <select className="rounded-md border border-line bg-white px-3 py-2 text-sm" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="all">Tất cả trạng thái</option>{Object.entries(procurementStatusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select>
        <select className="rounded-md border border-line bg-white px-3 py-2 text-sm" value={supplierFilter} onChange={(event) => setSupplierFilter(event.target.value)}><option value="all">Tất cả nhà cung cấp</option>{supplierOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
      </div>
    </div>

    <div className="hidden lg:block">
      <DataTable rows={filteredRows} loading={requests.isLoading} error={requests.error} onRetry={() => requests.refetch()} pageSize={10} defaultSort={{ key: 'created_at', direction: 'desc' }} columns={[
        { key: 'request', header: 'Yêu cầu / Nhà cung cấp', sortable: true, filterValue: (row) => [row.request_code, row.title, row.supplier_name], render: (row) => <button className="text-left" onClick={() => setViewId(row.id)}><span className="font-semibold text-blue-700 hover:underline">{row.request_code}</span><div className="font-semibold text-ink">{row.title}</div><div className="text-xs text-slate-500">{row.supplier_name || 'Chưa gán'} · {row.item_count ?? 0} dòng</div></button> },
        { key: 'progress', header: 'Tiến trình', render: (row) => <ProcurementProgress row={row} /> },
        { key: 'total_estimated_amount', header: 'Giá trị từ dòng mua', align: 'right', sortable: true, sortValue: (row) => numeric(row.total_estimated_amount), render: (row) => <div><CurrencyText value={row.total_estimated_amount} /><div className="text-xs text-slate-400">Tự tổng hợp</div></div> },
        { key: 'delivery', header: 'Giao dự kiến', sortable: true, sortValue: (row) => row.expected_delivery_date, render: (row) => <div className={isOverdue(row) ? 'font-semibold text-rose-700' : ''}>{formatDate(row.expected_delivery_date)}{isOverdue(row) ? <div className="mt-1 flex items-center gap-1 text-xs"><AlertTriangle className="h-3.5 w-3.5" />Trễ giao</div> : null}</div> },
        { key: 'next', header: 'Việc tiếp theo', action: true, align: 'right', render: (row) => <NextAction row={row} projectId={row.project_id ?? projectId} locationPath={`${location.pathname}${location.search}`} hasPermission={hasPermission} onView={() => setViewId(row.id)} onEdit={() => setEditRecord(row)} onAction={(name) => action.mutate({ requestId: row.id, name })} /> },
      ]} empty="Không có yêu cầu trong hàng đợi này" />
    </div>
    <div className="space-y-3 lg:hidden">
      {requests.isLoading ? <div className="rounded-lg border border-line bg-white p-4 text-sm text-slate-500">Đang tải dữ liệu...</div> : filteredRows.map((row) => <div key={row.id} className="rounded-lg border border-line bg-white p-4 shadow-panel"><div className="flex items-start justify-between gap-3"><div><button className="font-semibold text-blue-700" onClick={() => setViewId(row.id)}>{row.request_code}</button><p className="font-semibold text-ink">{row.title}</p><p className="text-xs text-slate-500">{row.supplier_name || 'Chưa gán'}</p></div><ProcurementStatus status={row.status} /></div><div className="my-3"><ProcurementProgress row={row} compact /></div><div className="flex items-end justify-between gap-3"><div className="text-sm"><CurrencyText value={row.total_estimated_amount} /><div className="text-xs text-slate-500">Giao {formatDate(row.expected_delivery_date)}</div></div><NextAction row={row} projectId={row.project_id ?? projectId} locationPath={`${location.pathname}${location.search}`} hasPermission={hasPermission} onView={() => setViewId(row.id)} onEdit={() => setEditRecord(row)} onAction={(name) => action.mutate({ requestId: row.id, name })} /></div></div>)}
    </div>

    <Dialog open={Boolean(viewId)} title="Chi tiết yêu cầu mua" description="Nguồn giá, khối lượng đạt nghiệm thu và khối lượng đã ghi nhận nhập kho." onClose={() => setViewId('')} size="xl">
      {detail.isLoading ? <div className="flex items-center gap-2 py-8 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Đang tải chi tiết...</div> : detail.error ? <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">Không tải được chi tiết yêu cầu mua.</div> : detail.data ? <ProcurementDetail record={detail.data} /> : null}
    </Dialog>
    <Dialog open={Boolean(editRecord)} title="Thông tin thương mại & đặt hàng" description={editRecord ? `${editRecord.request_code} · ${editRecord.title}` : undefined} onClose={() => setEditRecord(null)} size="lg">
      {editRecord ? <SimpleForm submitLabel="Lưu thông tin" fields={fields(editRecord)} onSubmit={(values) => update.mutateAsync({ requestId: editRecord.id, values: { ...values, project_id: editRecord.project_id, request_code: editRecord.request_code, status: editRecord.status, total_estimated_amount: editRecord.total_estimated_amount, budget_status: editRecord.budget_status, supplier_partner_id: values.supplier_partner_id || null } })} /> : null}
    </Dialog>
  </>;
}

function NextAction({ row, projectId, locationPath, hasPermission, onView, onEdit, onAction }: { row: ProcurementRow; projectId: string; locationPath: string; hasPermission: (permission: string) => boolean; onView: () => void; onEdit: () => void; onAction: (name: string) => void }) {
  const buttonClass = 'inline-flex items-center justify-center gap-1.5 rounded-md bg-blue-600 px-3 py-2 text-xs font-semibold text-white hover:bg-blue-700';
  if (row.status === 'draft' && hasPermission('procurement.create')) return <button className={buttonClass} onClick={() => onAction('submit')}><Send className="h-3.5 w-3.5" />Gửi duyệt</button>;
  if (['waiting_approval', 'submitted'].includes(row.status) && hasPermission('procurement.approve')) return <button className={buttonClass} onClick={() => onAction('approve')}><Check className="h-3.5 w-3.5" />Duyệt</button>;
  if (row.status === 'approved' && hasPermission('procurement.order')) return <button className={buttonClass} onClick={onEdit}><PackageCheck className="h-3.5 w-3.5" />Bổ sung PO / Đặt hàng</button>;
  if (['ordered', 'partially_received'].includes(row.status) && row.incoming_acceptance_work_item_id) return <Link className={buttonClass} to={workItemDetailPath(row.incoming_acceptance_work_item_id, locationPath, projectId)}><Truck className="h-3.5 w-3.5" />Mở nghiệm thu</Link>;
  if (['ordered', 'partially_received'].includes(row.status) && hasPermission('procurement.receive') && numeric(row.accepted_quantity) > 0) return <button className={buttonClass} onClick={() => onAction('receive')}><Truck className="h-3.5 w-3.5" />Đối soát nghiệm thu</button>;
  if (numeric(row.accepted_quantity) > numeric(row.stock_received_quantity) + 0.000001 && hasPermission('warehouse.receive')) return <Link className={buttonClass} to={`/projects/${projectId}/warehouse?tab=documents&receipt=${row.incoming_acceptance_id ?? ''}`}><Truck className="h-3.5 w-3.5" />Tạo phiếu nhập</Link>;
  return <div className="flex justify-end gap-1"><IconButton title="Xem chi tiết" onClick={onView} icon={<Eye className="h-4 w-4" />} />{!['ordered', 'partially_received', 'received'].includes(row.status) && hasPermission('procurement.create') ? <IconButton title="Sửa thông tin thương mại" onClick={onEdit} icon={<Pencil className="h-4 w-4" />} /> : null}</div>;
}

function ProcurementProgress({ row, compact = false }: { row: ProcurementRow; compact?: boolean }) {
  const requested = numeric(row.quantity_requested);
  const accepted = numeric(row.accepted_quantity);
  const stocked = numeric(row.stock_received_quantity);
  const acceptancePercent = requested > 0 ? Math.min(100, Math.round(accepted / requested * 100)) : 0;
  const stockPercent = accepted > 0 ? Math.min(100, Math.round(stocked / accepted * 100)) : 0;
  return <div className={compact ? 'space-y-1.5' : 'min-w-[260px] space-y-2'}><div className="flex items-center gap-1.5 text-[11px] font-semibold text-slate-600"><Step done={!['draft', 'waiting_approval', 'submitted', 'changes_requested'].includes(row.status)} label="Duyệt" /><Step done={['ordered', 'partially_received', 'received'].includes(row.status)} label="Đặt" /><Step done={row.status === 'received'} label={`NT ${acceptancePercent}%`} /><Step done={stockPercent >= 100 && accepted > 0} label={`Kho ${stockPercent}%`} /></div><div className="grid grid-cols-2 gap-2 text-xs text-slate-500"><span>Đạt NT: <strong className="text-ink">{formatQuantity(accepted)} / {formatQuantity(requested)}</strong></span><span>Đã nhập: <strong className="text-ink">{formatQuantity(stocked)}</strong></span></div></div>;
}

function Step({ done, label }: { done: boolean; label: string }) { return <span className={`rounded-full border px-2 py-1 ${done ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-slate-200 bg-slate-50 text-slate-500'}`}>{done ? '✓ ' : ''}{label}</span>; }
function ProcurementStatus({ status }: { status: string }) { return <span className="inline-flex rounded-full border border-slate-200 bg-slate-50 px-2 py-1 text-xs font-semibold text-slate-700">{procurementStatusLabels[status] ?? status}</span>; }
function ViewButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) { return <button type="button" onClick={onClick} className={`rounded-md border px-3 py-2 text-sm font-semibold ${active ? 'border-blue-600 bg-blue-600 text-white' : 'border-line bg-white text-slate-600 hover:bg-slate-50'}`}>{children}</button>; }
function IconButton({ title, onClick, icon }: { title: string; onClick: () => void; icon: React.ReactNode }) { return <button title={title} aria-label={title} className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={onClick}>{icon}</button>; }

function ProcurementDetail({ record }: { record: ProcurementRow }) {
  const items = record.items ?? [];
  return <div className="space-y-4"><div className="grid gap-3 rounded-md border border-line bg-slate-50 p-3 sm:grid-cols-4"><Metric label="Mã yêu cầu" value={record.request_code} /><Metric label="Nhà cung cấp" value={record.supplier_name || 'Chưa gán'} /><Metric label="PO / Đơn hàng" value={record.order_reference || 'Chưa lập'} /><Metric label="Giao dự kiến" value={formatDate(record.expected_delivery_date)} /></div>{!items.length ? <div className="rounded-md border border-dashed border-line p-4 text-sm text-slate-500">Yêu cầu chưa có dòng hàng; chưa đủ điều kiện gửi duyệt.</div> : <div className="overflow-x-auto rounded-md border border-line"><table className="min-w-[1050px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-3 py-2">Công việc / vật tư</th><th className="px-3 py-2">Nguồn giá</th><th className="px-3 py-2 text-right">Yêu cầu</th><th className="px-3 py-2 text-right">Đạt NT</th><th className="px-3 py-2 text-right">Đã nhập</th><th className="px-3 py-2 text-right">Chờ nhập</th><th className="px-3 py-2">Trạng thái giao</th></tr></thead><tbody className="divide-y divide-line">{items.map((item: ProcurementRow) => <tr key={item.id}><td className="px-3 py-2"><div className="font-semibold">{item.material_name || item.work_item_name}</div><div className="text-xs text-slate-500">{item.supplier_name}</div></td><td className="px-3 py-2">{item.quotation_code ? <span>{item.quotation_code}<br /><span className="text-xs text-slate-500">{item.valid_until ? `hiệu lực đến ${formatDate(item.valid_until)}` : ''}</span></span> : <span className="text-amber-700">Miễn báo giá: {item.price_waiver_reason || 'chưa có căn cứ'}</span>}</td><td className="px-3 py-2 text-right">{formatQuantity(item.quantity_requested)} {item.unit}</td><td className="px-3 py-2 text-right">{formatQuantity(item.accepted_quantity)} {item.unit}</td><td className="px-3 py-2 text-right">{formatQuantity(item.stock_received_quantity)} {item.unit}</td><td className="px-3 py-2 text-right font-semibold">{formatQuantity(item.awaiting_stock_quantity)} {item.unit}</td><td className="px-3 py-2"><ProcurementStatus status={item.delivery_status} /></td></tr>)}</tbody></table></div>}</div>;
}

function Metric({ label, value }: { label: string; value: React.ReactNode }) { return <div><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 font-semibold text-ink">{value}</div></div>; }
function numeric(value: unknown) { return Number(value ?? 0); }
function formatQuantity(value: unknown) { return numeric(value).toLocaleString('vi-VN', { maximumFractionDigits: 3 }); }
function formatDate(value: unknown) { return dateText(String(value ?? '').slice(0, 10), 'Chưa chốt'); }
function isOverdue(row: ProcurementRow) { const date = String(row.expected_delivery_date ?? '').slice(0, 10); return Boolean(date && date < new Date().toISOString().slice(0, 10) && !['received', 'cancelled'].includes(row.status)); }

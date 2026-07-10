import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, ClipboardList, Eye, Loader2, PackageCheck, Pencil, Send, Truck } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import type { Partner } from '../../lib/types/domain';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';
import { partnerOptions } from '../core-data/partnerUtils';

const procurementActionLabels: Record<string, string> = {
  submit: 'Đã gửi yêu cầu mua để duyệt',
  approve: 'Đã duyệt yêu cầu mua',
  order: 'Đã ghi nhận đặt hàng',
  receive: 'Đã đối soát khối lượng nghiệm thu đầu vào',
};

export function ProcurementPage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [viewId, setViewId] = useState('');
  const [editRecord, setEditRecord] = useState<Record<string, any> | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const requests = useQuery({ queryKey: ['procurement'], queryFn: () => get<Record<string, any>[]>('/procurement-requests') });
  const detail = useQuery({ queryKey: ['procurement-detail', viewId], queryFn: () => get<Record<string, any>>(`/procurement-requests/${viewId}`), enabled: Boolean(viewId) });
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const partners = useQuery({ queryKey: ['partners', 'supplier', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const invalidate = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ['procurement'] }),
    queryClient.invalidateQueries({ queryKey: ['procurement-detail'] }),
  ]);
  const update = useMutation({
    mutationFn: ({ requestId, values }: { requestId: string; values: Record<string, any> }) => put(`/procurement-requests/${requestId}`, values),
    onSuccess: async () => {
      await invalidate();
      setEditRecord(null);
      toast.success('Đã cập nhật thông tin thương mại yêu cầu mua');
    },
    onError: (error) => toast.error('Không cập nhật được yêu cầu mua', error),
  });
  const action = useMutation({
    mutationFn: ({ requestId, name }: { requestId: string; name: string }) => post(`/procurement-requests/${requestId}/${name}`, {}),
    onSuccess: async (_data, variables) => {
      await invalidate();
      toast.success(procurementActionLabels[variables.name] ?? 'Đã cập nhật yêu cầu mua');
    },
    onError: (error) => toast.error('Không cập nhật được yêu cầu mua', error),
  });
  const rows = (requests.data ?? []).filter((row) => (scoped || projectId) ? row.project_id === projectId : true);
  const supplierOptions = partnerOptions(partners.data ?? [], ['supplier', 'service_provider', 'other']);
  const fields = (record: Record<string, any>) => [
    { name: 'title', label: 'Tiêu đề', defaultValue: record.title ?? '', required: true },
    { name: 'total_estimated_amount', label: 'Giá trị dự kiến', type: 'number' as const, defaultValue: record.total_estimated_amount ?? 0, min: 0 },
    { name: 'budget_status', label: 'Ngân sách', type: 'select' as const, defaultValue: record.budget_status ?? 'within_budget', required: true, options: enumOptions('budget_status', ['within_budget', 'budget_exceeded']) },
    { name: 'supplier_partner_id', label: 'Nhà cung cấp', type: 'select' as const, defaultValue: record.supplier_partner_id ?? '', options: supplierOptions, helperText: 'Yêu cầu sinh từ BOQ đã được tách theo nhà cung cấp được chọn.' },
    { name: 'order_reference', label: 'Số đơn đặt hàng / PO', defaultValue: record.order_reference ?? '', helperText: 'Bắt buộc trước khi ghi nhận đã đặt hàng.' },
    { name: 'expected_delivery_date', label: 'Ngày giao dự kiến', type: 'date' as const, defaultValue: record.expected_delivery_date?.slice?.(0, 10) ?? '', helperText: 'Bắt buộc trước khi ghi nhận đã đặt hàng.' },
  ];

  return <>
    <PageHeader
      title="Mua sắm vật tư"
      description="Theo dõi yêu cầu mua đã sinh từ BOQ, duyệt mua, đặt hàng và đối soát nhận hàng qua nghiệm thu đầu vào."
      actions={<>
        {scoped ? <ProjectBackLink projectId={projectId} /> : null}
        {!scoped ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
        {projectId ? <Link className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" to={`/projects/${projectId}/boq`}><ClipboardList className="h-4 w-4" /> Mở BOQ & báo giá</Link> : null}
      </>}
    />
    <div className="mb-4 grid gap-3 sm:grid-cols-3">
      <Metric label="Yêu cầu đang theo dõi" value={rows.length} />
      <Metric label="Chờ duyệt / đặt hàng" value={rows.filter((row) => ['waiting_approval', 'approved'].includes(row.status)).length} />
      <Metric label="Đang giao / nhận" value={rows.filter((row) => ['ordered', 'partially_received'].includes(row.status)).length} />
    </div>
    <DataTable
      rows={rows}
      loading={requests.isLoading}
      error={requests.error}
      onRetry={() => requests.refetch()}
      searchPlaceholder="Tìm mã, nhà cung cấp, công trình"
      pageSize={10}
      defaultSort={{ key: 'created_at', direction: 'desc' }}
      columns={[
        { key: 'request_code', header: 'Mã', sortable: true, render: (row) => <button className="font-semibold text-blue-700 hover:underline" onClick={() => setViewId(row.id)}>{row.request_code}</button> },
        { key: 'title', header: 'Yêu cầu', sortable: true, render: (row) => <div><span className="font-semibold text-ink">{row.title}</span><div className="text-xs text-slate-500">{row.item_count ?? 0} dòng</div></div> },
        { key: 'supplier_name', header: 'Nhà cung cấp', sortable: true, render: (row) => row.supplier_name || <span className="text-amber-700">Chưa gán</span> },
        { key: 'project_name', header: 'Công trình', sortable: true },
        { key: 'total_estimated_amount', header: 'Giá trị', align: 'right', sortable: true, sortValue: (row) => Number(row.total_estimated_amount ?? 0), render: (row) => <CurrencyText value={row.total_estimated_amount} /> },
        { key: 'delivery', header: 'Đã nghiệm thu', align: 'right', render: (row) => <span>{Number(row.accepted_quantity ?? 0).toLocaleString('vi-VN')} / {Number(row.quantity_requested ?? 0).toLocaleString('vi-VN')}</span> },
        { key: 'budget_status', header: 'Ngân sách', sortable: true, render: (row) => <EnumChip group="budget_status" value={row.budget_status} /> },
        { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
        { key: 'actions', header: 'Thao tác', align: 'right', render: (row) => <div className="flex justify-end gap-1">
          <ActionButton title="Xem chi tiết" onClick={() => setViewId(row.id)} icon={<Eye className="h-4 w-4" />} />
          {!['ordered', 'partially_received', 'received'].includes(row.status) ? <ActionButton title="Sửa thông tin thương mại" onClick={() => setEditRecord(row)} icon={<Pencil className="h-4 w-4" />} /> : null}
          {row.status === 'draft' ? <ActionButton title="Gửi duyệt" onClick={() => action.mutate({ requestId: row.id, name: 'submit' })} icon={<Send className="h-4 w-4" />} /> : null}
          {['waiting_approval', 'submitted'].includes(row.status) ? <ActionButton title="Duyệt" onClick={() => action.mutate({ requestId: row.id, name: 'approve' })} icon={<Check className="h-4 w-4" />} /> : null}
          {row.status === 'approved' ? <ActionButton title="Đặt hàng" onClick={() => action.mutate({ requestId: row.id, name: 'order' })} icon={<PackageCheck className="h-4 w-4" />} /> : null}
          {['ordered', 'partially_received'].includes(row.status) ? <ActionButton title="Đối soát nhận hàng" onClick={() => action.mutate({ requestId: row.id, name: 'receive' })} icon={<Truck className="h-4 w-4" />} /> : null}
        </div> },
      ]}
    />

    <Dialog open={Boolean(viewId)} title="Chi tiết yêu cầu mua" description="Nguồn giá, tiến độ nghiệm thu đầu vào và khối lượng còn lại theo từng dòng." onClose={() => setViewId('')} size="xl">
      {detail.isLoading ? <div className="flex items-center gap-2 py-8 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> Đang tải chi tiết...</div> : detail.error ? <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">Không tải được chi tiết yêu cầu mua.</div> : detail.data ? <ProcurementDetail record={detail.data} /> : null}
    </Dialog>

    <Dialog open={Boolean(editRecord)} title="Thông tin thương mại & đặt hàng" description={editRecord ? `${editRecord.request_code} · ${editRecord.title}` : undefined} onClose={() => setEditRecord(null)} size="lg">
      {editRecord ? <SimpleForm submitLabel="Lưu thông tin" fields={fields(editRecord)} onSubmit={(values) => update.mutateAsync({ requestId: editRecord.id, values: { ...values, project_id: editRecord.project_id, request_code: editRecord.request_code, status: editRecord.status, supplier_partner_id: values.supplier_partner_id || null } })} /> : null}
    </Dialog>
  </>;
}

function ProcurementDetail({ record }: { record: Record<string, any> }) {
  const items = record.items ?? [];
  return <div className="space-y-4">
    <div className="grid gap-3 rounded-md border border-line bg-slate-50 p-3 sm:grid-cols-4">
      <Metric label="Mã yêu cầu" value={record.request_code} />
      <Metric label="Nhà cung cấp" value={record.supplier_name || 'Chưa gán'} />
      <Metric label="PO / Đơn hàng" value={record.order_reference || 'Chưa lập'} />
      <Metric label="Giao dự kiến" value={record.expected_delivery_date?.slice?.(0, 10) || 'Chưa chốt'} />
    </div>
    {!items.length ? <div className="rounded-md border border-dashed border-line p-4 text-sm text-slate-500">Yêu cầu chưa có dòng hàng; chưa đủ điều kiện gửi duyệt.</div> : <div className="overflow-x-auto rounded-md border border-line"><table className="min-w-[900px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-3 py-2">Công việc / vật tư</th><th className="px-3 py-2">Nguồn giá</th><th className="px-3 py-2 text-right">KL yêu cầu</th><th className="px-3 py-2 text-right">Đã nghiệm thu</th><th className="px-3 py-2 text-right">Còn lại</th><th className="px-3 py-2">Giao hàng</th></tr></thead><tbody className="divide-y divide-line">{items.map((item: Record<string, any>) => <tr key={item.id}><td className="px-3 py-2"><div className="font-semibold">{item.material_name || item.work_item_name}</div><div className="text-xs text-slate-500">{item.supplier_name}</div></td><td className="px-3 py-2">{item.quotation_code ? <span>{item.quotation_code}<br /><span className="text-xs text-slate-500">{item.valid_until ? `hiệu lực đến ${item.valid_until.slice(0, 10)}` : ''}</span></span> : <span className="text-amber-700">Miễn báo giá: {item.price_waiver_reason || 'chưa có căn cứ'}</span>}</td><td className="px-3 py-2 text-right">{item.quantity_requested} {item.unit}</td><td className="px-3 py-2 text-right">{item.accepted_quantity} {item.unit}</td><td className="px-3 py-2 text-right font-semibold">{item.remaining_quantity} {item.unit}</td><td className="px-3 py-2"><StatusBadge status={item.delivery_status} /></td></tr>)}</tbody></table></div>}
  </div>;
}

function Metric({ label, value }: { label: string; value: React.ReactNode }) { return <div className="rounded-md border border-line bg-white px-3 py-2"><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 font-semibold text-ink">{value}</div></div>; }
function ActionButton({ title, onClick, icon }: { title: string; onClick: () => void; icon: React.ReactNode }) { return <button title={title} aria-label={title} className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-50" onClick={onClick}>{icon}</button>; }

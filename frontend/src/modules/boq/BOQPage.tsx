import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Send, ThumbsUp, ShoppingCart } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { CurrencyText } from '../../components/CurrencyText';
import { SimpleForm } from '../_shared/SimpleForm';

export function BOQPage() {
  const { id = '' } = useParams();
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const boq = useQuery({ queryKey: ['boq', id], queryFn: () => get<Record<string, any>[]>(`/projects/${id}/boq`), enabled: Boolean(id) });
  const materials = useQuery({ queryKey: ['materials'], queryFn: () => get<Record<string, any>[]>('/materials') });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['boq', id] });
  const create = useMutation({ mutationFn: (values: Record<string, any>) => post(`/projects/${id}/boq`, values), onSuccess: () => { invalidate(); setOpen(false); } });
  const update = useMutation({ mutationFn: ({ itemId, values }: { itemId: string; values: Record<string, any> }) => put(`/boq-items/${itemId}`, values), onSuccess: invalidate });
  const submit = useMutation({ mutationFn: () => post(`/projects/${id}/boq/submit`, {}), onSuccess: invalidate });
  const approve = useMutation({ mutationFn: () => post(`/projects/${id}/boq/approve`, {}), onSuccess: invalidate });
  const generate = useMutation({ mutationFn: () => post(`/projects/${id}/boq/generate-procurement`, {}) });
  const total = (boq.data ?? []).reduce((sum, item) => sum + Number(item.total_amount ?? 0), 0);

  if (boq.isLoading) return <LoadingSpinner />;

  return (
    <>
      <PageHeader
        title="BOQ / Dự toán"
        description="Bóc tách khối lượng, kiểm soát đơn giá, submit/approve và sinh nhu cầu mua vật tư từ BOQ."
        actions={
          <>
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => submit.mutate()}><Send className="h-4 w-4" />Submit</button>
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => approve.mutate()}><ThumbsUp className="h-4 w-4" />Approve</button>
            <button className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700" onClick={() => generate.mutate()}><ShoppingCart className="h-4 w-4" />Sinh mua sắm</button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Dòng BOQ</button>
          </>
        }
      />
      <div className="mb-4 rounded-lg border border-line bg-white p-4 shadow-panel">
        <p className="text-sm text-slate-500">Tổng dự toán</p>
        <p className="mt-1 text-2xl font-semibold text-ink"><CurrencyText value={total} /></p>
      </div>
      <DataTable
        rows={boq.data ?? []}
        columns={[
          { key: 'category', header: 'Hạng mục' },
          { key: 'work_item_name', header: 'Công việc', render: (row) => <span className="font-semibold text-ink">{row.work_item_name}</span> },
          { key: 'quantity', header: 'KL' },
          { key: 'unit', header: 'ĐVT' },
          { key: 'unit_price', header: 'Đơn giá', render: (row) => <CurrencyText value={row.unit_price} /> },
          { key: 'total_amount', header: 'Thành tiền', render: (row) => <CurrencyText value={row.total_amount} /> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'action', header: 'Action', render: (row) => <button className="text-sm font-semibold text-blue-700" onClick={() => update.mutate({ itemId: row.id, values: { status: 'revised' } })}>Revised</button> },
        ]}
      />
      <FormDrawer open={open} title="Thêm dòng BOQ" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Thêm BOQ"
          fields={[
            { name: 'category', label: 'Hạng mục', defaultValue: 'Điện' },
            { name: 'work_item_name', label: 'Tên công việc', defaultValue: 'Hạng mục mới' },
            { name: 'description', label: 'Mô tả', type: 'textarea' },
            { name: 'unit', label: 'Đơn vị', defaultValue: 'm2' },
            { name: 'quantity', label: 'Khối lượng', type: 'number', defaultValue: 1 },
            { name: 'unit_price', label: 'Đơn giá', type: 'number', defaultValue: 1000000 },
            { name: 'material_id', label: 'Vật tư liên kết', type: 'select', options: (materials.data ?? []).map((m) => ({ label: m.name, value: m.id })) },
            { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', options: ['draft', 'submitted', 'approved'].map((x) => ({ label: x, value: x })) },
          ]}
          onSubmit={(values) => create.mutateAsync(values)}
        />
      </FormDrawer>
    </>
  );
}

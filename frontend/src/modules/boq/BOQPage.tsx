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
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { SimpleForm } from '../_shared/SimpleForm';
import { enumOptions } from '../../lib/utils/status';

export function BOQPage() {
  const { id = '' } = useParams();
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const boq = useQuery({ queryKey: ['boq', id], queryFn: () => get<Record<string, any>[]>(`/projects/${id}/boq`), enabled: Boolean(id) });
  const materials = useQuery({ queryKey: ['materials'], queryFn: () => get<Record<string, any>[]>('/materials') });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['boq', id] });
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${id}/boq`, values),
    onSuccess: () => {
      invalidate();
      setOpen(false);
      toast.success('Đã thêm dòng BOQ');
    },
    onError: (error) => toast.error('Không thêm được dòng BOQ', error),
  });
  const update = useMutation({
    mutationFn: ({ itemId, values }: { itemId: string; values: Record<string, any> }) => put(`/boq-items/${itemId}`, values),
    onSuccess: () => {
      invalidate();
      toast.success('Đã cập nhật dòng BOQ');
    },
    onError: (error) => toast.error('Không cập nhật được dòng BOQ', error),
  });
  const submit = useMutation({
    mutationFn: () => post(`/projects/${id}/boq/submit`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã submit BOQ');
    },
    onError: (error) => toast.error('Không submit được BOQ', error),
  });
  const approve = useMutation({
    mutationFn: () => post(`/projects/${id}/boq/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt BOQ');
    },
    onError: (error) => toast.error('Không duyệt được BOQ', error),
  });
  const generate = useMutation({
    mutationFn: () => post(`/projects/${id}/boq/generate-procurement`, {}),
    onSuccess: () => toast.success('Đã sinh yêu cầu mua sắm'),
    onError: (error) => toast.error('Không sinh được yêu cầu mua sắm', error),
  });
  const total = (boq.data ?? []).reduce((sum, item) => sum + Number(item.total_amount ?? 0), 0);
  const boqFields = (record?: Record<string, any>) => [
    { name: 'category', label: 'Hạng mục', defaultValue: record?.category ?? 'Điện', required: true },
    { name: 'work_item_name', label: 'Tên công việc', defaultValue: record?.work_item_name ?? 'Hạng mục mới', required: true },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '' },
    { name: 'unit', label: 'Đơn vị', defaultValue: record?.unit ?? 'm2', required: true },
    { name: 'quantity', label: 'Khối lượng', type: 'number' as const, defaultValue: record?.quantity ?? 1, required: true, min: 0 },
    { name: 'unit_price', label: 'Đơn giá', type: 'number' as const, defaultValue: record?.unit_price ?? 1000000, required: true, min: 0 },
    { name: 'material_id', label: 'Vật tư liên kết', type: 'select' as const, defaultValue: record?.material_id ?? '', options: (materials.data ?? []).map((m) => ({ label: m.name, value: m.id })) },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'draft', required: true, options: enumOptions('status', ['draft', 'submitted', 'approved', 'revised']) },
  ];

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
          {
            key: 'action',
            header: 'Action',
            render: (row) => (
              <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={() => setDialog({ mode: 'edit', record: row })}>
                <button className="text-sm font-semibold text-blue-700" onClick={() => update.mutate({ itemId: row.id, values: { status: 'revised' } })}>Revised</button>
              </RowActions>
            ),
          },
        ]}
      />
      <FormDrawer open={open} title="Thêm dòng BOQ" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Thêm BOQ"
          fields={boqFields()}
          onSubmit={(values) => create.mutateAsync(values)}
        />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết dòng BOQ"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'category', label: 'Hạng mục' },
          { key: 'work_item_name', label: 'Công việc' },
          { key: 'description', label: 'Mô tả' },
          { key: 'quantity', label: 'Khối lượng' },
          { key: 'unit', label: 'ĐVT' },
          { key: 'unit_price', label: 'Đơn giá', render: (row) => <CurrencyText value={row.unit_price} /> },
          { key: 'total_amount', label: 'Thành tiền', render: (row) => <CurrencyText value={row.total_amount} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa dòng BOQ" onClose={() => setDialog(null)}>
        {dialog?.record ? (
          <SimpleForm
            submitLabel="Lưu thay đổi"
            fields={boqFields(dialog.record)}
            onSubmit={(values) => update.mutateAsync({ itemId: dialog.record.id, values: { ...values, material_id: values.material_id || null } })}
          />
        ) : null}
      </FormDrawer>
    </>
  );
}

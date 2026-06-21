import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, PackageCheck, Plus, Truck } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';

const procurementActionLabels: Record<string, string> = {
  approve: 'Đã duyệt yêu cầu mua',
  order: 'Đã đặt hàng',
  receive: 'Đã nhận hàng',
};

export function ProcurementPage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const requests = useQuery({ queryKey: ['procurement'], queryFn: () => get<Record<string, any>[]>('/procurement-requests') });
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['procurement'] });
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post('/procurement-requests', values),
    onSuccess: () => {
      invalidate();
      setOpen(false);
      toast.success('Đã tạo yêu cầu mua');
    },
    onError: (error) => toast.error('Không tạo được yêu cầu mua', error),
  });
  const update = useMutation({
    mutationFn: ({ requestId, values }: { requestId: string; values: Record<string, any> }) => put(`/procurement-requests/${requestId}`, values),
    onSuccess: () => {
      invalidate();
      setDialog(null);
      toast.success('Đã cập nhật yêu cầu mua');
    },
    onError: (error) => toast.error('Không cập nhật được yêu cầu mua', error),
  });
  const action = useMutation({
    mutationFn: ({ requestId, name }: { requestId: string; name: string }) => post(`/procurement-requests/${requestId}/${name}`, {}),
    onSuccess: (_data, variables) => {
      invalidate();
      toast.success(procurementActionLabels[variables.name] ?? 'Đã cập nhật yêu cầu mua');
    },
    onError: (error) => toast.error('Không cập nhật được yêu cầu mua', error),
  });
  const rows = (requests.data ?? []).filter((row) => (scoped || projectId) ? row.project_id === projectId : true);
  const procurementFields = (record?: Record<string, any>) => [
    { name: 'project_id', label: 'Công trình', type: 'select' as const, defaultValue: record?.project_id ?? projectId, required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
    { name: 'request_code', label: 'Mã yêu cầu', defaultValue: record?.request_code ?? `PR-${Date.now().toString().slice(-5)}`, required: true },
    { name: 'title', label: 'Tiêu đề', defaultValue: record?.title ?? 'Yêu cầu mua vật tư mới', required: true },
    { name: 'total_estimated_amount', label: 'Giá trị dự kiến', type: 'number' as const, defaultValue: record?.total_estimated_amount ?? 50000000, min: 0 },
    { name: 'budget_status', label: 'Budget status', type: 'select' as const, defaultValue: record?.budget_status ?? 'within_budget', required: true, options: enumOptions('budget_status', ['within_budget', 'budget_exceeded']) },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'draft', required: true, options: enumOptions('status', ['draft', 'submitted', 'waiting_approval', 'approved', 'ordered', 'received']) },
  ];

  return (
    <>
      <PageHeader
        title="Mua sắm vật tư"
        description="Quản lý yêu cầu mua, duyệt mua, đặt hàng, giao hàng và cảnh báo vượt ngân sách/BOQ."
        actions={
          <>
            {scoped ? <ProjectBackLink projectId={projectId} /> : null}
            {!scoped ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Yêu cầu mua</button>
          </>
        }
      />
      <DataTable
        rows={rows}
        loading={requests.isLoading}
        error={requests.error}
        onRetry={() => requests.refetch()}
        searchPlaceholder="Tìm mã, yêu cầu, công trình"
        pageSize={10}
        defaultSort={{ key: 'request_code', direction: 'asc' }}
        columns={[
          { key: 'request_code', header: 'Mã', sortable: true },
          { key: 'title', header: 'Yêu cầu', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
          { key: 'project_name', header: 'Công trình', sortable: true },
          { key: 'total_estimated_amount', header: 'Giá trị', align: 'right', sortable: true, sortValue: (row) => Number(row.total_estimated_amount ?? 0), render: (row) => <CurrencyText value={row.total_estimated_amount} /> },
          { key: 'budget_status', header: 'Budget', sortable: true, render: (row) => <EnumChip group="budget_status" value={row.budget_status} /> },
          { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
          {
            key: 'actions',
            header: 'Action',
            align: 'right',
            render: (row) => (
              <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={() => setDialog({ mode: 'edit', record: row })}>
                <button title="Duyệt" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ requestId: row.id, name: 'approve' })}><Check className="h-4 w-4" /></button>
                <button title="Đặt hàng" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ requestId: row.id, name: 'order' })}><PackageCheck className="h-4 w-4" /></button>
                <button title="Nhận hàng" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ requestId: row.id, name: 'receive' })}><Truck className="h-4 w-4" /></button>
              </RowActions>
            ),
          },
        ]}
      />
      <FormDrawer open={open} title="Tạo yêu cầu mua" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Tạo yêu cầu"
          fields={procurementFields()}
          onSubmit={(values) => create.mutateAsync(values)}
        />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết yêu cầu mua"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'request_code', label: 'Mã' },
          { key: 'title', label: 'Yêu cầu' },
          { key: 'project_name', label: 'Công trình' },
          { key: 'total_estimated_amount', label: 'Giá trị', render: (row) => <CurrencyText value={row.total_estimated_amount} /> },
          { key: 'budget_status', label: 'Budget', render: (row) => <EnumChip group="budget_status" value={row.budget_status} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'created_at', label: 'Ngày tạo' },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa yêu cầu mua" onClose={() => setDialog(null)}>
        {dialog?.record ? <SimpleForm submitLabel="Lưu thay đổi" fields={procurementFields(dialog.record)} onSubmit={(values) => update.mutateAsync({ requestId: dialog.record.id, values })} /> : null}
      </FormDrawer>
    </>
  );
}

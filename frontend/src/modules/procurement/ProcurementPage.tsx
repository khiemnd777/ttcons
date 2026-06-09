import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, PackageCheck, Plus, Truck } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { FormDrawer } from '../../components/FormDrawer';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function ProcurementPage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const requests = useQuery({ queryKey: ['procurement'], queryFn: () => get<Record<string, any>[]>('/procurement-requests') });
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['procurement'] });
  const create = useMutation({ mutationFn: (values: Record<string, any>) => post('/procurement-requests', values), onSuccess: () => { invalidate(); setOpen(false); } });
  const action = useMutation({ mutationFn: ({ requestId, name }: { requestId: string; name: string }) => post(`/procurement-requests/${requestId}/${name}`, {}), onSuccess: invalidate });
  const rows = (requests.data ?? []).filter((row) => (scoped || projectId) ? row.project_id === projectId : true);

  return (
    <>
      <PageHeader
        title="Mua sắm vật tư"
        description="Quản lý yêu cầu mua, duyệt mua, đặt hàng, giao hàng và cảnh báo vượt ngân sách/BOQ."
        actions={
          <>
            {!scoped ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Yêu cầu mua</button>
          </>
        }
      />
      <DataTable
        rows={rows}
        columns={[
          { key: 'request_code', header: 'Mã' },
          { key: 'title', header: 'Yêu cầu', render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
          { key: 'project_name', header: 'Công trình' },
          { key: 'total_estimated_amount', header: 'Giá trị', render: (row) => <CurrencyText value={row.total_estimated_amount} /> },
          { key: 'budget_status', header: 'Budget' },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          {
            key: 'actions',
            header: 'Action',
            render: (row) => (
              <div className="flex gap-2">
                <button title="Duyệt" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ requestId: row.id, name: 'approve' })}><Check className="h-4 w-4" /></button>
                <button title="Đặt hàng" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ requestId: row.id, name: 'order' })}><PackageCheck className="h-4 w-4" /></button>
                <button title="Nhận hàng" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ requestId: row.id, name: 'receive' })}><Truck className="h-4 w-4" /></button>
              </div>
            ),
          },
        ]}
      />
      <FormDrawer open={open} title="Tạo yêu cầu mua" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Tạo yêu cầu"
          fields={[
            { name: 'project_id', label: 'Công trình', type: 'select', defaultValue: projectId, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
            { name: 'request_code', label: 'Mã yêu cầu', defaultValue: `PR-${Date.now().toString().slice(-5)}` },
            { name: 'title', label: 'Tiêu đề', defaultValue: 'Yêu cầu mua vật tư mới' },
            { name: 'total_estimated_amount', label: 'Giá trị dự kiến', type: 'number', defaultValue: 50000000 },
            { name: 'budget_status', label: 'Budget status', type: 'select', defaultValue: 'within_budget', options: ['within_budget', 'budget_exceeded'].map((x) => ({ label: x, value: x })) },
            { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', options: ['draft', 'submitted', 'waiting_approval'].map((x) => ({ label: x, value: x })) },
          ]}
          onSubmit={(values) => create.mutateAsync(values)}
        />
      </FormDrawer>
    </>
  );
}

import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function AccountingPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState<'budget' | 'payment' | null>(null);
  const queryClient = useQueryClient();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const budgets = useQuery({ queryKey: ['budget', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/budget`), enabled: Boolean(projectId) });
  const payments = useQuery({ queryKey: ['payments'], queryFn: () => get<Record<string, any>[]>('/payments') });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['budget', projectId] });
    queryClient.invalidateQueries({ queryKey: ['payments'] });
  };
  const createBudget = useMutation({ mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/budget`, values), onSuccess: () => { invalidate(); setOpen(null); } });
  const createPayment = useMutation({ mutationFn: (values: Record<string, any>) => post('/payments', values), onSuccess: () => { invalidate(); setOpen(null); } });
  const approveBudget = useMutation({ mutationFn: (budgetId: string) => post(`/budgets/${budgetId}/approve`, {}), onSuccess: invalidate });
  const approvePayment = useMutation({ mutationFn: (paymentId: string) => post(`/payments/${paymentId}/approve`, {}), onSuccess: invalidate });
  const paymentRows = (payments.data ?? []).filter((row) => !projectId || row.project_id === projectId);

  return (
    <>
      <PageHeader
        title="Kế toán"
        description="Ngân sách, thanh toán, tạm ứng/hoàn ứng, dòng tiền và kiểm soát tài chính công trình."
        actions={
          <>
            {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setOpen('payment')}><Plus className="h-4 w-4" />Payment</button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('budget')}><Plus className="h-4 w-4" />Budget</button>
          </>
        }
      />
      <div className="grid gap-4 xl:grid-cols-2">
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Ngân sách công trình</h2>
          <DataTable rows={budgets.data ?? []} columns={[
            { key: 'budget_code', header: 'Mã' },
            { key: 'name', header: 'Ngân sách' },
            { key: 'approved_amount', header: 'Đã duyệt', render: (row) => <CurrencyText value={row.approved_amount} /> },
            { key: 'used_amount', header: 'Đã dùng', render: (row) => <CurrencyText value={row.used_amount} /> },
            { key: 'remaining_amount', header: 'Còn lại', render: (row) => <CurrencyText value={row.remaining_amount} /> },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <button className="text-sm font-semibold text-blue-700" onClick={() => approveBudget.mutate(row.id)}>Approve</button> },
          ]} />
        </section>
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Payments / Receipts</h2>
          <DataTable rows={paymentRows} columns={[
            { key: 'payment_code', header: 'Mã' },
            { key: 'payment_type', header: 'Loại' },
            { key: 'project_name', header: 'Công trình' },
            { key: 'amount', header: 'Số tiền', render: (row) => <CurrencyText value={row.amount} /> },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <button className="text-sm font-semibold text-blue-700" onClick={() => approvePayment.mutate(row.id)}><Check className="inline h-4 w-4" /> Approve</button> },
          ]} />
        </section>
      </div>
      <FormDrawer open={open === 'budget'} title="Tạo ngân sách" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'budget_code', label: 'Mã ngân sách', defaultValue: `BUD-${Date.now().toString().slice(-5)}` },
          { name: 'name', label: 'Tên ngân sách', defaultValue: 'Ngân sách công trình' },
          { name: 'planned_amount', label: 'Kế hoạch', type: 'number', defaultValue: 100000000 },
          { name: 'approved_amount', label: 'Đã duyệt', type: 'number', defaultValue: 0 },
          { name: 'used_amount', label: 'Đã dùng', type: 'number', defaultValue: 0 },
          { name: 'remaining_amount', label: 'Còn lại', type: 'number', defaultValue: 100000000 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', options: ['draft', 'submitted', 'approved'].map((x) => ({ label: x, value: x })) },
        ]} onSubmit={(values) => createBudget.mutateAsync(values)} />
      </FormDrawer>
      <FormDrawer open={open === 'payment'} title="Tạo payment / receipt" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'project_id', label: 'Công trình', type: 'select', defaultValue: projectId, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
          { name: 'payment_code', label: 'Mã', defaultValue: `PMT-${Date.now().toString().slice(-5)}` },
          { name: 'payment_type', label: 'Loại', type: 'select', defaultValue: 'supplier_payment', options: ['supplier_payment', 'subcontractor_payment', 'client_receipt', 'advance', 'advance_settlement', 'other'].map((x) => ({ label: x, value: x })) },
          { name: 'amount', label: 'Số tiền', type: 'number', defaultValue: 10000000 },
          { name: 'payment_method', label: 'Phương thức', defaultValue: 'bank_transfer' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', options: ['draft', 'waiting_approval', 'approved', 'paid', 'received'].map((x) => ({ label: x, value: x })) },
        ]} onSubmit={(values) => createPayment.mutateAsync(values)} />
      </FormDrawer>
    </>
  );
}

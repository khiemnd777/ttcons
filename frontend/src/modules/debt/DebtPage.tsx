import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { SimpleForm } from '../_shared/SimpleForm';
import { enumOptions } from '../../lib/utils/status';

const debtActionLabels: Record<string, string> = {
  pay: 'Đã ghi nhận thanh toán phải trả',
  receive: 'Đã ghi nhận thu tiền phải thu',
};

export function DebtPage({ kind }: { kind: 'payables' | 'receivables' | 'both' }) {
  const { id } = useParams();
  const [open, setOpen] = useState<'payable' | 'receivable' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'payable' | 'receivable'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const payables = useQuery({ queryKey: ['payables'], queryFn: () => get<Record<string, any>[]>('/payables'), enabled: kind !== 'receivables' });
  const receivables = useQuery({ queryKey: ['receivables'], queryFn: () => get<Record<string, any>[]>('/receivables'), enabled: kind !== 'payables' });
  const createPayable = useMutation({
    mutationFn: (values: Record<string, any>) => post('/payables', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['payables'] });
      setOpen(null);
      toast.success('Đã ghi nhận phải trả');
    },
    onError: (error) => toast.error('Không ghi nhận được phải trả', error),
  });
  const createReceivable = useMutation({
    mutationFn: (values: Record<string, any>) => post('/receivables', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['receivables'] });
      setOpen(null);
      toast.success('Đã ghi nhận phải thu');
    },
    onError: (error) => toast.error('Không ghi nhận được phải thu', error),
  });
  const action = useMutation({
    mutationFn: ({ path, actionName }: { path: string; actionName: string }) => post(`/${path}/${actionName}`, {}),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['payables'] });
      queryClient.invalidateQueries({ queryKey: ['receivables'] });
      toast.success(debtActionLabels[variables.actionName] ?? 'Đã cập nhật công nợ');
    },
    onError: (error) => toast.error('Không cập nhật được công nợ', error),
  });

  return (
    <>
      <PageHeader
        title={kind === 'payables' ? 'Công nợ đầu vào' : kind === 'receivables' ? 'Công nợ đầu ra' : 'Công nợ'}
        description="Theo dõi phải trả, phải thu, quá hạn, tạm ứng, giữ lại, khấu trừ và các khoản thanh toán/thu tiền."
        actions={
          <>
            {id ? <ProjectBackLink projectId={id} /> : null}
            {kind !== 'receivables' ? <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setOpen('payable')}><Plus className="h-4 w-4" />Phải trả</button> : null}
            {kind !== 'payables' ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('receivable')}><Plus className="h-4 w-4" />Phải thu</button> : null}
          </>
        }
      />
      <div className="grid gap-4 xl:grid-cols-2">
        {kind !== 'receivables' ? (
          <section>
            <h2 className="mb-3 text-base font-semibold text-ink">Payables</h2>
            <DataTable rows={payables.data ?? []} columns={[
              { key: 'payable_code', header: 'Mã' },
              { key: 'vendor_name', header: 'Vendor', render: (row) => <span className="font-semibold text-ink">{row.vendor_name}</span> },
              { key: 'project_name', header: 'Công trình' },
              { key: 'gross_amount', header: 'Gross', render: (row) => <CurrencyText value={row.gross_amount} /> },
              { key: 'remaining_amount', header: 'Còn phải trả', render: (row) => <CurrencyText value={row.remaining_amount} /> },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'payable', record: row })}><button className="text-sm font-semibold text-blue-700" onClick={() => action.mutate({ path: `payables/${row.id}`, actionName: 'pay' })}>Pay</button></RowActions> },
            ]} />
          </section>
        ) : null}
        {kind !== 'payables' ? (
          <section>
            <h2 className="mb-3 text-base font-semibold text-ink">Receivables</h2>
            <DataTable rows={receivables.data ?? []} columns={[
              { key: 'receivable_code', header: 'Mã' },
              { key: 'client_name', header: 'Khách hàng', render: (row) => <span className="font-semibold text-ink">{row.client_name}</span> },
              { key: 'project_name', header: 'Công trình' },
              { key: 'milestone_name', header: 'Milestone' },
              { key: 'remaining_amount', header: 'Còn phải thu', render: (row) => <CurrencyText value={row.remaining_amount} /> },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'receivable', record: row })}><button className="text-sm font-semibold text-blue-700" onClick={() => action.mutate({ path: `receivables/${row.id}`, actionName: 'receive' })}>Receive</button></RowActions> },
            ]} />
          </section>
        ) : null}
      </div>
      <FormDrawer open={open === 'payable'} title="Ghi nhận phải trả" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'project_id', label: 'Công trình', type: 'select', required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
          { name: 'payable_code', label: 'Mã phải trả', defaultValue: `PAY-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'vendor_type', label: 'Loại vendor', type: 'select', defaultValue: 'supplier', required: true, options: enumOptions('vendor_type', ['supplier', 'subcontractor', 'labor_team', 'transport', 'equipment_rental', 'service_provider', 'other']) },
          { name: 'vendor_name', label: 'Vendor', defaultValue: 'Nhà cung cấp mới', required: true },
          { name: 'gross_amount', label: 'Gross', type: 'number', defaultValue: 10000000, required: true, min: 0 },
          { name: 'advance_amount', label: 'Tạm ứng', type: 'number', defaultValue: 0, min: 0 },
          { name: 'retention_amount', label: 'Giữ lại', type: 'number', defaultValue: 0, min: 0 },
          { name: 'deduction_amount', label: 'Khấu trừ', type: 'number', defaultValue: 0, min: 0 },
          { name: 'paid_amount', label: 'Đã trả', type: 'number', defaultValue: 0, min: 0 },
          { name: 'remaining_amount', label: 'Còn lại', type: 'number', defaultValue: 10000000, min: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'recorded', required: true, options: enumOptions('status', ['draft', 'recorded', 'approved', 'partially_paid', 'paid']) },
        ]} onSubmit={(values) => createPayable.mutateAsync(values)} />
      </FormDrawer>
      <FormDrawer open={open === 'receivable'} title="Ghi nhận phải thu" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'project_id', label: 'Công trình', type: 'select', required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
          { name: 'receivable_code', label: 'Mã phải thu', defaultValue: `REC-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'client_name', label: 'Khách hàng', defaultValue: 'Khách hàng', required: true },
          { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number', defaultValue: 100000000, min: 0 },
          { name: 'milestone_name', label: 'Milestone', defaultValue: 'Đợt thanh toán', required: true },
          { name: 'gross_amount', label: 'Gross', type: 'number', defaultValue: 30000000, required: true, min: 0 },
          { name: 'received_amount', label: 'Đã thu', type: 'number', defaultValue: 0, min: 0 },
          { name: 'remaining_amount', label: 'Còn lại', type: 'number', defaultValue: 30000000, min: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'recorded', required: true, options: enumOptions('status', ['draft', 'recorded', 'approved', 'partially_received', 'received']) },
        ]} onSubmit={(values) => createReceivable.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.type === 'payable'}
        title="Chi tiết phải trả"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'payable_code', label: 'Mã' },
          { key: 'vendor_name', label: 'Vendor' },
          { key: 'project_name', label: 'Công trình' },
          { key: 'gross_amount', label: 'Gross', render: (row) => <CurrencyText value={row.gross_amount} /> },
          { key: 'remaining_amount', label: 'Còn phải trả', render: (row) => <CurrencyText value={row.remaining_amount} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <RecordDetailDialog
        open={dialog?.type === 'receivable'}
        title="Chi tiết phải thu"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'receivable_code', label: 'Mã' },
          { key: 'client_name', label: 'Khách hàng' },
          { key: 'project_name', label: 'Công trình' },
          { key: 'milestone_name', label: 'Milestone' },
          { key: 'gross_amount', label: 'Gross', render: (row) => <CurrencyText value={row.gross_amount} /> },
          { key: 'remaining_amount', label: 'Còn phải thu', render: (row) => <CurrencyText value={row.remaining_amount} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
    </>
  );
}

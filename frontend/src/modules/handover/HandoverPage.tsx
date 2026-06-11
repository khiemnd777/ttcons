import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ClipboardCheck, Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function HandoverPage() {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState(false);
  const [check, setCheck] = useState<Record<string, any> | null>(null);
  const [selected, setSelected] = useState<Record<string, any> | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const handover = useQuery({ queryKey: ['handover', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/handover`), enabled: Boolean(projectId) });
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/handover`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handover', projectId] });
      setOpen(false);
      toast.success('Đã tạo handover checklist');
    },
    onError: (error) => toast.error('Không tạo được handover checklist', error),
  });
  const checkMutation = useMutation({
    mutationFn: (handoverId: string) => post<Record<string, any>>(`/handover/${handoverId}/check`, {}),
    onSuccess: (result) => {
      setCheck(result);
      if (result.ready) toast.success('Đủ điều kiện bàn giao');
      else toast.warn({ title: 'Chưa đủ điều kiện bàn giao', description: Array.isArray(result.blockers) ? result.blockers.join(', ') : undefined });
    },
    onError: (error) => toast.error('Không kiểm tra được điều kiện bàn giao', error),
  });
  const complete = useMutation({
    mutationFn: (handoverId: string) => post(`/handover/${handoverId}/complete`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handover', projectId] });
      toast.success('Đã hoàn tất bàn giao');
    },
    onError: (error) => toast.error('Không hoàn tất được bàn giao', error),
  });

  return (
    <>
      <PageHeader
        title="Bàn giao"
        description="Closure checker trước bàn giao: nghiệm thu cuối, punch list, vật tư dư, kho, công nợ và chứng từ."
        actions={
          <>
            {!id ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Handover</button>
          </>
        }
      />
      {check ? (
        <div className={`mb-4 rounded-lg border p-4 ${check.ready ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50'}`}>
          <p className="font-semibold text-ink">{check.ready ? 'Đủ điều kiện bàn giao' : 'Chưa đủ điều kiện bàn giao'}</p>
          {check.blockers?.length ? <ul className="mt-2 list-disc pl-5 text-sm text-slate-700">{check.blockers.map((item: string) => <li key={item}>{item}</li>)}</ul> : null}
        </div>
      ) : null}
      <DataTable rows={handover.data ?? []} columns={[
        { key: 'client_representative', header: 'Đại diện khách hàng' },
        { key: 'company_representative', header: 'Đại diện công ty' },
        { key: 'documents_ready', header: 'Chứng từ', render: (row) => row.documents_ready ? 'Đủ' : 'Thiếu' },
        { key: 'warehouse_closed', header: 'Kho closure', render: (row) => row.warehouse_closed ? 'Đã đóng' : 'Chưa' },
        { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        { key: 'action', header: 'Action', render: (row) => (
          <RowActions onView={() => setSelected(row)}>
            <button title="Check" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => checkMutation.mutate(row.id)}><ClipboardCheck className="h-4 w-4" /></button>
            <button title="Complete" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => complete.mutate(row.id)}><CheckCircle2 className="h-4 w-4" /></button>
          </RowActions>
        ) },
      ]} />
      <FormDrawer open={open} title="Tạo handover checklist" onClose={() => setOpen(false)}>
        <SimpleForm fields={[
          { name: 'client_representative', label: 'Đại diện khách hàng', defaultValue: 'Khách hàng', required: true },
          { name: 'company_representative', label: 'Đại diện công ty', defaultValue: 'PM phụ trách', required: true },
          { name: 'documents_ready', label: 'Chứng từ đủ', type: 'checkbox' },
          { name: 'warehouse_closed', label: 'Kho đã closure', type: 'checkbox' },
          { name: 'payables_recorded', label: 'Phải trả đã ghi nhận', type: 'checkbox' },
          { name: 'receivables_recorded', label: 'Phải thu đã ghi nhận', type: 'checkbox' },
          { name: 'notes', label: 'Ghi chú', type: 'textarea' },
        ]} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={Boolean(selected)}
        title="Chi tiết bàn giao"
        record={selected}
        onClose={() => setSelected(null)}
        fields={[
          { key: 'client_representative', label: 'Đại diện khách hàng' },
          { key: 'company_representative', label: 'Đại diện công ty' },
          { key: 'documents_ready', label: 'Chứng từ đủ' },
          { key: 'warehouse_closed', label: 'Kho closure' },
          { key: 'payables_recorded', label: 'Phải trả đã ghi nhận' },
          { key: 'receivables_recorded', label: 'Phải thu đã ghi nhận' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'notes', label: 'Ghi chú' },
        ]}
      />
    </>
  );
}

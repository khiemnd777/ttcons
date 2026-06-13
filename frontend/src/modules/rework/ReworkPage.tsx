import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';

const reworkActionLabels: Record<string, string> = {
  complete: 'Đã hoàn tất rework',
  approve: 'Đã duyệt replanning',
};

export function ReworkPage() {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState<'rework' | 'replanning' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'rework' | 'replanning'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const reworks = useQuery({ queryKey: ['reworks', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/reworks`), enabled: Boolean(projectId) });
  const replannings = useQuery({ queryKey: ['replannings', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/replannings`), enabled: Boolean(projectId) });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['reworks', projectId] });
    queryClient.invalidateQueries({ queryKey: ['replannings', projectId] });
  };
  const createRework = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/reworks`, values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo rework');
    },
    onError: (error) => toast.error('Không tạo được rework', error),
  });
  const createReplanning = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/replannings`, values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo replanning');
    },
    onError: (error) => toast.error('Không tạo được replanning', error),
  });
  const action = useMutation({
    mutationFn: ({ path, actionName }: { path: string; actionName: string }) => post(`/${path}/${actionName}`, {}),
    onSuccess: (_data, variables) => {
      invalidate();
      toast.success(reworkActionLabels[variables.actionName] ?? 'Đã cập nhật rework');
    },
    onError: (error) => toast.error('Không cập nhật được rework', error),
  });

  return (
    <>
      <PageHeader
        title="Rework / Replanning"
        description="Rework khi sửa trong phạm vi hiện tại; Replanning khi ảnh hưởng thiết kế, BOQ, ngân sách hoặc timeline."
        actions={
          <>
            {id ? <ProjectBackLink projectId={projectId} /> : null}
            {!id ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setOpen('replanning')}><Plus className="h-4 w-4" />Replanning</button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('rework')}><Plus className="h-4 w-4" />Rework</button>
          </>
        }
      />
      <div className="grid gap-4 xl:grid-cols-2">
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Rework board</h2>
          <DataTable rows={reworks.data ?? []} columns={[
            { key: 'title', header: 'Rework', render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'due_date', header: 'Deadline' },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'rework', record: row })}><button className="text-sm font-semibold text-blue-700" onClick={() => action.mutate({ path: `reworks/${row.id}`, actionName: 'complete' })}>Complete</button></RowActions> },
          ]} />
        </section>
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Replanning requests</h2>
          <DataTable rows={replannings.data ?? []} columns={[
            { key: 'reason', header: 'Lý do', render: (row) => <span className="font-semibold text-ink">{row.reason}</span> },
            { key: 'impact_scope', header: 'Impact', render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
            { key: 'budget_impact', header: 'Ngân sách', render: (row) => <CurrencyText value={row.budget_impact} /> },
            { key: 'timeline_impact', header: 'Ngày' },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'replanning', record: row })}><button className="text-sm font-semibold text-blue-700" onClick={() => action.mutate({ path: `replannings/${row.id}`, actionName: 'approve' })}>Approve</button></RowActions> },
          ]} />
        </section>
      </div>
      <FormDrawer open={open === 'rework'} title="Tạo rework" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Rework mới', required: true },
          { name: 'description', label: 'Mô tả', type: 'textarea' },
          { name: 'due_date', label: 'Deadline', type: 'date' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'approved', 'in_progress']) },
        ]} onSubmit={(values) => createRework.mutateAsync(values)} />
      </FormDrawer>
      <FormDrawer open={open === 'replanning'} title="Tạo replanning" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: 'Issue lớn ảnh hưởng kế hoạch', required: true },
          { name: 'impact_scope', label: 'Impact scope', type: 'select', defaultValue: 'boq', required: true, options: enumOptions('impact_scope', ['design', 'boq', 'budget', 'timeline', 'material', 'subcontractor', 'client_requirement']) },
          { name: 'budget_impact', label: 'Ảnh hưởng ngân sách', type: 'number', defaultValue: 0 },
          { name: 'timeline_impact', label: 'Ảnh hưởng timeline (ngày)', type: 'number', defaultValue: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'submitted']) },
        ]} onSubmit={(values) => createReplanning.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.type === 'rework'}
        title="Chi tiết rework"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'title', label: 'Rework' },
          { key: 'description', label: 'Mô tả' },
          { key: 'due_date', label: 'Deadline' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <RecordDetailDialog
        open={dialog?.type === 'replanning'}
        title="Chi tiết replanning"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'reason', label: 'Lý do' },
          { key: 'impact_scope', label: 'Impact scope', render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
          { key: 'budget_impact', label: 'Ngân sách', render: (row) => <CurrencyText value={row.budget_impact} /> },
          { key: 'timeline_impact', label: 'Timeline' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
    </>
  );
}

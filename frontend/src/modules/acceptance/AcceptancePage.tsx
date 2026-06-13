import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus, RotateCcw, X } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';

const acceptanceActionLabels: Record<string, string> = {
  pass: 'Đã pass nghiệm thu',
  fail: 'Đã fail nghiệm thu',
  'create-rework': 'Đã tạo rework từ nghiệm thu',
};

export function AcceptancePage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<Record<string, any> | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const acceptances = useQuery({ queryKey: ['acceptances', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/acceptances`), enabled: Boolean(projectId) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/acceptances`, values),
    onSuccess: () => {
      invalidate();
      setOpen(false);
      toast.success('Đã tạo nghiệm thu');
    },
    onError: (error) => toast.error('Không tạo được nghiệm thu', error),
  });
  const action = useMutation({
    mutationFn: ({ acceptanceId, name }: { acceptanceId: string; name: string }) => post(`/acceptances/${acceptanceId}/${name}`, {}),
    onSuccess: (_data, variables) => {
      invalidate();
      toast.success(acceptanceActionLabels[variables.name] ?? 'Đã cập nhật nghiệm thu');
    },
    onError: (error) => toast.error('Không cập nhật được nghiệm thu', error),
  });

  return (
    <>
      <PageHeader
        title="Nghiệm thu"
        description="Tạo nghiệm thu, checklist, pass/fail, punch list, rework request và re-nghiệm thu."
        actions={
          <>
            {!global ? <ProjectBackLink projectId={projectId} /> : null}
            {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Nghiệm thu</button>
          </>
        }
      />
      <DataTable rows={acceptances.data ?? []} columns={[
        { key: 'title', header: 'Biên bản', render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
        { key: 'type', header: 'Loại', render: (row) => <EnumChip group="acceptance_type" value={row.type} /> },
        { key: 'result', header: 'Kết quả', render: (row) => <StatusBadge status={row.result} /> },
        { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        { key: 'notes', header: 'Ghi chú' },
        { key: 'action', header: 'Action', render: (row) => (
          <RowActions onView={() => setSelected(row)}>
            <button title="Pass" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ acceptanceId: row.id, name: 'pass' })}><Check className="h-4 w-4" /></button>
            <button title="Fail" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ acceptanceId: row.id, name: 'fail' })}><X className="h-4 w-4" /></button>
            <button title="Tạo rework" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ acceptanceId: row.id, name: 'create-rework' })}><RotateCcw className="h-4 w-4" /></button>
          </RowActions>
        ) },
      ]} />
      <FormDrawer open={open} title="Tạo nghiệm thu" onClose={() => setOpen(false)}>
        <SimpleForm fields={[
          { name: 'type', label: 'Loại', type: 'select', defaultValue: 'partial', required: true, options: enumOptions('acceptance_type', ['partial', 'final']) },
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Nghiệm thu hạng mục mới', required: true },
          { name: 'notes', label: 'Ghi chú', type: 'textarea' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'completed']) },
        ]} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={Boolean(selected)}
        title="Chi tiết nghiệm thu"
        record={selected}
        onClose={() => setSelected(null)}
        fields={[
          { key: 'title', label: 'Biên bản' },
          { key: 'type', label: 'Loại', render: (row) => <EnumChip group="acceptance_type" value={row.type} /> },
          { key: 'result', label: 'Kết quả', render: (row) => <StatusBadge status={row.result} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'notes', label: 'Ghi chú' },
          { key: 'created_at', label: 'Ngày tạo' },
        ]}
      />
    </>
  );
}

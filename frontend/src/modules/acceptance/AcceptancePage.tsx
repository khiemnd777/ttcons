import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus, RotateCcw, X } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { StatusBadge } from '../../components/StatusBadge';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function AcceptancePage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const acceptances = useQuery({ queryKey: ['acceptances', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/acceptances`), enabled: Boolean(projectId) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
  const create = useMutation({ mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/acceptances`, values), onSuccess: () => { invalidate(); setOpen(false); } });
  const action = useMutation({ mutationFn: ({ acceptanceId, name }: { acceptanceId: string; name: string }) => post(`/acceptances/${acceptanceId}/${name}`, {}), onSuccess: invalidate });

  return (
    <>
      <PageHeader
        title="Nghiệm thu"
        description="Tạo nghiệm thu, checklist, pass/fail, punch list, rework request và re-nghiệm thu."
        actions={
          <>
            {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Nghiệm thu</button>
          </>
        }
      />
      <DataTable rows={acceptances.data ?? []} columns={[
        { key: 'title', header: 'Biên bản', render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
        { key: 'type', header: 'Loại' },
        { key: 'result', header: 'Kết quả', render: (row) => <StatusBadge status={row.result} /> },
        { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        { key: 'notes', header: 'Ghi chú' },
        { key: 'action', header: 'Action', render: (row) => (
          <div className="flex gap-2">
            <button title="Pass" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ acceptanceId: row.id, name: 'pass' })}><Check className="h-4 w-4" /></button>
            <button title="Fail" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ acceptanceId: row.id, name: 'fail' })}><X className="h-4 w-4" /></button>
            <button title="Tạo rework" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => action.mutate({ acceptanceId: row.id, name: 'create-rework' })}><RotateCcw className="h-4 w-4" /></button>
          </div>
        ) },
      ]} />
      <FormDrawer open={open} title="Tạo nghiệm thu" onClose={() => setOpen(false)}>
        <SimpleForm fields={[
          { name: 'type', label: 'Loại', type: 'select', defaultValue: 'partial', options: [{ label: 'Bán phần', value: 'partial' }, { label: 'Toàn phần', value: 'final' }] },
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Nghiệm thu hạng mục mới' },
          { name: 'notes', label: 'Ghi chú', type: 'textarea' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', options: ['draft', 'completed'].map((x) => ({ label: x, value: x })) },
        ]} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
    </>
  );
}

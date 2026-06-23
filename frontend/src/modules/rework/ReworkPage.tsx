import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { ListSection } from '../../components/ListSection';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
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
  const [reworkStatus, setReworkStatus] = useState('all');
  const [replanningStatus, setReplanningStatus] = useState('all');
  const [open, setOpen] = useState<'rework' | 'replanning' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'rework' | 'replanning'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const reworks = useQuery({ queryKey: ['reworks', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/reworks`), enabled: Boolean(projectId) });
  const replannings = useQuery({ queryKey: ['replannings', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/replannings`), enabled: Boolean(projectId) });
  const reworkRows = reworks.data ?? [];
  const replanningRows = replannings.data ?? [];
  const filteredReworks = useMemo(() => reworkStatus === 'all' ? reworkRows : reworkRows.filter((row) => row.status === reworkStatus), [reworkRows, reworkStatus]);
  const filteredReplannings = useMemo(() => replanningStatus === 'all' ? replanningRows : replanningRows.filter((row) => row.status === replanningStatus), [replanningRows, replanningStatus]);
  const reworkFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: reworkRows.length },
    { value: 'draft', label: 'Draft', count: countBy(reworkRows, 'status', 'draft') },
    { value: 'approved', label: 'Approved', count: countBy(reworkRows, 'status', 'approved') },
    { value: 'in_progress', label: 'In progress', count: countBy(reworkRows, 'status', 'in_progress') },
    { value: 'done', label: 'Done', count: countBy(reworkRows, 'status', 'done') },
  ], [reworkRows]);
  const replanningFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: replanningRows.length },
    { value: 'draft', label: 'Draft', count: countBy(replanningRows, 'status', 'draft') },
    { value: 'submitted', label: 'Submitted', count: countBy(replanningRows, 'status', 'submitted') },
    { value: 'approved', label: 'Approved', count: countBy(replanningRows, 'status', 'approved') },
  ], [replanningRows]);
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
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('replanning')}><Plus className="h-4 w-4" />Replanning</button>
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('rework')}><Plus className="h-4 w-4" />Rework</button>
          </>
        }
      />
      <div className="mb-4 grid gap-3 xl:grid-cols-2">
        <FilterBar label="Rework status" value={reworkStatus} options={reworkFilterOptions} onChange={setReworkStatus} disabled={!projectId} />
        <FilterBar label="Replanning status" value={replanningStatus} options={replanningFilterOptions} onChange={setReplanningStatus} disabled={!projectId} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <ListSection title="Rework board">
          <DataTable rows={filteredReworks} loading={reworks.isLoading} error={reworks.error} onRetry={() => reworks.refetch()} searchPlaceholder="Tìm rework, mô tả, deadline..." pageSize={10} defaultSort={{ key: 'due_date', direction: 'asc' }} columns={[
            { key: 'title', header: 'Rework', sortable: true, filterValue: (row) => [row.title, row.description], render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'due_date', header: 'Deadline', sortable: true },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'rework', record: row })}><button disabled={row.status === 'done'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => action.mutate({ path: `reworks/${row.id}`, actionName: 'complete' })}>Complete</button></RowActions> },
          ]} empty={projectId ? 'Không có rework trong bộ lọc này' : 'Chọn công trình để xem rework'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('rework')}>
              <Plus className="h-4 w-4" />Rework
            </button>
          ) : undefined} />
        </ListSection>
        <ListSection title="Replanning requests">
          <DataTable rows={filteredReplannings} loading={replannings.isLoading} error={replannings.error} onRetry={() => replannings.refetch()} searchPlaceholder="Tìm lý do, impact, trạng thái..." pageSize={10} defaultSort={{ key: 'timeline_impact', direction: 'desc' }} columns={[
            { key: 'reason', header: 'Lý do', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.reason}</span> },
            { key: 'impact_scope', header: 'Impact', sortable: true, render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
            { key: 'budget_impact', header: 'Ngân sách', align: 'right', sortable: true, sortValue: (row) => Number(row.budget_impact ?? 0), render: (row) => <CurrencyText value={row.budget_impact} /> },
            { key: 'timeline_impact', header: 'Ngày', align: 'right', sortable: true, sortValue: (row) => Number(row.timeline_impact ?? 0) },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'replanning', record: row })}><button disabled={row.status === 'approved'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => action.mutate({ path: `replannings/${row.id}`, actionName: 'approve' })}>Approve</button></RowActions> },
          ]} empty={projectId ? 'Không có replanning trong bộ lọc này' : 'Chọn công trình để xem replanning'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpen('replanning')}>
              <Plus className="h-4 w-4" />Replanning
            </button>
          ) : undefined} />
        </ListSection>
      </div>
      <RecordFormDialog
        open={open === 'rework'}
        title="Tạo rework"
        submitLabel="Tạo rework"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Rework mới', required: true },
          { name: 'description', label: 'Mô tả', type: 'textarea' },
          { name: 'due_date', label: 'Deadline', type: 'date' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'approved', 'in_progress']) },
        ]}
        onSubmit={(values) => createRework.mutateAsync(values)}
      />
      <RecordFormDialog
        open={open === 'replanning'}
        title="Tạo replanning"
        submitLabel="Tạo replanning"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: 'Issue lớn ảnh hưởng kế hoạch', required: true },
          { name: 'impact_scope', label: 'Impact scope', type: 'select', defaultValue: 'boq', required: true, options: enumOptions('impact_scope', ['design', 'boq', 'budget', 'timeline', 'material', 'subcontractor', 'client_requirement']) },
          { name: 'budget_impact', label: 'Ảnh hưởng ngân sách', type: 'number', defaultValue: 0 },
          { name: 'timeline_impact', label: 'Ảnh hưởng timeline (ngày)', type: 'number', defaultValue: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'submitted']) },
        ]}
        onSubmit={(values) => createReplanning.mutateAsync(values)}
      />
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

function countBy(rows: Record<string, any>[], key: string, value: string) {
  return rows.filter((row) => row[key] === value).length;
}

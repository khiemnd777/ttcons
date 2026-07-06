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
  complete: 'Đã hoàn tất sửa lỗi',
  approve: 'Đã duyệt lập kế hoạch lại',
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
    { value: 'draft', label: 'Nháp', count: countBy(reworkRows, 'status', 'draft') },
    { value: 'approved', label: 'Đã duyệt', count: countBy(reworkRows, 'status', 'approved') },
    { value: 'in_progress', label: 'Đang xử lý', count: countBy(reworkRows, 'status', 'in_progress') },
    { value: 'done', label: 'Đã xong', count: countBy(reworkRows, 'status', 'done') },
  ], [reworkRows]);
  const replanningFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: replanningRows.length },
    { value: 'draft', label: 'Nháp', count: countBy(replanningRows, 'status', 'draft') },
    { value: 'submitted', label: 'Đã gửi', count: countBy(replanningRows, 'status', 'submitted') },
    { value: 'approved', label: 'Đã duyệt', count: countBy(replanningRows, 'status', 'approved') },
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
      toast.success('Đã tạo yêu cầu sửa lỗi');
    },
    onError: (error) => toast.error('Không tạo được yêu cầu sửa lỗi', error),
  });
  const createReplanning = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/replannings`, values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo yêu cầu lập kế hoạch lại');
    },
    onError: (error) => toast.error('Không tạo được yêu cầu lập kế hoạch lại', error),
  });
  const action = useMutation({
    mutationFn: ({ path, actionName }: { path: string; actionName: string }) => post(`/${path}/${actionName}`, {}),
    onSuccess: (_data, variables) => {
      invalidate();
      toast.success(reworkActionLabels[variables.actionName] ?? 'Đã cập nhật yêu cầu');
    },
    onError: (error) => toast.error('Không cập nhật được yêu cầu', error),
  });

  return (
    <>
      <PageHeader
        title="Sửa lỗi / Lập kế hoạch lại"
        description="Sửa lỗi khi xử lý trong phạm vi hiện tại; lập kế hoạch lại khi ảnh hưởng thiết kế, BOQ, ngân sách hoặc tiến độ."
        actions={
          <>
            {id ? <ProjectBackLink projectId={projectId} /> : null}
            {!id ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('replanning')}><Plus className="h-4 w-4" />Lập kế hoạch lại</button>
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('rework')}><Plus className="h-4 w-4" />Sửa lỗi</button>
          </>
        }
      />
      <div className="mb-4 grid gap-3 xl:grid-cols-2">
        <FilterBar label="Trạng thái sửa lỗi" value={reworkStatus} options={reworkFilterOptions} onChange={setReworkStatus} disabled={!projectId} />
        <FilterBar label="Trạng thái lập kế hoạch lại" value={replanningStatus} options={replanningFilterOptions} onChange={setReplanningStatus} disabled={!projectId} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <ListSection title="Bảng sửa lỗi">
          <DataTable rows={filteredReworks} loading={reworks.isLoading} error={reworks.error} onRetry={() => reworks.refetch()} searchPlaceholder="Tìm yêu cầu sửa lỗi, mô tả, hạn xử lý..." pageSize={10} defaultSort={{ key: 'due_date', direction: 'asc' }} columns={[
            { key: 'title', header: 'Sửa lỗi', sortable: true, filterValue: (row) => [row.title, row.description], render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'due_date', header: 'Hạn xử lý', sortable: true },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'rework', record: row })}><button disabled={row.status === 'done'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => action.mutate({ path: `reworks/${row.id}`, actionName: 'complete' })}>Hoàn tất</button></RowActions> },
          ]} empty={projectId ? 'Không có yêu cầu sửa lỗi trong bộ lọc này' : 'Chọn công trình để xem yêu cầu sửa lỗi'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('rework')}>
              <Plus className="h-4 w-4" />Sửa lỗi
            </button>
          ) : undefined} />
        </ListSection>
        <ListSection title="Yêu cầu lập kế hoạch lại">
          <DataTable rows={filteredReplannings} loading={replannings.isLoading} error={replannings.error} onRetry={() => replannings.refetch()} searchPlaceholder="Tìm lý do, ảnh hưởng, trạng thái..." pageSize={10} defaultSort={{ key: 'timeline_impact', direction: 'desc' }} columns={[
            { key: 'reason', header: 'Lý do', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.reason}</span> },
            { key: 'impact_scope', header: 'Ảnh hưởng', sortable: true, render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
            { key: 'budget_impact', header: 'Ngân sách', align: 'right', sortable: true, sortValue: (row) => Number(row.budget_impact ?? 0), render: (row) => <CurrencyText value={row.budget_impact} /> },
            { key: 'timeline_impact', header: 'Ngày', align: 'right', sortable: true, sortValue: (row) => Number(row.timeline_impact ?? 0) },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'replanning', record: row })}><button disabled={row.status === 'approved'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => action.mutate({ path: `replannings/${row.id}`, actionName: 'approve' })}>Duyệt</button></RowActions> },
          ]} empty={projectId ? 'Không có yêu cầu lập kế hoạch lại trong bộ lọc này' : 'Chọn công trình để xem yêu cầu lập kế hoạch lại'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpen('replanning')}>
              <Plus className="h-4 w-4" />Lập kế hoạch lại
            </button>
          ) : undefined} />
        </ListSection>
      </div>
      <RecordFormDialog
        open={open === 'rework'}
        title="Tạo yêu cầu sửa lỗi"
        submitLabel="Tạo yêu cầu sửa lỗi"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Yêu cầu sửa lỗi mới', required: true },
          { name: 'description', label: 'Mô tả', type: 'textarea' },
          { name: 'due_date', label: 'Hạn xử lý', type: 'date' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'approved', 'in_progress']) },
        ]}
        onSubmit={(values) => createRework.mutateAsync(values)}
      />
      <RecordFormDialog
        open={open === 'replanning'}
        title="Tạo yêu cầu lập kế hoạch lại"
        submitLabel="Tạo yêu cầu lập kế hoạch lại"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: 'Issue lớn ảnh hưởng kế hoạch', required: true },
          { name: 'impact_scope', label: 'Phạm vi ảnh hưởng', type: 'select', defaultValue: 'boq', required: true, options: enumOptions('impact_scope', ['design', 'boq', 'budget', 'timeline', 'material', 'subcontractor', 'client_requirement']) },
          { name: 'budget_impact', label: 'Ảnh hưởng ngân sách', type: 'number', defaultValue: 0 },
          { name: 'timeline_impact', label: 'Ảnh hưởng tiến độ (ngày)', type: 'number', defaultValue: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'submitted']) },
        ]}
        onSubmit={(values) => createReplanning.mutateAsync(values)}
      />
      <RecordDetailDialog
        open={dialog?.type === 'rework'}
        title="Chi tiết sửa lỗi"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'title', label: 'Sửa lỗi' },
          { key: 'description', label: 'Mô tả' },
          { key: 'due_date', label: 'Hạn xử lý' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <RecordDetailDialog
        open={dialog?.type === 'replanning'}
        title="Chi tiết lập kế hoạch lại"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'reason', label: 'Lý do' },
          { key: 'impact_scope', label: 'Phạm vi ảnh hưởng', render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
          { key: 'budget_impact', label: 'Ngân sách', render: (row) => <CurrencyText value={row.budget_impact} /> },
          { key: 'timeline_impact', label: 'Tiến độ' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
    </>
  );
}

function countBy(rows: Record<string, any>[], key: string, value: string) {
  return rows.filter((row) => row[key] === value).length;
}

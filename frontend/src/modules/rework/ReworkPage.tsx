import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import { PageHeader } from '../../components/PageHeader';
import { ContextHelp } from '../../components/ContextHelp';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RecordSummary } from '../../components/RecordDetailLayout';
import { RelatedRecords } from '../_shared/RecordLinks';
import { useRecordDeepLink } from '../_shared/useRecordDeepLink';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { ListSection } from '../../components/ListSection';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';
import { useAuth } from '../../lib/auth/AuthContext';

const reworkActionLabels: Record<string, string> = {
  complete: 'Đã hoàn tất sửa lỗi',
  approve: 'Đã duyệt lập kế hoạch lại',
};

export function ReworkPage({ embedded = false, projectIdOverride }: { embedded?: boolean; projectIdOverride?: string } = {}) {
  const { id: routeId } = useParams();
  const [projectId, setProjectId] = useState(projectIdOverride ?? routeId ?? '');
  const [reworkStatus, setReworkStatus] = useState('all');
  const [replanningStatus, setReplanningStatus] = useState('all');
  const [open, setOpen] = useState<'rework' | 'replanning' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'rework' | 'replanning'; record: Record<string, any> } | null>(null);
  const reworkLink = useRecordDeepLink({ kind: 'rework', projectId: projectIdOverride ?? routeId, onOpen: record => { setProjectId(record.project_id); setDialog({ type: 'rework', record }); } });
  const replanningLink = useRecordDeepLink({ kind: 'replanning', projectId: projectIdOverride ?? routeId, onOpen: record => { setProjectId(record.project_id); setDialog({ type: 'replanning', record }); } });
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canViewReworks = hasAnyPermission(['rework.view']);
  const canCreateRework = hasAnyPermission(['rework.create']);
  const canApproveRework = hasAnyPermission(['rework.approve']);
  const canViewReplanning = hasAnyPermission(['replanning.view']);
  const canCreateReplanning = hasAnyPermission(['replanning.create']);
  const canApproveReplanning = hasAnyPermission(['replanning.approve']);
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  useEffect(() => { if (projectIdOverride && projectIdOverride !== projectId) setProjectId(projectIdOverride); }, [projectId, projectIdOverride]);
  const reworks = useQuery({ queryKey: ['reworks', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/reworks`), enabled: Boolean(projectId) && canViewReworks });
  const replannings = useQuery({ queryKey: ['replannings', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/replannings`), enabled: Boolean(projectId) && canViewReplanning });
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
      {!embedded ? <PageHeader
        title="Sửa lỗi / Lập kế hoạch lại"
        actions={
          <>
            <ContextHelp label="Phân biệt sửa lỗi và lập kế hoạch lại">Sửa lỗi trong phạm vi hiện tại; lập kế hoạch lại khi ảnh hưởng thiết kế, BOQ, ngân sách hoặc tiến độ.</ContextHelp>
            {routeId ? <ProjectBackLink projectId={projectId} /> : null}
            {!routeId ? <div className="w-full sm:w-80"><AsyncProjectPicker value={projectId} onChange={setProjectId} endpoint="/projects/options" queryKey="project-options" emptyLabel="Chọn công trình" allowEmptySelection={false} /></div> : null}
            {canCreateReplanning ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('replanning')}><Plus className="h-4 w-4" />Lập kế hoạch lại</button> : null}
            {canCreateRework ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('rework')}><Plus className="h-4 w-4" />Sửa lỗi</button> : null}
          </>
        }
      /> : <div className="mb-4 flex flex-col gap-3 rounded-lg border border-line bg-white px-4 py-3 sm:flex-row sm:items-center sm:justify-between"><div><h2 className="font-semibold text-ink">Yêu cầu khắc phục & lập kế hoạch lại</h2><ContextHelp label="Phân biệt sửa lỗi và lập kế hoạch lại">Sửa lỗi trong phạm vi hiện tại; lập kế hoạch lại khi ảnh hưởng thiết kế, BOQ, ngân sách hoặc tiến độ.</ContextHelp></div><div className="flex flex-wrap gap-2">{canCreateReplanning ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:opacity-50" onClick={() => setOpen('replanning')}><Plus className="h-4 w-4" /> Lập kế hoạch lại</button> : null}{canCreateRework ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={() => setOpen('rework')}><Plus className="h-4 w-4" /> Yêu cầu sửa lỗi</button> : null}</div></div>}
      <div className={`mb-4 grid gap-3 ${canViewReworks && canViewReplanning ? 'xl:grid-cols-2' : ''}`}>
        {canViewReworks ? <FilterBar label="Trạng thái sửa lỗi" value={reworkStatus} options={reworkFilterOptions} onChange={setReworkStatus} disabled={!projectId} /> : null}
        {canViewReplanning ? <FilterBar label="Trạng thái lập kế hoạch lại" value={replanningStatus} options={replanningFilterOptions} onChange={setReplanningStatus} disabled={!projectId} /> : null}
      </div>
      <div className={`grid gap-4 ${canViewReworks && canViewReplanning ? 'xl:grid-cols-2' : ''}`}>
        {canViewReworks ? <ListSection title="Bảng sửa lỗi">
          <DataTable rows={filteredReworks} loading={reworks.isLoading} error={reworks.error} onRetry={() => reworks.refetch()} searchPlaceholder="Tìm yêu cầu sửa lỗi, mô tả, hạn xử lý..." pageSize={10} defaultSort={{ key: 'due_date', direction: 'asc' }} columns={[
            { key: 'title', header: 'Sửa lỗi', sortable: true, filterValue: (row) => [row.title, row.description], render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'due_date', header: 'Hạn xử lý', sortable: true, render: (row) => dateText(row.due_date) },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'rework', record: row })}>{canApproveRework ? <button disabled={row.status === 'done'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => action.mutate({ path: `reworks/${row.id}`, actionName: 'complete' })}>Hoàn tất</button> : null}</RowActions> },
          ]} empty={projectId ? 'Không có yêu cầu sửa lỗi trong bộ lọc này' : 'Chọn công trình để xem yêu cầu sửa lỗi'} emptyAction={projectId && canCreateRework ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('rework')}>
              <Plus className="h-4 w-4" />Sửa lỗi
            </button>
          ) : undefined} />
        </ListSection> : null}
        {canViewReplanning ? <ListSection title="Yêu cầu lập kế hoạch lại">
          <DataTable rows={filteredReplannings} loading={replannings.isLoading} error={replannings.error} onRetry={() => replannings.refetch()} searchPlaceholder="Tìm lý do, ảnh hưởng, trạng thái..." pageSize={10} defaultSort={{ key: 'timeline_impact', direction: 'desc' }} columns={[
            { key: 'reason', header: 'Lý do', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.reason}</span> },
            { key: 'impact_scope', header: 'Ảnh hưởng', sortable: true, render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
            { key: 'budget_impact', header: 'Ngân sách', align: 'right', sortable: true, sortValue: (row) => Number(row.budget_impact ?? 0), render: (row) => <CurrencyText value={row.budget_impact} /> },
            { key: 'timeline_impact', header: 'Ngày', align: 'right', sortable: true, sortValue: (row) => Number(row.timeline_impact ?? 0) },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'replanning', record: row })}>{canApproveReplanning ? <button disabled={row.status === 'approved'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => action.mutate({ path: `replannings/${row.id}`, actionName: 'approve' })}>Duyệt</button> : null}</RowActions> },
          ]} empty={projectId ? 'Không có yêu cầu lập kế hoạch lại trong bộ lọc này' : 'Chọn công trình để xem yêu cầu lập kế hoạch lại'} emptyAction={projectId && canCreateReplanning ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpen('replanning')}>
              <Plus className="h-4 w-4" />Lập kế hoạch lại
            </button>
          ) : undefined} />
        </ListSection> : null}
      </div>
      <RecordFormDialog
        open={open === 'rework'}
        title="Tạo yêu cầu sửa lỗi"
        description="Dùng khi xử lý trong phạm vi hiện tại."
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
        description="Dùng khi ảnh hưởng thiết kế, BOQ, ngân sách hoặc tiến độ."
        submitLabel="Tạo yêu cầu lập kế hoạch lại"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: 'Issue lớn ảnh hưởng kế hoạch', required: true },
          { name: 'impact_scope', label: 'Phạm vi ảnh hưởng', type: 'select', defaultValue: 'boq', required: true, options: enumOptions('impact_scope', ['design', 'boq', 'budget', 'timeline', 'material', 'subcontractor', 'client_requirement']) },
          { name: 'budget_impact', label: 'Ảnh hưởng ngân sách', type: 'currency', defaultValue: 0, allowNegative: true },
          { name: 'timeline_impact', label: 'Ảnh hưởng tiến độ (ngày)', type: 'number', defaultValue: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'draft', required: true, options: enumOptions('status', ['draft', 'submitted']) },
        ]}
        onSubmit={(values) => createReplanning.mutateAsync(values)}
      />
      {reworkLink.feedback}
      {replanningLink.feedback}
      <RecordDetailDialog
        open={dialog?.type === 'rework'}
        title="Chi tiết sửa lỗi"
        summary={row => <RecordSummary title={row.title} status={<StatusBadge status={row.status} />} subtitle={`Hạn xử lý: ${dateText(row.due_date)}`} />}
        sections={[{ title: 'Nội dung cần xử lý', keys: ['description'] }]}
        record={dialog?.record ?? null}
        related={dialog?.type === 'rework' ? <RelatedRecords kind="rework" record={dialog.record} /> : null}
        onClose={() => { setDialog(null); if (reworkLink.active) reworkLink.dismiss(); }}
        fields={[
          { key: 'title', label: 'Sửa lỗi' },
          { key: 'description', label: 'Mô tả', wide: true },
            { key: 'due_date', label: 'Hạn xử lý', render: (row) => dateText(row.due_date) },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <RecordDetailDialog
        open={dialog?.type === 'replanning'}
        title="Chi tiết lập kế hoạch lại"
        summary={row => <RecordSummary title="Yêu cầu lập kế hoạch lại" subtitle={<EnumChip group="impact_scope" value={row.impact_scope} />} status={<StatusBadge status={row.status} />} value={<CurrencyText value={row.budget_impact} />} valueLabel="Ảnh hưởng ngân sách" />}
        sections={[{ title: 'Nguyên nhân', keys: ['reason'] }, { title: 'Ảnh hưởng tiến độ', keys: ['timeline_impact'] }]}
        record={dialog?.record ?? null}
        related={dialog?.type === 'replanning' ? <RelatedRecords kind="replanning" record={dialog.record} /> : null}
        onClose={() => { setDialog(null); if (replanningLink.active) replanningLink.dismiss(); }}
        fields={[
          { key: 'reason', label: 'Lý do', wide: true },
          { key: 'impact_scope', label: 'Phạm vi ảnh hưởng', render: (row) => <EnumChip group="impact_scope" value={row.impact_scope} /> },
          { key: 'budget_impact', label: 'Ngân sách', render: (row) => <CurrencyText value={row.budget_impact} /> },
          { key: 'timeline_impact', label: 'Thay đổi tiến độ', render: row => row.timeline_impact == null ? '—' : `${row.timeline_impact} ngày` },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
    </>
  );
}

function countBy(rows: Record<string, any>[], key: string, value: string) {
  return rows.filter((row) => row[key] === value).length;
}

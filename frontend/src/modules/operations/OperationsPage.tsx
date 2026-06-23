import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { ListSection } from '../../components/ListSection';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { dateText } from '../../lib/utils/format';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';

export function OperationsPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [issueStatus, setIssueStatus] = useState('all');
  const [open, setOpen] = useState<'report' | 'issue' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'report' | 'issue'; mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => {
    if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id);
  }, [projects.data, projectId]);
  const reports = useQuery({ queryKey: ['daily-reports', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/daily-reports`), enabled: Boolean(projectId) });
  const issues = useQuery({ queryKey: ['issues', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/issues`), enabled: Boolean(projectId) });
  const reportRows = reports.data ?? [];
  const issueRows = issues.data ?? [];
  const filteredIssues = useMemo(() => issueStatus === 'all' ? issueRows : issueRows.filter((row) => row.status === issueStatus), [issueRows, issueStatus]);
  const issueFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: issueRows.length },
    { value: 'open', label: 'Open', count: countBy(issueRows, 'status', 'open') },
    { value: 'in_progress', label: 'In progress', count: countBy(issueRows, 'status', 'in_progress') },
    { value: 'closed', label: 'Closed', count: countBy(issueRows, 'status', 'closed') },
  ], [issueRows]);
  const openIssueCount = issueRows.filter((row) => row.status === 'open' || row.status === 'in_progress').length;
  const criticalIssueCount = issueRows.filter((row) => row.severity === 'critical' || row.severity === 'high').length;
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['daily-reports', projectId] });
    queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
  };
  const createReport = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/daily-reports`, values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo daily report');
    },
    onError: (error) => toast.error('Không tạo được daily report', error),
  });
  const createIssue = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/issues`, values),
    onSuccess: () => {
      invalidate();
      setOpen(null);
      toast.success('Đã tạo issue');
    },
    onError: (error) => toast.error('Không tạo được issue', error),
  });
  const closeIssue = useMutation({
    mutationFn: (issueId: string) => put(`/issues/${issueId}`, { status: 'closed' }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã đóng issue');
    },
    onError: (error) => toast.error('Không đóng được issue', error),
  });
  const updateIssue = useMutation({
    mutationFn: ({ issueId, values }: { issueId: string; values: Record<string, any> }) => put(`/issues/${issueId}`, values),
    onSuccess: () => {
      invalidate();
      setDialog(null);
      toast.success('Đã cập nhật issue');
    },
    onError: (error) => toast.error('Không cập nhật được issue', error),
  });

  const issueFields = (record?: Record<string, any>) => [
    { name: 'title', label: 'Tiêu đề', defaultValue: record?.title ?? 'Issue phát sinh', required: true, fullWidth: true, helperText: 'Tên ngắn gọn để đội thi công nhận diện vấn đề.' },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '', helperText: 'Ghi rõ vị trí, ảnh hưởng và bước xử lý tạm thời nếu có.' },
    { name: 'severity', label: 'Mức độ', type: 'select' as const, defaultValue: record?.severity ?? 'medium', required: true, options: enumOptions('severity', ['low', 'medium', 'high', 'critical']), helperText: 'Dùng Critical khi issue chặn nghiệm thu hoặc an toàn.' },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'open', required: true, options: enumOptions('status', ['open', 'in_progress', 'closed']), helperText: 'Issue mới thường bắt đầu ở Open.' },
    { name: 'requires_rework', label: 'Cần rework', type: 'checkbox' as const, defaultValue: Boolean(record?.requires_rework), helperText: 'Bật khi cần tạo luồng sửa lỗi sau nghiệm thu hoặc kiểm tra nội bộ.' },
    { name: 'requires_replanning', label: 'Cần replanning', type: 'checkbox' as const, defaultValue: Boolean(record?.requires_replanning), helperText: 'Bật khi issue ảnh hưởng timeline hoặc thứ tự thi công.' },
  ];

  return (
    <>
      <PageHeader
        title="Thi công"
        description="Daily report, ảnh hiện trường, issue phát sinh, tiến độ theo node, vật tư đã dùng và nhân công/thầu phụ."
        actions={
          <>
            {!global ? <ProjectBackLink projectId={projectId} /> : null}
            {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('issue')}><Plus className="h-4 w-4" />Issue</button>
            <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setOpen('report')}><Plus className="h-4 w-4" />Daily report</button>
          </>
        }
      />
      <div className="mb-4 grid gap-3 xl:grid-cols-[minmax(0,1fr)_auto]">
        <FilterBar label="Trạng thái issue" value={issueStatus} options={issueFilterOptions} onChange={setIssueStatus} disabled={!projectId} />
        <div className="flex flex-wrap items-center gap-2 text-xs font-semibold text-slate-600">
          <span className="rounded-md border border-line bg-white px-3 py-2">Reports: {reportRows.length}</span>
          <span className="rounded-md border border-line bg-white px-3 py-2">Issue mở: {openIssueCount}</span>
          <span className="rounded-md border border-line bg-white px-3 py-2">High/Critical: {criticalIssueCount}</span>
        </div>
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <ListSection title="Daily reports">
          <DataTable rows={reportRows} loading={reports.isLoading} error={reports.error} onRetry={() => reports.refetch()} searchPlaceholder="Tìm ngày, công việc, thời tiết..." pageSize={10} defaultSort={{ key: 'report_date', direction: 'desc' }} columns={[
            { key: 'report_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.report_date) },
            { key: 'weather', header: 'Thời tiết', sortable: true },
            { key: 'work_summary', header: 'Công việc', filterValue: (row) => [row.work_summary, row.manpower_summary, row.material_usage_summary, row.issues_summary] },
            { key: 'progress_percent', header: 'Tiến độ', align: 'right', sortable: true, sortValue: (row) => Number(row.progress_percent ?? 0), render: (row) => `${row.progress_percent}%` },
            { key: 'actions', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'report', mode: 'view', record: row })} /> },
          ]} empty={projectId ? 'Chưa có daily report' : 'Chọn công trình để xem daily report'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('report')}>
              <Plus className="h-4 w-4" />Daily report
            </button>
          ) : undefined} />
        </ListSection>
        <ListSection title="Issue phát sinh">
          <DataTable rows={filteredIssues} loading={issues.isLoading} error={issues.error} onRetry={() => issues.refetch()} searchPlaceholder="Tìm issue, mô tả, mức độ..." pageSize={10} defaultSort={{ key: 'severity', direction: 'desc' }} columns={[
            { key: 'title', header: 'Issue', sortable: true, filterValue: (row) => [row.title, row.description], render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'severity', header: 'Mức độ', sortable: true, render: (row) => <EnumChip group="severity" value={row.severity} /> },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            { key: 'requires_replanning', header: 'Replanning', align: 'center', render: (row) => row.requires_replanning ? 'Có' : 'Không' },
            {
              key: 'action',
              header: 'Action',
              render: (row) => (
                <RowActions onView={() => setDialog({ type: 'issue', mode: 'view', record: row })} onEdit={() => setDialog({ type: 'issue', mode: 'edit', record: row })}>
                  <button disabled={row.status === 'closed'} className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:text-slate-400" onClick={() => closeIssue.mutate(row.id)}>Đóng</button>
                </RowActions>
              ),
            },
          ]} empty={projectId ? 'Chưa có issue trong bộ lọc này' : 'Chọn công trình để xem issue'} emptyAction={projectId ? (
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpen('issue')}>
              <Plus className="h-4 w-4" />Issue
            </button>
          ) : undefined} />
        </ListSection>
      </div>
      <RecordFormDialog
        open={open === 'report'}
        title="Tạo daily report"
        submitLabel="Tạo daily report"
        layout="two-column"
        onClose={() => setOpen(null)}
        fields={[
          { name: 'report_date', label: 'Ngày report', type: 'date', defaultValue: new Date().toISOString().slice(0, 10), required: true },
          { name: 'weather', label: 'Thời tiết', defaultValue: 'Nắng', required: true },
          { name: 'work_summary', label: 'Công việc', type: 'textarea', defaultValue: 'Thi công theo kế hoạch ngày', required: true, helperText: 'Tóm tắt phần đã làm trong ngày theo khu vực hoặc hạng mục.' },
          { name: 'manpower_summary', label: 'Nhân công', defaultValue: '8 công nhân', helperText: 'Ghi số lượng hoặc nhóm đội có mặt tại công trình.' },
          { name: 'material_usage_summary', label: 'Vật tư sử dụng', defaultValue: 'Theo phiếu xuất kho', helperText: 'Ghi vật tư chính đã dùng hoặc liên kết phiếu xuất kho nếu có.' },
          { name: 'issues_summary', label: 'Vấn đề', defaultValue: 'Không có', helperText: 'Ghi Không có nếu không phát sinh issue trong ngày.' },
          { name: 'progress_percent', label: 'Tiến độ %', type: 'number', defaultValue: 50, min: 0, max: 100, helperText: 'Giá trị từ 0 đến 100.' },
        ]}
        onSubmit={(values) => createReport.mutateAsync(values)}
      />
      <RecordFormDialog
        open={open === 'issue'}
        title="Tạo issue"
        submitLabel="Tạo issue"
        layout="two-column"
        fields={issueFields()}
        onSubmit={(values) => createIssue.mutateAsync(values)}
        onClose={() => setOpen(null)}
      />
      <RecordDetailDialog
        open={dialog?.type === 'report' && dialog.mode === 'view'}
        title="Chi tiết daily report"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'report_date', label: 'Ngày', render: (row) => dateText(row.report_date) },
          { key: 'weather', label: 'Thời tiết' },
          { key: 'work_summary', label: 'Công việc' },
          { key: 'manpower_summary', label: 'Nhân công' },
          { key: 'material_usage_summary', label: 'Vật tư sử dụng' },
          { key: 'issues_summary', label: 'Vấn đề' },
          { key: 'progress_percent', label: 'Tiến độ', render: (row) => `${row.progress_percent ?? 0}%` },
        ]}
      />
      <RecordDetailDialog
        open={dialog?.type === 'issue' && dialog.mode === 'view'}
        title="Chi tiết issue"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'title', label: 'Tiêu đề' },
          { key: 'description', label: 'Mô tả' },
          { key: 'severity', label: 'Mức độ', render: (row) => <EnumChip group="severity" value={row.severity} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'requires_rework', label: 'Cần rework' },
          { key: 'requires_replanning', label: 'Cần replanning' },
        ]}
      />
      <RecordFormDialog
        open={dialog?.type === 'issue' && dialog.mode === 'edit'}
        title="Chỉnh sửa issue"
        submitLabel="Lưu thay đổi"
        layout="two-column"
        fields={dialog?.record ? issueFields(dialog.record) : []}
        onSubmit={(values) => dialog?.record ? updateIssue.mutateAsync({ issueId: dialog.record.id, values }) : undefined}
        onClose={() => setDialog(null)}
      />
    </>
  );
}

function countBy(rows: Record<string, any>[], key: string, value: string) {
  return rows.filter((row) => row[key] === value).length;
}

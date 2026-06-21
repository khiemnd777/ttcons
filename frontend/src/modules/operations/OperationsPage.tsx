import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
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
import { dateText } from '../../lib/utils/format';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';

export function OperationsPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
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
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setOpen('issue')}><Plus className="h-4 w-4" />Issue</button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('report')}><Plus className="h-4 w-4" />Daily report</button>
          </>
        }
      />
      <div className="grid gap-4 xl:grid-cols-2">
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Daily reports</h2>
          <DataTable rows={reports.data ?? []} columns={[
            { key: 'report_date', header: 'Ngày', render: (row) => dateText(row.report_date) },
            { key: 'weather', header: 'Thời tiết' },
            { key: 'work_summary', header: 'Công việc' },
            { key: 'progress_percent', header: 'Tiến độ', render: (row) => `${row.progress_percent}%` },
            { key: 'actions', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'report', mode: 'view', record: row })} /> },
          ]} empty="Chưa có daily report" />
        </section>
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Issue phát sinh</h2>
          <DataTable rows={issues.data ?? []} columns={[
            { key: 'title', header: 'Issue', render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'severity', header: 'Mức độ', render: (row) => <EnumChip group="severity" value={row.severity} /> },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'requires_replanning', header: 'Replanning', render: (row) => row.requires_replanning ? 'Có' : 'Không' },
            {
              key: 'action',
              header: 'Action',
              render: (row) => (
                <RowActions onView={() => setDialog({ type: 'issue', mode: 'view', record: row })} onEdit={() => setDialog({ type: 'issue', mode: 'edit', record: row })}>
                  <button className="text-sm font-semibold text-blue-700" onClick={() => closeIssue.mutate(row.id)}>Đóng</button>
                </RowActions>
              ),
            },
          ]} empty="Chưa có issue" />
        </section>
      </div>
      <FormDrawer open={open === 'report'} title="Tạo daily report" onClose={() => setOpen(null)}>
        <SimpleForm layout="two-column" fields={[
          { name: 'report_date', label: 'Ngày report', type: 'date', defaultValue: new Date().toISOString().slice(0, 10), required: true },
          { name: 'weather', label: 'Thời tiết', defaultValue: 'Nắng', required: true },
          { name: 'work_summary', label: 'Công việc', type: 'textarea', defaultValue: 'Thi công theo kế hoạch ngày', required: true, helperText: 'Tóm tắt phần đã làm trong ngày theo khu vực hoặc hạng mục.' },
          { name: 'manpower_summary', label: 'Nhân công', defaultValue: '8 công nhân', helperText: 'Ghi số lượng hoặc nhóm đội có mặt tại công trình.' },
          { name: 'material_usage_summary', label: 'Vật tư sử dụng', defaultValue: 'Theo phiếu xuất kho', helperText: 'Ghi vật tư chính đã dùng hoặc liên kết phiếu xuất kho nếu có.' },
          { name: 'issues_summary', label: 'Vấn đề', defaultValue: 'Không có', helperText: 'Ghi Không có nếu không phát sinh issue trong ngày.' },
          { name: 'progress_percent', label: 'Tiến độ %', type: 'number', defaultValue: 50, min: 0, max: 100, helperText: 'Giá trị từ 0 đến 100.' },
        ]} onSubmit={(values) => createReport.mutateAsync(values)} />
      </FormDrawer>
      <FormDrawer open={open === 'issue'} title="Tạo issue" onClose={() => setOpen(null)}>
        <SimpleForm layout="two-column" fields={issueFields()} onSubmit={(values) => createIssue.mutateAsync(values)} />
      </FormDrawer>
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
      <FormDrawer open={dialog?.type === 'issue' && dialog.mode === 'edit'} title="Chỉnh sửa issue" onClose={() => setDialog(null)}>
        {dialog?.record ? <SimpleForm layout="two-column" submitLabel="Lưu thay đổi" fields={issueFields(dialog.record)} onSubmit={(values) => updateIssue.mutateAsync({ issueId: dialog.record.id, values })} /> : null}
      </FormDrawer>
    </>
  );
}

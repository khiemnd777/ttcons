import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { StatusBadge } from '../../components/StatusBadge';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { dateText } from '../../lib/utils/format';

export function OperationsPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState<'report' | 'issue' | null>(null);
  const queryClient = useQueryClient();
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
  const createReport = useMutation({ mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/daily-reports`, values), onSuccess: () => { invalidate(); setOpen(null); } });
  const createIssue = useMutation({ mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/issues`, values), onSuccess: () => { invalidate(); setOpen(null); } });
  const closeIssue = useMutation({ mutationFn: (issueId: string) => put(`/issues/${issueId}`, { status: 'closed' }), onSuccess: invalidate });

  return (
    <>
      <PageHeader
        title="Thi công"
        description="Daily report, ảnh hiện trường, issue phát sinh, tiến độ theo node, vật tư đã dùng và nhân công/thầu phụ."
        actions={
          <>
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
          ]} empty="Chưa có daily report" />
        </section>
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Issue phát sinh</h2>
          <DataTable rows={issues.data ?? []} columns={[
            { key: 'title', header: 'Issue', render: (row) => <span className="font-semibold text-ink">{row.title}</span> },
            { key: 'severity', header: 'Mức độ' },
            { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'requires_replanning', header: 'Replanning', render: (row) => row.requires_replanning ? 'Có' : 'Không' },
            { key: 'action', header: 'Action', render: (row) => <button className="text-sm font-semibold text-blue-700" onClick={() => closeIssue.mutate(row.id)}>Đóng</button> },
          ]} empty="Chưa có issue" />
        </section>
      </div>
      <FormDrawer open={open === 'report'} title="Tạo daily report" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'report_date', label: 'Ngày report', type: 'date', defaultValue: new Date().toISOString().slice(0, 10) },
          { name: 'weather', label: 'Thời tiết', defaultValue: 'Nắng' },
          { name: 'work_summary', label: 'Công việc', type: 'textarea', defaultValue: 'Thi công theo kế hoạch ngày' },
          { name: 'manpower_summary', label: 'Nhân công', defaultValue: '8 công nhân' },
          { name: 'material_usage_summary', label: 'Vật tư sử dụng', defaultValue: 'Theo phiếu xuất kho' },
          { name: 'issues_summary', label: 'Vấn đề', defaultValue: 'Không có' },
          { name: 'progress_percent', label: 'Tiến độ %', type: 'number', defaultValue: 50 },
        ]} onSubmit={(values) => createReport.mutateAsync(values)} />
      </FormDrawer>
      <FormDrawer open={open === 'issue'} title="Tạo issue" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Issue phát sinh' },
          { name: 'description', label: 'Mô tả', type: 'textarea' },
          { name: 'severity', label: 'Mức độ', type: 'select', defaultValue: 'medium', options: ['low', 'medium', 'high', 'critical'].map((x) => ({ label: x, value: x })) },
          { name: 'requires_rework', label: 'Cần rework', type: 'checkbox' },
          { name: 'requires_replanning', label: 'Cần replanning', type: 'checkbox' },
        ]} onSubmit={(values) => createIssue.mutateAsync(values)} />
      </FormDrawer>
    </>
  );
}

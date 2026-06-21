import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderOpen, Plus } from 'lucide-react';
import { Link } from 'react-router-dom';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { SimpleForm } from '../_shared/SimpleForm';
import type { Project } from '../../lib/types/domain';

export function ProjectsPage() {
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: Project } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Project[]>('/projects') });
  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<Record<string, any>[]>('/pipeline-templates') });
  const createProject = useMutation({
    mutationFn: (values: Record<string, any>) => post<Project>('/projects', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setOpen(false);
      toast.success('Đã tạo công trình');
    },
    onError: (error) => toast.error('Không tạo được công trình', error),
  });
  const updateProject = useMutation({
    mutationFn: ({ projectId, values }: { projectId: string; values: Record<string, any> }) => put<Project>(`/projects/${projectId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setDialog(null);
      toast.success('Đã cập nhật công trình');
    },
    onError: (error) => toast.error('Không cập nhật được công trình', error),
  });

  const projectFields = (record?: Project) => [
    { name: 'name', label: 'Tên công trình', defaultValue: record?.name ?? 'Công trình mới', required: true },
    { name: 'code', label: 'Mã công trình', defaultValue: record?.code ?? `PRJ-${Date.now().toString().slice(-5)}`, required: true },
    { name: 'client_name', label: 'Khách hàng / Chủ đầu tư', defaultValue: record?.client_name ?? 'Khách hàng mới', required: true },
    { name: 'address', label: 'Địa chỉ', defaultValue: record?.address ?? '' },
    { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date' as const, defaultValue: record?.planned_start_date?.slice?.(0, 10) ?? '' },
    { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date' as const, defaultValue: record?.planned_end_date?.slice?.(0, 10) ?? '' },
    { name: 'estimated_budget', label: 'Ngân sách dự kiến', type: 'number' as const, defaultValue: record?.estimated_budget ?? 300000000, min: 0 },
    { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number' as const, defaultValue: record?.contract_amount ?? 450000000, min: 0 },
    {
      name: 'pipeline_template_id',
      label: 'Pipeline template',
      type: 'select' as const,
      defaultValue: record?.pipeline_template_id ?? '',
      options: (templates.data ?? []).map((template) => ({ label: `${template.name} v${template.version}`, value: template.id })),
    },
  ];

  return (
    <>
      <PageHeader
        title="Công trình"
        description="Quản lý danh sách công trình, trạng thái pipeline, PM phụ trách, ngân sách và các điểm đang bị block."
        actions={
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" />
            Tạo công trình
          </button>
        }
      />
      <DataTable
        rows={projects.data ?? []}
        loading={projects.isLoading}
        error={projects.error}
        onRetry={() => projects.refetch()}
        searchPlaceholder="Tìm công trình, mã, chủ đầu tư"
        pageSize={10}
        defaultSort={{ key: 'name', direction: 'asc' }}
        columns={[
          {
            key: 'name',
            header: 'Công trình',
            sortable: true,
            render: (row) => (
              <button className="font-semibold text-blue-700 hover:underline" onClick={() => setDialog({ mode: 'view', record: row })}>
                {row.name}
              </button>
            ),
          },
          { key: 'code', header: 'Mã', sortable: true },
          { key: 'client_name', header: 'Chủ đầu tư', sortable: true },
          { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
          { key: 'estimated_budget', header: 'Ngân sách', align: 'right', sortable: true, sortValue: (row) => Number(row.estimated_budget ?? 0), render: (row) => <CurrencyText value={row.estimated_budget} /> },
          { key: 'progress_percent', header: 'Tiến độ', align: 'right', sortable: true, sortValue: (row) => Number(row.progress_percent ?? 0), render: (row) => `${row.progress_percent ?? 0}%` },
          {
            key: 'actions',
            header: 'Action',
            align: 'right',
            render: (row) => (
              <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={() => setDialog({ mode: 'edit', record: row })}>
                <Link title="Mở module công trình" aria-label="Mở module công trình" className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100" to={`/projects/${row.id}`}>
                  <FolderOpen className="h-4 w-4" />
                </Link>
              </RowActions>
            ),
          },
        ]}
      />

      <FormDrawer open={open} title="Tạo công trình mới" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Tạo công trình"
          fields={projectFields()}
          onSubmit={(values) => createProject.mutateAsync(values)}
        />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết công trình"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'name', label: 'Tên công trình' },
          { key: 'code', label: 'Mã' },
          { key: 'client_name', label: 'Chủ đầu tư' },
          { key: 'address', label: 'Địa chỉ' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'estimated_budget', label: 'Ngân sách', render: (row) => <CurrencyText value={row.estimated_budget} /> },
          { key: 'contract_amount', label: 'Hợp đồng', render: (row) => <CurrencyText value={row.contract_amount} /> },
          { key: 'progress_percent', label: 'Tiến độ', render: (row) => `${row.progress_percent ?? 0}%` },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa công trình" onClose={() => setDialog(null)}>
        {dialog?.record ? (
          <SimpleForm
            submitLabel="Lưu thay đổi"
            fields={projectFields(dialog.record)}
            onSubmit={(values) => updateProject.mutateAsync({
              projectId: dialog.record.id,
              values: { ...values, pipeline_template_id: values.pipeline_template_id || null },
            })}
          />
        ) : null}
      </FormDrawer>
    </>
  );
}

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { CurrencyText } from '../../components/CurrencyText';
import { SimpleForm } from '../_shared/SimpleForm';
import type { Project } from '../../lib/types/domain';

export function ProjectsPage() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Project[]>('/projects') });
  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<Record<string, any>[]>('/pipeline-templates') });
  const createProject = useMutation({
    mutationFn: (values: Record<string, any>) => post<Project>('/projects', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setOpen(false);
    },
  });

  if (projects.isLoading) return <LoadingSpinner />;

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
        columns={[
          {
            key: 'name',
            header: 'Công trình',
            render: (row) => (
              <Link to={`/projects/${row.id}`} className="font-semibold text-blue-700 hover:underline">
                {row.name}
              </Link>
            ),
          },
          { key: 'code', header: 'Mã' },
          { key: 'client_name', header: 'Chủ đầu tư' },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'estimated_budget', header: 'Ngân sách', render: (row) => <CurrencyText value={row.estimated_budget} /> },
          { key: 'progress_percent', header: 'Tiến độ', render: (row) => `${row.progress_percent ?? 0}%` },
        ]}
      />

      <FormDrawer open={open} title="Tạo công trình mới" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Tạo công trình"
          fields={[
            { name: 'name', label: 'Tên công trình', defaultValue: 'Công trình mới' },
            { name: 'code', label: 'Mã công trình', defaultValue: `PRJ-${Date.now().toString().slice(-5)}` },
            { name: 'client_name', label: 'Khách hàng / Chủ đầu tư', defaultValue: 'Khách hàng mới' },
            { name: 'address', label: 'Địa chỉ' },
            { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date' },
            { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date' },
            { name: 'estimated_budget', label: 'Ngân sách dự kiến', type: 'number', defaultValue: 300000000 },
            { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number', defaultValue: 450000000 },
            {
              name: 'pipeline_template_id',
              label: 'Pipeline template',
              type: 'select',
              options: (templates.data ?? []).map((template) => ({ label: `${template.name} v${template.version}`, value: template.id })),
            },
          ]}
          onSubmit={(values) => createProject.mutateAsync(values)}
        />
      </FormDrawer>
    </>
  );
}

import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';

export function PermissionsPage() {
  const permissions = useQuery({ queryKey: ['permissions'], queryFn: () => get<Record<string, any>[]>('/permissions') });
  return (
    <>
      <PageHeader title="Phân quyền" description="Permission matrix cho project, pipeline, BOQ, mua sắm, kho, thi công, nghiệm thu, kế toán và hệ thống." />
      <DataTable
        rows={permissions.data ?? []}
        loading={permissions.isLoading}
        error={permissions.error}
        onRetry={() => permissions.refetch()}
        searchPlaceholder="Tìm permission..."
        pageSize={20}
        defaultSort={{ key: 'module' }}
        columns={[
          { key: 'module', header: 'Module', sortable: true },
          { key: 'key', header: 'Permission key', sortable: true, render: (row) => <code className="rounded bg-slate-100 px-2 py-1 text-xs">{row.key}</code> },
          { key: 'name', header: 'Tên quyền', sortable: true },
          { key: 'description', header: 'Mô tả' },
        ]}
      />
    </>
  );
}

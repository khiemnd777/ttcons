import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';

export function PermissionsPage() {
  const permissions = useQuery({ queryKey: ['permissions'], queryFn: () => get<Record<string, any>[]>('/permissions') });
  return (
    <>
      <PageHeader title="Phân quyền" description="Permission matrix cho project, pipeline, BOQ, mua sắm, kho, thi công, nghiệm thu, kế toán và hệ thống." />
      <DataTable rows={permissions.data ?? []} columns={[
        { key: 'module', header: 'Module' },
        { key: 'key', header: 'Permission key', render: (row) => <code className="rounded bg-slate-100 px-2 py-1 text-xs">{row.key}</code> },
        { key: 'name', header: 'Tên quyền' },
        { key: 'description', header: 'Mô tả' },
      ]} />
    </>
  );
}

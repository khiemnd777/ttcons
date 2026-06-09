import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { SimpleForm } from '../_shared/SimpleForm';

export function RolesPage() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const roles = useQuery({ queryKey: ['roles'], queryFn: () => get<Record<string, any>[]>('/roles') });
  const create = useMutation({ mutationFn: (values: Record<string, any>) => post('/roles', values), onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['roles'] }); setOpen(false); } });
  return (
    <>
      <PageHeader
        title="Vai trò"
        description="Role hệ thống và role theo công trình; pipeline node gán theo role thay vì gán cứng người."
        actions={<button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Role</button>}
      />
      <DataTable rows={roles.data ?? []} columns={[
        { key: 'display_name', header: 'Vai trò', render: (row) => <span className="font-semibold text-ink">{row.display_name}</span> },
        { key: 'name', header: 'Key' },
        { key: 'description', header: 'Mô tả' },
        { key: 'system_role', header: 'System', render: (row) => row.system_role ? 'Có' : 'Không' },
      ]} />
      <FormDrawer open={open} title="Tạo role" onClose={() => setOpen(false)}>
        <SimpleForm fields={[
          { name: 'name', label: 'Role key', defaultValue: `role_${Date.now().toString().slice(-4)}` },
          { name: 'display_name', label: 'Tên hiển thị', defaultValue: 'Role mới' },
          { name: 'description', label: 'Mô tả', type: 'textarea' },
        ]} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
    </>
  );
}

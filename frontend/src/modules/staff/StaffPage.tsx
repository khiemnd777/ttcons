import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { SimpleForm } from '../_shared/SimpleForm';

export function StaffPage() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const staff = useQuery({ queryKey: ['staff'], queryFn: () => get<Record<string, any>[]>('/staff') });
  const create = useMutation({ mutationFn: (values: Record<string, any>) => post('/staff', values), onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['staff'] }); setOpen(false); } });

  return (
    <>
      <PageHeader
        title="Nhân sự"
        description="Quản lý nhân sự, phòng ban, chức vụ, trạng thái hoạt động và phân công vào công trình theo role."
        actions={<button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Nhân sự</button>}
      />
      <DataTable rows={staff.data ?? []} columns={[
        { key: 'full_name', header: 'Họ tên', render: (row) => <span className="font-semibold text-ink">{row.full_name}</span> },
        { key: 'email', header: 'Email' },
        { key: 'phone', header: 'Điện thoại' },
        { key: 'department', header: 'Phòng ban' },
        { key: 'position', header: 'Chức vụ' },
        { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
      ]} />
      <FormDrawer open={open} title="Tạo nhân sự" onClose={() => setOpen(false)}>
        <SimpleForm fields={[
          { name: 'full_name', label: 'Họ tên', defaultValue: 'Nhân sự mới' },
          { name: 'email', label: 'Email', defaultValue: `staff${Date.now().toString().slice(-4)}@consflow.local` },
          { name: 'phone', label: 'Điện thoại' },
          { name: 'department', label: 'Phòng ban', defaultValue: 'Dự án' },
          { name: 'position', label: 'Chức vụ', defaultValue: 'Nhân viên' },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'active', options: ['active', 'inactive'].map((x) => ({ label: x, value: x })) },
        ]} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
    </>
  );
}

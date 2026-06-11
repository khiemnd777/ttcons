import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { SimpleForm } from '../_shared/SimpleForm';

export function RolesPage() {
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const roles = useQuery({ queryKey: ['roles'], queryFn: () => get<Record<string, any>[]>('/roles') });
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post('/roles', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setOpen(false);
      toast.success('Đã tạo role');
    },
    onError: (error) => toast.error('Không tạo được role', error),
  });
  const update = useMutation({
    mutationFn: ({ roleId, values }: { roleId: string; values: Record<string, any> }) => put(`/roles/${roleId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setDialog(null);
      toast.success('Đã cập nhật role');
    },
    onError: (error) => toast.error('Không cập nhật được role', error),
  });

  const roleFields = (record?: Record<string, any>) => [
    { name: 'name', label: 'Role key', defaultValue: record?.name ?? `role_${Date.now().toString().slice(-4)}`, required: true },
    { name: 'display_name', label: 'Tên hiển thị', defaultValue: record?.display_name ?? 'Role mới', required: true },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '' },
  ];

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
        { key: 'actions', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={() => setDialog({ mode: 'edit', record: row })} /> },
      ]} />
      <FormDrawer open={open} title="Tạo role" onClose={() => setOpen(false)}>
        <SimpleForm fields={roleFields()} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết role"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'display_name', label: 'Tên hiển thị' },
          { key: 'name', label: 'Key' },
          { key: 'description', label: 'Mô tả' },
          { key: 'system_role', label: 'System role' },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa role" onClose={() => setDialog(null)}>
        {dialog?.record ? <SimpleForm submitLabel="Lưu thay đổi" fields={roleFields(dialog.record)} onSubmit={(values) => update.mutateAsync({ roleId: dialog.record.id, values })} /> : null}
      </FormDrawer>
    </>
  );
}

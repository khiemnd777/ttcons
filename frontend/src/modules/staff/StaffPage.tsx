import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { SimpleForm } from '../_shared/SimpleForm';
import { enumOptions } from '../../lib/utils/status';

export function StaffPage() {
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const staff = useQuery({ queryKey: ['staff'], queryFn: () => get<Record<string, any>[]>('/staff') });
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post('/staff', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['staff'] });
      setOpen(false);
      toast.success('Đã tạo nhân sự');
    },
    onError: (error) => toast.error('Không tạo được nhân sự', error),
  });
  const update = useMutation({
    mutationFn: ({ staffId, values }: { staffId: string; values: Record<string, any> }) => put(`/staff/${staffId}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['staff'] });
      setDialog(null);
      toast.success('Đã cập nhật nhân sự');
    },
    onError: (error) => toast.error('Không cập nhật được nhân sự', error),
  });

  const staffFields = (record?: Record<string, any>) => [
    { name: 'full_name', label: 'Họ tên', defaultValue: record?.full_name ?? 'Nhân sự mới', required: true },
    { name: 'email', label: 'Email', defaultValue: record?.email ?? `staff${Date.now().toString().slice(-4)}@consflow.local`, required: true },
    { name: 'phone', label: 'Điện thoại', defaultValue: record?.phone ?? '' },
    { name: 'department', label: 'Phòng ban', defaultValue: record?.department ?? 'Dự án', required: true },
    { name: 'position', label: 'Chức vụ', defaultValue: record?.position ?? 'Nhân viên', required: true },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'active', required: true, options: enumOptions('status', ['active', 'inactive']) },
  ];

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
        { key: 'actions', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={() => setDialog({ mode: 'edit', record: row })} /> },
      ]} />
      <FormDrawer open={open} title="Tạo nhân sự" onClose={() => setOpen(false)}>
        <SimpleForm fields={staffFields()} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết nhân sự"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'full_name', label: 'Họ tên' },
          { key: 'email', label: 'Email' },
          { key: 'phone', label: 'Điện thoại' },
          { key: 'department', label: 'Phòng ban' },
          { key: 'position', label: 'Chức vụ' },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />
      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa nhân sự" onClose={() => setDialog(null)}>
        {dialog?.record ? <SimpleForm submitLabel="Lưu thay đổi" fields={staffFields(dialog.record)} onSubmit={(values) => update.mutateAsync({ staffId: dialog.record.id, values })} /> : null}
      </FormDrawer>
    </>
  );
}

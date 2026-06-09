import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDownToLine, ArrowUpFromLine, Plus, RotateCcw } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { CurrencyText } from '../../components/CurrencyText';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';

export function WarehousePage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState<'material' | 'movement' | null>(null);
  const queryClient = useQueryClient();
  const materials = useQuery({ queryKey: ['materials'], queryFn: () => get<Record<string, any>[]>('/materials') });
  const warehouses = useQuery({ queryKey: ['warehouses'], queryFn: () => get<Record<string, any>[]>('/warehouses') });
  const stock = useQuery({ queryKey: ['stock'], queryFn: () => get<Record<string, any>[]>('/stock') });
  const balances = useQuery({ queryKey: ['material-balances', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/material-balances`), enabled: Boolean(projectId) });
  const createMaterial = useMutation({ mutationFn: (values: Record<string, any>) => post('/materials', values), onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['materials'] }); setOpen(null); } });
  const movement = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/stock/${values.movement_type}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stock'] });
      queryClient.invalidateQueries({ queryKey: ['material-balances', projectId] });
      setOpen(null);
    },
  });

  return (
    <>
      <PageHeader
        title="Kho vật tư"
        description="Danh mục vật tư, nhập xuất tồn, tồn tại công trình và xử lý vật tư dư."
        actions={
          <>
            {!scoped ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setOpen('movement')}><ArrowDownToLine className="h-4 w-4" />Nhập / xuất</button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('material')}><Plus className="h-4 w-4" />Vật tư</button>
          </>
        }
      />
      <div className="grid gap-4 xl:grid-cols-2">
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Tồn kho tổng</h2>
          <DataTable rows={stock.data ?? []} columns={[
            { key: 'code', header: 'Mã' },
            { key: 'name', header: 'Vật tư', render: (row) => <span className="font-semibold text-ink">{row.name}</span> },
            { key: 'category', header: 'Nhóm' },
            { key: 'stock_quantity', header: 'Tồn' },
            { key: 'unit', header: 'ĐVT' },
          ]} />
        </section>
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Tồn tại công trình / vật tư dư</h2>
          <DataTable rows={balances.data ?? []} columns={[
            { key: 'material_name', header: 'Vật tư' },
            { key: 'planned_quantity', header: 'Kế hoạch' },
            { key: 'issued_quantity', header: 'Đã xuất' },
            { key: 'used_quantity', header: 'Đã dùng' },
            { key: 'surplus_quantity', header: 'Dư' },
            { key: 'surplus_status', header: 'Xử lý' },
          ]} empty="Chọn công trình để xem tồn tại site" />
        </section>
      </div>
      <FormDrawer open={open === 'material'} title="Thêm vật tư" onClose={() => setOpen(null)}>
        <SimpleForm
          submitLabel="Thêm vật tư"
          fields={[
            { name: 'code', label: 'Mã vật tư', defaultValue: `MAT-${Date.now().toString().slice(-4)}` },
            { name: 'name', label: 'Tên vật tư', defaultValue: 'Vật tư mới' },
            { name: 'unit', label: 'Đơn vị', defaultValue: 'cái' },
            { name: 'category', label: 'Nhóm', defaultValue: 'Phụ kiện' },
            { name: 'default_supplier', label: 'Nhà cung cấp' },
            { name: 'standard_price', label: 'Giá chuẩn', type: 'number', defaultValue: 100000 },
          ]}
          onSubmit={(values) => createMaterial.mutateAsync(values)}
        />
      </FormDrawer>
      <FormDrawer open={open === 'movement'} title="Tạo nhập / xuất kho" onClose={() => setOpen(null)}>
        <SimpleForm
          submitLabel="Ghi nhận"
          fields={[
            { name: 'movement_type', label: 'Loại', type: 'select', defaultValue: 'receive', options: [
              { label: 'Nhập kho', value: 'receive' },
              { label: 'Xuất công trình', value: 'issue' },
              { label: 'Lưu kho vật tư dư', value: 'return-to-warehouse' },
              { label: 'Trả thầu phụ', value: 'return-to-subcontractor' },
            ] },
            { name: 'project_id', label: 'Công trình', defaultValue: projectId },
            { name: 'warehouse_id', label: 'Kho', type: 'select', options: (warehouses.data ?? []).map((w) => ({ label: w.name, value: w.id })) },
            { name: 'material_id', label: 'Vật tư', type: 'select', options: (materials.data ?? []).map((m) => ({ label: m.name, value: m.id })) },
            { name: 'quantity', label: 'Số lượng', type: 'number', defaultValue: 1 },
            { name: 'unit', label: 'Đơn vị', defaultValue: 'cái' },
            { name: 'reason', label: 'Lý do', defaultValue: 'Ghi nhận nhập/xuất vật tư' },
          ]}
          onSubmit={(values) => movement.mutateAsync(values)}
        />
      </FormDrawer>
    </>
  );
}

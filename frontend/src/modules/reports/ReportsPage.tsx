import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { CurrencyText } from '../../components/CurrencyText';

export function ReportsPage() {
  const projects = useQuery({ queryKey: ['report', 'projects'], queryFn: () => get<Record<string, any>[]>('/reports/projects') });
  const warehouse = useQuery({ queryKey: ['report', 'warehouse'], queryFn: () => get<Record<string, any>[]>('/reports/warehouse') });
  const procurement = useQuery({ queryKey: ['report', 'procurement'], queryFn: () => get<Record<string, any>[]>('/reports/procurement') });
  const accounting = useQuery({ queryKey: ['report', 'accounting'], queryFn: () => get<Record<string, any>[]>('/reports/accounting') });
  const staff = useQuery({ queryKey: ['report', 'staff'], queryFn: () => get<Record<string, any>[]>('/reports/staff') });

  return (
    <>
      <PageHeader title="Báo cáo" description="Báo cáo công trình, vật tư/kho, mua sắm, kế toán/công nợ và nhân sự." />
      <div className="grid gap-5">
        <section>
          <h2 className="mb-3 text-base font-semibold text-ink">Báo cáo công trình</h2>
          <DataTable rows={projects.data ?? []} columns={[
            { key: 'name', header: 'Công trình' },
            { key: 'status', header: 'Trạng thái' },
            { key: 'progress_percent', header: 'Tiến độ' },
            { key: 'open_issues', header: 'Issue mở' },
            { key: 'pending_approvals', header: 'Chờ duyệt' },
          ]} />
        </section>
        <section className="grid gap-4 xl:grid-cols-2">
          <div>
            <h2 className="mb-3 text-base font-semibold text-ink">Vật tư / Kho</h2>
            <DataTable rows={warehouse.data ?? []} columns={[
              { key: 'name', header: 'Vật tư' },
              { key: 'category', header: 'Nhóm' },
              { key: 'stock_quantity', header: 'Tồn' },
              { key: 'surplus_quantity', header: 'Dư' },
            ]} />
          </div>
          <div>
            <h2 className="mb-3 text-base font-semibold text-ink">Mua sắm</h2>
            <DataTable rows={procurement.data ?? []} columns={[
              { key: 'status', header: 'Trạng thái' },
              { key: 'budget_status', header: 'Budget' },
              { key: 'total', header: 'Số request' },
              { key: 'amount', header: 'Giá trị', render: (row) => <CurrencyText value={row.amount} /> },
            ]} />
          </div>
        </section>
        <section className="grid gap-4 xl:grid-cols-2">
          <div>
            <h2 className="mb-3 text-base font-semibold text-ink">Kế toán / Công nợ</h2>
            <DataTable rows={accounting.data ?? []} columns={[
              { key: 'name', header: 'Công trình' },
              { key: 'payable_remaining', header: 'Phải trả', render: (row) => <CurrencyText value={row.payable_remaining} /> },
              { key: 'receivable_remaining', header: 'Phải thu', render: (row) => <CurrencyText value={row.receivable_remaining} /> },
              { key: 'used_budget', header: 'Đã dùng', render: (row) => <CurrencyText value={row.used_budget} /> },
            ]} />
          </div>
          <div>
            <h2 className="mb-3 text-base font-semibold text-ink">Nhân sự</h2>
            <DataTable rows={staff.data ?? []} columns={[
              { key: 'full_name', header: 'Nhân sự' },
              { key: 'department', header: 'Phòng ban' },
              { key: 'position', header: 'Chức vụ' },
              { key: 'project_count', header: 'Công trình' },
            ]} />
          </div>
        </section>
      </div>
    </>
  );
}

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { NavLink } from 'react-router-dom';
import { DataTable, type DataTableRowKey } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { PageHeader } from '../../components/PageHeader';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import type { Partner } from '../../lib/types/domain';
import { SimpleForm, type FieldDef } from '../_shared/SimpleForm';
import { outputPartnerTypeOptions, partnerTypeLabel, partnerTypeOptions, partnerTypesText } from './partnerUtils';
import { useAuth } from '../../lib/auth/AuthContext';
import { BulkSelectionBar } from '../../components/BulkSelectionBar';
import { BulkDeleteDialog, type BulkDeletionResult } from '../../components/BulkDeleteDialog';

type PartnerDialog = { mode: 'view' | 'edit'; record: Partner } | { mode: 'create'; record?: undefined } | null;
type PartnerCatalogGroup = 'input' | 'output';

const partnerCatalogConfig: Record<PartnerCatalogGroup, {
  actionLabel: string;
  createDialogTitle: string;
  defaultType: 'client' | 'supplier';
  metricLabel: string;
  metricValue: string;
  empty: string;
}> = {
  input: {
    actionLabel: 'Thêm đầu vào',
    createDialogTitle: 'Tạo đối tác đầu vào',
    defaultType: 'client',
    metricLabel: 'Nhóm',
    metricValue: 'Khách hàng / CĐT',
    empty: 'Chưa có đối tác đầu vào',
  },
  output: {
    actionLabel: 'Thêm đầu ra',
    createDialogTitle: 'Tạo đối tác đầu ra',
    defaultType: 'supplier',
    metricLabel: 'Loại lọc',
    metricValue: 'Tất cả đầu ra',
    empty: 'Chưa có đối tác đầu ra',
  },
};

const partnerCatalogTabs: Array<{ group: PartnerCatalogGroup; label: string; to: string }> = [
  { group: 'input', label: 'Đầu vào', to: '/core-data/input' },
  { group: 'output', label: 'Đầu ra', to: '/core-data/output' },
];

export function CoreDataPage({ group }: { group: PartnerCatalogGroup }) {
  const [status, setStatus] = useState<'active' | 'inactive' | 'all'>('active');
  const [type, setType] = useState('');
  const [dialog, setDialog] = useState<PartnerDialog>(null);
  const [selectedKeys, setSelectedKeys] = useState<Set<DataTableRowKey>>(new Set());
  const [deleteOpen, setDeleteOpen] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasPermission } = useAuth();
  const canDelete = hasPermission('catalog.manage') && status === 'active';
  const config = partnerCatalogConfig[group];
  const partners = useQuery({
    queryKey: ['partners', group, status, type],
    queryFn: () => get<Partner[]>(`/partners?status=${encodeURIComponent(status)}&group=${group}${type ? `&type=${encodeURIComponent(type)}` : ''}`),
  });

  const createPartner = useMutation({
    mutationFn: (values: Record<string, any>) => post<Partner>('/partners', values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['partners'] });
      setDialog(null);
      toast.success('Đã tạo đối tác');
    },
    onError: (error) => toast.error('Không tạo được đối tác', error),
  });

  const updatePartner = useMutation({
    mutationFn: ({ id, values }: { id: string; values: Record<string, any> }) => put<Partner>(`/partners/${id}`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['partners'] });
      setDialog(null);
      toast.success('Đã cập nhật đối tác');
    },
    onError: (error) => toast.error('Không cập nhật được đối tác', error),
  });

  const rows = partners.data ?? [];
  const activeCount = useMemo(() => rows.filter((partner) => partner.active !== false).length, [rows]);
  const selectedPartnerIDs = useMemo(() => Array.from(selectedKeys, String), [selectedKeys]);

  return (
    <>
      <PageHeader
        title="Đối tác"
        description="Quản lý khách hàng, nhà cung cấp, thầu phụ và đơn vị dịch vụ theo nhóm nghiệp vụ."
        actions={
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setDialog({ mode: 'create' })}>
            <Plus className="h-4 w-4" />
            {config.actionLabel}
          </button>
        }
      />

      <nav aria-label="Phân nhóm đối tác" className="mb-4 grid grid-cols-2 border-b border-line sm:flex">
        {partnerCatalogTabs.map((tab) => (
          <NavLink
            key={tab.group}
            to={tab.to}
            end
            className={({ isActive }) => `-mb-px inline-flex min-h-11 items-center justify-center border-b-2 px-5 py-2 text-sm font-semibold transition-colors ${
              isActive
                ? 'border-blue-600 text-blue-700'
                : 'border-transparent text-slate-500 hover:border-slate-300 hover:text-slate-800'
            }`}
          >
            {tab.label}
          </NavLink>
        ))}
      </nav>

      {canDelete ? <BulkSelectionBar count={selectedKeys.size} itemLabel="đối tác" actionLabel="Xóa đối tác đã chọn" onClear={() => setSelectedKeys(new Set())} onDelete={() => setDeleteOpen(true)} /> : null}

      <div className={`mb-4 grid gap-3 ${group === 'input' ? 'md:grid-cols-[1fr_auto]' : 'md:grid-cols-[1fr_auto_auto]'} md:items-end`}>
        <div className="grid gap-3 sm:grid-cols-3">
          <Metric label="Đang hiển thị" value={String(rows.length)} />
          <Metric label="Active" value={String(activeCount)} />
          <Metric label={config.metricLabel} value={type ? partnerTypeLabel(type) : config.metricValue} />
        </div>
        {group === 'output' ? (
          <label className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
            Loại
            <select className={filterClassName} value={type} onChange={(event) => { setType(event.target.value); setSelectedKeys(new Set()); }}>
              <option value="">Tất cả đầu ra</option>
              {outputPartnerTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
        ) : null}
        <label className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
          Trạng thái
          <select className={filterClassName} value={status} onChange={(event) => { setStatus(event.target.value as typeof status); setSelectedKeys(new Set()); }}>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
            <option value="all">Tất cả</option>
          </select>
        </label>
      </div>

      <DataTable
        rows={rows}
        loading={partners.isLoading}
        error={partners.error}
        onRetry={() => void partners.refetch()}
        empty={config.empty}
        searchPlaceholder="Tìm tên, mã, MST, liên hệ..."
        pageSize={10}
        defaultSort={{ key: 'name', direction: 'asc' }}
        getRowKey={(row) => row.id}
        selection={canDelete ? { selectedKeys, onSelectedKeysChange: setSelectedKeys, ariaLabel: `Chọn tất cả đối tác ${group === 'input' ? 'Đầu vào' : 'Đầu ra'} trên trang hiện tại` } : undefined}
        columns={[
          { key: 'partner_code', header: 'Mã', sortable: true },
          {
            key: 'name',
            header: 'Tên đối tác',
            sortable: true,
            render: (row) => (
              <button className="font-semibold text-blue-700 hover:underline" onClick={() => setDialog({ mode: 'view', record: row })}>
                {row.name}
              </button>
            ),
          },
          { key: 'partner_types', header: 'Loại', filterValue: (row) => partnerTypesText(row), render: (row) => <span>{partnerTypesText(row)}</span> },
          { key: 'tax_code', header: 'MST', sortable: true, render: (row) => row.tax_code || <span className="text-slate-400">-</span> },
          { key: 'contact_name', header: 'Liên hệ', sortable: true, render: (row) => row.contact_name || <span className="text-slate-400">-</span> },
          {
            key: 'active',
            header: 'Status',
            sortable: true,
            render: (row) => (
              <span className={row.active === false ? 'rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600' : 'rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700'}>
                {row.active === false ? 'Inactive' : 'Active'}
              </span>
            ),
          },
          { key: 'actions', header: 'Action', align: 'right', render: (row) => <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={() => setDialog({ mode: 'edit', record: row })} /> },
        ]}
      />

      <BulkDeleteDialog
        kind="partner"
        open={deleteOpen}
        ids={selectedPartnerIDs}
        onClose={() => setDeleteOpen(false)}
        onSuccess={async (result: BulkDeletionResult) => {
          setDeleteOpen(false);
          setSelectedKeys(new Set());
          await queryClient.invalidateQueries({ queryKey: ['partners'] });
          toast.success({ title: `Đã xóa ${result.deleted_count} đối tác`, description: 'Đối tác đã chuyển sang Inactive; dữ liệu lịch sử vẫn được giữ nguyên.' });
        }}
        onError={(title, error) => toast.error(title, error)}
      />

      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết đối tác"
        record={dialog?.mode === 'view' ? dialog.record : null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'partner_code', label: 'Mã' },
          { key: 'name', label: 'Tên' },
          { key: 'partner_types', label: 'Loại', render: (row) => partnerTypesText(row as Partner) },
          { key: 'tax_code', label: 'MST' },
          { key: 'contact_name', label: 'Liên hệ' },
          { key: 'phone', label: 'Điện thoại' },
          { key: 'email', label: 'Email' },
          { key: 'address', label: 'Địa chỉ' },
          { key: 'payment_terms_days', label: 'Điều khoản TT' },
          { key: 'active', label: 'Trạng thái', render: (row) => (row.active === false ? 'Inactive' : 'Active') },
        ]}
      />

      <Dialog
        open={dialog?.mode === 'create' || dialog?.mode === 'edit'}
        title={dialog?.mode === 'edit' ? 'Sửa đối tác' : config.createDialogTitle}
        onClose={() => setDialog(null)}
        size="lg"
      >
        <SimpleForm
          layout="two-column"
          submitLabel={dialog?.mode === 'edit' ? 'Lưu đối tác' : 'Tạo đối tác'}
          fields={partnerFields(dialog?.mode === 'edit' ? dialog.record : undefined, config.defaultType)}
          onSubmit={(values) => {
            if (dialog?.mode === 'edit') {
              return updatePartner.mutateAsync({ id: dialog.record.id, values });
            }
            return createPartner.mutateAsync(values);
          }}
        />
      </Dialog>
    </>
  );
}

function partnerFields(record: Partner | undefined, defaultType: 'client' | 'supplier'): FieldDef[] {
  return [
    { name: 'partner_code', label: 'Mã đối tác', defaultValue: record?.partner_code ?? `PT-${Date.now().toString().slice(-5)}`, required: true },
    { name: 'name', label: 'Tên đối tác', defaultValue: record?.name ?? '', required: true },
    { name: 'partner_types', label: 'Loại', type: 'multi-select', defaultValue: record?.partner_types ?? [defaultType], required: true, options: partnerTypeOptions, fullWidth: true },
    { name: 'tax_code', label: 'MST', defaultValue: record?.tax_code ?? '' },
    { name: 'contact_name', label: 'Người liên hệ', defaultValue: record?.contact_name ?? '' },
    { name: 'phone', label: 'Điện thoại', defaultValue: record?.phone ?? '' },
    { name: 'email', label: 'Email', defaultValue: record?.email ?? '' },
    { name: 'address', label: 'Địa chỉ', type: 'textarea', defaultValue: record?.address ?? '', fullWidth: true },
    { name: 'notes', label: 'Ghi chú', type: 'textarea', defaultValue: record?.notes ?? '', fullWidth: true },
    { name: 'active', label: 'Active', type: 'checkbox', defaultValue: record?.active ?? true, helperText: 'Inactive partner sẽ không xuất hiện trong dropdown nghiệp vụ.' },
  ];
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1 text-base font-semibold text-ink">{value}</p>
    </div>
  );
}

const filterClassName = 'mt-1 h-10 min-w-44 rounded-md border border-line bg-white px-3 text-sm font-medium normal-case tracking-normal text-slate-700 outline-none focus:ring-2 focus:ring-blue-100';

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { ArrowUpRight } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type {
  AccountingReportRow,
  ProcurementReportRow,
  ProjectReportRow,
  ReportProjectOption,
  StaffReportRow,
  WarehouseReportRow,
} from '../../lib/types/domain';
import { useAuth } from '../../lib/auth/AuthContext';
import { ReportFilterActions, ReportProjectField } from './ReportFilters';

export type OperationalReportKind = 'projects' | 'warehouse' | 'procurement' | 'accounting' | 'staff';

type OperationalReportPanelProps = {
  kind: OperationalReportKind;
  projectOptions: ReportProjectOption[];
};

const reportCopy: Record<OperationalReportKind, { title: string; basis: string; empty: string; fileName: string }> = {
  projects: {
    title: 'Công trình & tiến độ',
    basis: 'Tiến độ công trình, số vấn đề đang mở và yêu cầu đang chờ duyệt trong phạm vi được phép xem.',
    empty: 'Chưa có công trình trong phạm vi lọc',
    fileName: 'bao-cao-cong-trinh',
  },
  warehouse: {
    title: 'Vật tư / Kho',
    basis: 'Tồn vật tư được tổng hợp độc lập với khối lượng dư để tránh nhân chéo số liệu.',
    empty: 'Chưa có dữ liệu vật tư trong phạm vi lọc',
    fileName: 'bao-cao-vat-tu-kho',
  },
  procurement: {
    title: 'Mua sắm',
    basis: 'Số lượng và giá trị yêu cầu mua sắm theo trạng thái xử lý và trạng thái ngân sách.',
    empty: 'Chưa có yêu cầu mua sắm trong phạm vi lọc',
    fileName: 'bao-cao-mua-sam',
  },
  accounting: {
    title: 'Công nợ & ngân sách',
    basis: 'Phải trả, phải thu và ngân sách đã sử dụng được cộng theo từng công trình, không khử các khoản bằng nhau.',
    empty: 'Chưa có dữ liệu công nợ trong phạm vi lọc',
    fileName: 'bao-cao-cong-no-ngan-sach',
  },
  staff: {
    title: 'Nhân sự theo công trình',
    basis: 'Chỉ hiển thị nhân sự thuộc các công trình người dùng được phép xem. Danh sách toàn công ty nằm ở module Nhân sự.',
    empty: 'Chưa có nhân sự trong các công trình thuộc phạm vi lọc',
    fileName: 'bao-cao-nhan-su-cong-trinh',
  },
};

export function OperationalReportPanel({ kind, projectOptions }: OperationalReportPanelProps) {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialProjectId = searchParams.get('project_id') ?? '';
  const [draftProjectId, setDraftProjectId] = useState(initialProjectId);
  const [projectId, setProjectId] = useState(initialProjectId);
  const [exporting, setExporting] = useState(false);
  const toast = useToast();
  const copy = reportCopy[kind];

  useEffect(() => {
    const nextProjectId = searchParams.get('project_id') ?? '';
    setDraftProjectId(nextProjectId);
    setProjectId(nextProjectId);
  }, [searchParams]);

  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (projectId) params.set('project_id', projectId);
    return params.toString();
  }, [projectId]);

  const report = useQuery({
    queryKey: ['operational-report', kind, projectId || 'all'],
    queryFn: () => get<Record<string, unknown>[]>(`/reports/${kind}${queryString ? `?${queryString}` : ''}`),
  });

  function applyFilters() {
    const params = new URLSearchParams(searchParams);
    if (draftProjectId) params.set('project_id', draftProjectId);
    else params.delete('project_id');
    setProjectId(draftProjectId);
    setSearchParams(params, { replace: true });
  }

  function resetFilters() {
    const params = new URLSearchParams(searchParams);
    params.delete('project_id');
    setDraftProjectId('');
    setProjectId('');
    setSearchParams(params, { replace: true });
  }

  async function exportCSV() {
    setExporting(true);
    try {
      const blob = await downloadBlob(`/reports/${kind}/export${queryString ? `?${queryString}` : ''}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `${copy.fileName}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success(`Đã xuất CSV ${copy.title.toLocaleLowerCase('vi-VN')}`);
    } catch (error) {
      toast.error(`Không xuất được CSV ${copy.title.toLocaleLowerCase('vi-VN')}`, error);
    } finally {
      setExporting(false);
    }
  }

  const rows = report.data ?? [];

  return (
    <section className="space-y-4">
      <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div className="min-w-0">
            <h2 className="text-lg font-semibold text-ink">{copy.title}</h2>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">{copy.basis}</p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end">
            <ReportProjectField value={draftProjectId} options={projectOptions} onChange={setDraftProjectId} />
            <ReportFilterActions
              applying={report.isFetching}
              exporting={exporting || report.isLoading}
              onApply={applyFilters}
              onReset={resetFilters}
              onExport={() => void exportCSV()}
            />
          </div>
        </div>
      </div>

      {operationalTable(kind, rows, report.isLoading, report.error, () => void report.refetch(), copy.empty)}
    </section>
  );
}

function operationalTable(
  kind: OperationalReportKind,
  rows: Record<string, unknown>[],
  loading: boolean,
  error: unknown,
  onRetry: () => void,
  empty: string,
) {
  const common = { loading, error, onRetry, empty, pageSize: 10 };

  if (kind === 'projects') {
    return (
      <DataTable<ProjectReportRow>
        {...common}
        rows={rows as ProjectReportRow[]}
        searchPlaceholder="Tìm công trình, trạng thái..."
        defaultSort={{ key: 'name', direction: 'asc' }}
        columns={[
          { key: 'name', header: 'Công trình', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.name}</span> },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'progress_percent', header: 'Tiến độ', align: 'right', sortable: true, render: (row) => `${Number(row.progress_percent ?? 0)}%` },
          { key: 'open_issues', header: 'Vấn đề mở', align: 'right', sortable: true },
          { key: 'pending_approvals', header: 'Chờ duyệt', align: 'right', sortable: true },
          { key: 'action', header: '', align: 'right', render: (row) => <SourceLink to={`/projects/${row.project_id}`} label="Mở công trình" permission="project.view" /> },
        ]}
      />
    );
  }

  if (kind === 'warehouse') {
    return (
      <DataTable<WarehouseReportRow>
        {...common}
        rows={rows as WarehouseReportRow[]}
        searchPlaceholder="Tìm vật tư, nhóm vật tư..."
        defaultSort={{ key: 'name', direction: 'asc' }}
        columns={[
          { key: 'name', header: 'Vật tư', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.name}</span> },
          { key: 'category', header: 'Nhóm' },
          { key: 'stock_quantity', header: 'Tồn', align: 'right', sortable: true, render: (row) => quantityText(row.stock_quantity) },
          { key: 'surplus_quantity', header: 'Khối lượng dư', align: 'right', sortable: true, render: (row) => quantityText(row.surplus_quantity) },
          { key: 'action', header: '', align: 'right', render: () => <SourceLink to="/warehouse" label="Mở kho" permission="warehouse.view" /> },
        ]}
      />
    );
  }

  if (kind === 'procurement') {
    return (
      <DataTable<ProcurementReportRow>
        {...common}
        rows={rows as ProcurementReportRow[]}
        searchPlaceholder="Tìm trạng thái mua sắm, ngân sách..."
        defaultSort={{ key: 'amount', direction: 'desc' }}
        columns={[
          { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
          { key: 'budget_status', header: 'Ngân sách', render: (row) => <EnumChip group="budget_status" value={row.budget_status} /> },
          { key: 'total', header: 'Số yêu cầu', align: 'right', sortable: true },
          { key: 'amount', header: 'Giá trị', align: 'right', sortable: true, render: (row) => <CurrencyText value={row.amount} /> },
          { key: 'action', header: '', align: 'right', render: () => <SourceLink to="/procurement" label="Mở mua sắm" permission="procurement.view" /> },
        ]}
      />
    );
  }

  if (kind === 'accounting') {
    return (
      <DataTable<AccountingReportRow>
        {...common}
        rows={rows as AccountingReportRow[]}
        searchPlaceholder="Tìm công trình..."
        defaultSort={{ key: 'name', direction: 'asc' }}
        columns={[
          { key: 'name', header: 'Công trình', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.name}</span> },
          { key: 'payable_remaining', header: 'Còn phải trả', align: 'right', sortable: true, render: (row) => <CurrencyText value={row.payable_remaining} /> },
          { key: 'receivable_remaining', header: 'Còn phải thu', align: 'right', sortable: true, render: (row) => <CurrencyText value={row.receivable_remaining} /> },
          { key: 'used_budget', header: 'Ngân sách đã dùng', align: 'right', sortable: true, render: (row) => <CurrencyText value={row.used_budget} /> },
          { key: 'action', header: '', align: 'right', render: (row) => <SourceLink to={`/projects/${row.project_id}/accounting`} label="Mở kế toán" permission="accounting.view" /> },
        ]}
      />
    );
  }

  return (
    <DataTable<StaffReportRow>
      {...common}
      rows={rows as StaffReportRow[]}
      searchPlaceholder="Tìm nhân sự, phòng ban, chức vụ..."
      defaultSort={{ key: 'full_name', direction: 'asc' }}
      columns={[
        { key: 'full_name', header: 'Nhân sự', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.full_name}</span> },
        { key: 'department', header: 'Phòng ban', sortable: true },
        { key: 'position', header: 'Chức vụ' },
        { key: 'project_count', header: 'Số công trình', align: 'right', sortable: true },
        { key: 'action', header: '', align: 'right', render: () => <SourceLink to="/staff" label="Mở nhân sự" permission="staff.view" /> },
      ]}
    />
  );
}

function SourceLink({ to, label, permission }: { to: string; label: string; permission: string }) {
  const { hasPermission } = useAuth();
  if (!hasPermission(permission)) return null;
  return (
    <Link className="inline-flex items-center gap-1 whitespace-nowrap text-xs font-semibold text-blue-700 hover:text-blue-800" to={to}>
      {label}
      <ArrowUpRight className="h-3.5 w-3.5" />
    </Link>
  );
}

function quantityText(value?: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 }).format(Number(value ?? 0));
}

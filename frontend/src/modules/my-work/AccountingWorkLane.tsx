import { type FormEvent, useEffect, useState } from 'react';
import { CheckCircle2, ExternalLink, RefreshCw, Search, Undo2 } from 'lucide-react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable, type Column } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import type { AccountingWorkItem, AccountingWorkPage, AccountingWorkStatus } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import {
  AccountingHandoffActionDialog,
} from '../project-finance/AccountingHandoffActionDialog';
import { useAccountingWorkActions } from '../project-finance/accountingWorkActions';
import { AccountingHandoffDetailDialog } from '../project-finance/AccountingHandoffDetailDialog';
import { AccountingRequestQueue } from '../project-finance/AccountingRequestQueue';

const statusTabs: Array<{
  value: AccountingWorkStatus;
  label: string;
  countKey?: keyof AccountingWorkPage['summary'];
}> = [
  { value: 'handed_off', label: 'Chờ tiếp nhận', countKey: 'handed_off_count' },
  { value: 'returned', label: 'Đã trả bổ sung', countKey: 'returned_count' },
  { value: 'acknowledged', label: 'Đã tiếp nhận', countKey: 'acknowledged_count' },
  { value: 'all', label: 'Tất cả', countKey: 'total' },
];

export function AccountingWorkLane({
  rows,
  summary,
  total,
  loading,
  fetchingMore,
  error,
  hasMore,
  project,
  status,
  search,
  canAcknowledge,
  onFilterChange,
  onLoadMore,
  onRetry,
}: {
  rows: AccountingWorkItem[];
  summary?: AccountingWorkPage['summary'];
  total: number;
  loading: boolean;
  fetchingMore: boolean;
  error: unknown;
  hasMore: boolean;
  project: string;
  status: AccountingWorkStatus;
  search: string;
  canAcknowledge: boolean;
  onFilterChange: (changes: { project?: string | null; accounting_status?: string | null; search?: string | null }) => void;
  onLoadMore: () => void;
  onRetry: () => void;
}) {
  const [searchDraft, setSearchDraft] = useState(search);
  const { dialog, setDialog, mutation } = useAccountingWorkActions();

  useEffect(() => setSearchDraft(search), [search]);

  function submitSearch(event: FormEvent) {
    event.preventDefault();
    onFilterChange({ search: searchDraft.trim() || null });
  }

  function resetFilters() {
    setSearchDraft('');
    onFilterChange({ project: null, search: null });
  }

  const hasFilters = Boolean(project || search);
  const columns: Column<AccountingWorkItem>[] = [
    {
      key: 'actions',
      header: 'Thao tác',
      action: true,
      align: 'right',
      className: 'w-px whitespace-nowrap',
      render: (item) => <AccountingRowActions item={item} canAcknowledge={canAcknowledge} onAction={(action) => setDialog({ action, item })} />,
    },
    {
      key: 'source',
      header: 'Hồ sơ nguồn',
      className: 'min-w-48 max-w-80',
      render: (item) => <SourceCell item={item} />,
    },
    {
      key: 'project',
      header: 'Công trình',
      className: 'min-w-36 max-w-56',
      render: (item) => <ProjectCell item={item} />,
    },
    {
      key: 'direction',
      header: 'Nhóm hồ sơ',
      className: 'min-w-28',
      render: (item) => <DossierGroupCell item={item} />,
    },
    {
      key: 'reference_amount',
      header: 'Giá trị tham chiếu',
      align: 'right',
      className: 'min-w-36',
      render: (item) => <ReferenceAmountCell item={item} />,
    },
    {
      key: 'status',
      header: 'Bàn giao',
      className: 'min-w-36 max-w-60',
      render: (item) => <HandoffStatusCell item={item} />,
    },
  ];

  return (
    <section className="mt-4 space-y-4" aria-label="Nghiệp vụ Kế toán cần xử lý">
      <AccountingRequestQueue projectId={project}/>
      <div className="flex gap-2 overflow-x-auto" aria-label="Lọc trạng thái hồ sơ kế toán">
        {statusTabs.map((tab) => (
          <button
            key={tab.value}
            type="button"
            aria-pressed={status === tab.value}
            className={clsx(
              'shrink-0 rounded-md border px-3 py-2 text-sm font-semibold',
              status === tab.value ? 'border-blue-600 bg-blue-50 text-blue-700' : 'border-line bg-white text-slate-600 hover:bg-slate-50',
            )}
            onClick={() => onFilterChange({ accounting_status: tab.value === 'handed_off' ? null : tab.value })}
          >
            {tab.label}
            <span className="ml-2 rounded-full bg-white px-1.5 py-0.5 text-xs ring-1 ring-inset ring-slate-200">
              {summary && tab.countKey ? summary[tab.countKey] : '…'}
            </span>
          </button>
        ))}
      </div>

      <p className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-900">
        Ưu tiên tiếp nhận hồ sơ kế toán toàn công ty. Giá trị hiển thị là tham chiếu từ hồ sơ nguồn; nghiệp vụ kế toán thực hiện trên MISA.
      </p>

      <div className="relative z-10 grid gap-3 lg:grid-cols-[minmax(12rem,20rem)_minmax(0,1fr)_auto] lg:items-end">
        <AsyncProjectPicker
          value={project}
          onChange={(value) => onFilterChange({ project: value || null })}
          endpoint="/project-finance/projects"
          queryKey="my-accounting-work-project-options"
          emptyLabel="Tất cả công trình"
        />
        <form className="flex min-w-0 items-end gap-3" onSubmit={submitSearch}>
          <label className="min-w-0 flex-1 text-xs font-semibold text-slate-600">
            Tìm hồ sơ
            <span className="mt-1 flex h-10 items-center rounded-md border border-line bg-white px-3 focus-within:border-blue-500 focus-within:ring-1 focus-within:ring-blue-500">
              <Search className="h-4 w-4 shrink-0 text-slate-400" />
              <input
                className="h-full min-w-0 flex-1 border-0 bg-transparent pl-2 text-sm font-normal outline-none"
                value={searchDraft}
                onChange={(event) => setSearchDraft(event.target.value)}
                placeholder="Mã hồ sơ, đối tác, mã MISA..."
              />
            </span>
          </label>
          <button type="submit" className="h-10 shrink-0 rounded-md bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-700">Tìm</button>
        </form>
        <button
          type="button"
          className="h-10 rounded-md border border-line bg-white px-3 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          disabled={!hasFilters && !searchDraft}
          onClick={resetFilters}
        >
          Xóa lọc
        </button>
      </div>

      <div className="space-y-2">
        {!loading && !error ? <p className="text-right text-xs font-medium text-slate-500" role="status">Đã hiển thị {rows.length}/{total} hồ sơ</p> : null}
        {/* The parent owns server filtering and cursor paging; do not filter or sort a partial batch locally. */}
        <DataTable
          rows={rows}
          columns={columns}
          getRowKey={(item) => item.handoff_id}
          loading={loading}
          error={error}
          onRetry={onRetry}
          empty={!hasFilters && status === 'handed_off' ? 'Không có hồ sơ đang chờ Kế toán tiếp nhận.' : 'Không có hồ sơ phù hợp với bộ lọc.'}
          emptyAction={hasFilters ? <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={resetFilters}>Xóa lọc</button> : undefined}
          renderMobileCard={(item) => (
            <article className="space-y-3 rounded-lg border border-line bg-white p-4 text-sm">
              <SourceCell item={item} />
              <dl className="grid grid-cols-2 gap-3 text-sm">
                <div className="col-span-2 min-w-0"><dt className="mb-1 text-xs text-slate-500">Công trình</dt><dd><ProjectCell item={item} /></dd></div>
                <div className="min-w-0"><dt className="mb-1 text-xs text-slate-500">Nhóm hồ sơ</dt><dd><DossierGroupCell item={item} /></dd></div>
                <div className="min-w-0 text-right"><dt className="mb-1 text-xs text-slate-500">Giá trị tham chiếu</dt><dd><ReferenceAmountCell item={item} /></dd></div>
                <div className="col-span-2 min-w-0"><dt className="mb-1 text-xs text-slate-500">Bàn giao</dt><dd><HandoffStatusCell item={item} /></dd></div>
              </dl>
              <div className="border-t border-line pt-3"><AccountingRowActions item={item} canAcknowledge={canAcknowledge} onAction={(action) => setDialog({ action, item })} /></div>
            </article>
          )}
        />
        {!loading && !error && rows.length > 0 && hasMore ? (
          <button type="button" className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={fetchingMore} onClick={onLoadMore}>
            {fetchingMore ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}
            {fetchingMore ? 'Đang tải thêm' : 'Tải thêm'} · Đã hiển thị {rows.length}/{total}
          </button>
        ) : null}
      </div>

      <AccountingHandoffActionDialog
        dialog={dialog}
        pending={mutation.isPending}
        onClose={() => setDialog(null)}
        onSubmit={(values) => {
          if (!dialog || dialog.action === 'handoff') return;
          mutation.mutate({ action: dialog.action, item: dialog.item, ...values });
        }}
      />
    </section>
  );
}

function SourceCell({ item }: { item: AccountingWorkItem }) {
  const [detailOpen, setDetailOpen] = useState(false);
  return <div className="min-w-0 break-words">
    <button type="button" className="font-semibold text-blue-700 hover:underline" onClick={() => setDetailOpen(true)}>{item.source_code}</button>
    {detailOpen ? <AccountingHandoffDetailDialog key={item.handoff_id} target={item} onClose={() => setDetailOpen(false)} /> : null}
    <p className="font-semibold text-ink">{item.source_title}</p>
    <p className="text-xs text-slate-500">{item.counterparty_name || 'Chưa xác định đối tác'}</p>
  </div>;
}

function ProjectCell({ item }: { item: AccountingWorkItem }) {
  return <div className="min-w-0 break-words">
    <Link className="font-semibold text-slate-700 hover:text-blue-700 hover:underline" to={`/projects/${item.project_id}`}>{item.project_code}</Link>
    <p className="text-xs text-slate-500">{item.project_name}</p>
  </div>;
}

function DossierGroupCell({ item }: { item: AccountingWorkItem }) {
  return <div>
    <span className="font-semibold text-slate-700">{item.direction === 'payable' ? 'Hồ sơ mua' : 'Hồ sơ thu'}</span>
    <p className="text-xs text-slate-500">{item.source_type === 'procurement_request' ? 'Yêu cầu mua' : 'Mốc hợp đồng'}</p>
  </div>;
}

function ReferenceAmountCell({ item }: { item: AccountingWorkItem }) {
  return <div className="break-words tabular-nums">
    <CurrencyText value={item.reference_amount} />
    <p className="text-xs text-slate-400">{item.currency} · từ hồ sơ nguồn</p>
  </div>;
}

function HandoffStatusCell({ item }: { item: AccountingWorkItem }) {
  const statusDate = item.status === 'acknowledged' ? item.acknowledged_at : item.status === 'returned' ? item.returned_at : item.handed_off_at;
  return <div className="min-w-0 break-words">
    <StatusBadge status={item.status} />
    {statusDate ? <p className="mt-1 text-xs text-slate-500">{dateText(statusDate)}</p> : null}
    {item.return_reason ? <p className="mt-1 text-xs text-rose-700">{item.return_reason}</p> : null}
  </div>;
}

function AccountingRowActions({ item, canAcknowledge, onAction }: {
  item: AccountingWorkItem;
  canAcknowledge: boolean;
  onAction: (action: 'acknowledge' | 'return') => void;
}) {
  const buttonClass = 'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 py-2 text-xs font-semibold';
  if (item.status === 'handed_off' && canAcknowledge) return <div className="flex items-center gap-1 md:justify-end">
    <button type="button" className={`${buttonClass} bg-emerald-600 text-white hover:bg-emerald-700`} onClick={() => onAction('acknowledge')}>
      <CheckCircle2 className="h-3.5 w-3.5" />Tiếp nhận
    </button>
    <button type="button" className={`${buttonClass} border border-line bg-white text-slate-700 hover:bg-slate-50`} onClick={() => onAction('return')}>
      <Undo2 className="h-3.5 w-3.5" />Trả lại
    </button>
  </div>;
  return <Link className={`${buttonClass} border border-line bg-white text-slate-700 hover:bg-slate-50`} to={item.source_route}>
    <ExternalLink className="h-3.5 w-3.5" />Mở nguồn
  </Link>;
}

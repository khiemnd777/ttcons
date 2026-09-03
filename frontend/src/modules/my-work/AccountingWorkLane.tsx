import { type FormEvent, useEffect, useMemo, useState } from 'react';
import { CheckCircle2, ExternalLink, RefreshCw, Search, Undo2 } from 'lucide-react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import { CurrencyText } from '../../components/CurrencyText';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import type { AccountingWorkItem, AccountingWorkPage, AccountingWorkStatus } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import {
  AccountingHandoffActionDialog,
} from '../project-finance/AccountingHandoffActionDialog';
import { useAccountingWorkActions } from '../project-finance/accountingWorkActions';

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

  const selectedTotal = useMemo(() => {
    if (!summary) return total;
    if (status === 'all') return summary.total;
    return summary[`${status}_count` as keyof AccountingWorkPage['summary']];
  }, [status, summary, total]);

  return (
    <section className="mt-4 space-y-4" aria-label="Nghiệp vụ Kế toán cần xử lý">
      <div className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3">
        <p className="font-semibold text-blue-950">Ưu tiên tiếp nhận hồ sơ kế toán toàn công ty</p>
        <p className="mt-1 text-sm leading-6 text-blue-900">
          Xử lý ngay hồ sơ bàn giao tại đây. Mở hồ sơ nguồn để đối chiếu BOQ, hợp đồng, mua sắm, kho, thi công và bàn giao khi cần.
        </p>
      </div>

      <div className="flex gap-2 overflow-x-auto" aria-label="Lọc trạng thái hồ sơ kế toán">
        {statusTabs.map((tab) => (
          <button
            key={tab.value}
            type="button"
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

      <div className="grid gap-3 rounded-lg border border-line bg-white p-3 lg:grid-cols-[minmax(16rem,24rem)_minmax(18rem,1fr)_auto] lg:items-end">
        <label className="block text-xs font-semibold text-slate-600">
          Công trình
          <div className="mt-1">
            <AsyncProjectPicker
              value={project}
              onChange={(value) => onFilterChange({ project: value || null })}
              endpoint="/project-finance/projects"
              queryKey="my-accounting-work-project-options"
              emptyLabel="Tất cả công trình"
            />
          </div>
        </label>
        <form className="flex min-w-0 gap-2" onSubmit={submitSearch}>
          <label className="min-w-0 flex-1 text-xs font-semibold text-slate-600">
            Tìm hồ sơ
            <span className="mt-1 flex items-center rounded-md border border-line bg-white px-3 focus-within:border-blue-500 focus-within:ring-1 focus-within:ring-blue-500">
              <Search className="h-4 w-4 shrink-0 text-slate-400" />
              <input
                className="min-w-0 flex-1 border-0 bg-transparent px-2 py-2 text-sm outline-none"
                value={searchDraft}
                onChange={(event) => setSearchDraft(event.target.value)}
                placeholder="Mã hồ sơ, đối tác, mã MISA..."
              />
            </span>
          </label>
          <button type="submit" className="mt-5 rounded-md bg-slate-800 px-3 py-2 text-sm font-semibold text-white hover:bg-slate-900">Tìm</button>
        </form>
        <button
          type="button"
          className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          disabled={!project && !search}
          onClick={() => {
            setSearchDraft('');
            onFilterChange({ project: null, search: null });
          }}
        >
          Xóa lọc
        </button>
      </div>

      {loading ? <div className="rounded-lg border border-line bg-white p-8"><LoadingSpinner /></div> : error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          <p>Không tải được danh sách hồ sơ kế toán.</p>
          <button className="mt-2 font-semibold underline" onClick={onRetry}>Thử lại</button>
        </div>
      ) : rows.length === 0 ? (
        <EmptyState title={status === 'handed_off' ? 'Không có hồ sơ đang chờ Kế toán tiếp nhận.' : 'Không có hồ sơ phù hợp với bộ lọc.'} />
      ) : (
        <div className="overflow-hidden rounded-lg border border-line bg-white">
          <div className="hidden grid-cols-[minmax(16rem,1.6fr)_minmax(12rem,1fr)_10rem_10rem_15rem] gap-4 border-b border-line bg-slate-50 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-slate-500 xl:grid">
            <span>Hồ sơ nguồn</span><span>Công trình</span><span className="text-right">Giá trị</span><span>Trạng thái</span><span className="text-right">Xử lý</span>
          </div>
          {rows.map((item) => (
            <article key={item.handoff_id} className="grid gap-3 border-b border-line px-4 py-4 last:border-b-0 xl:grid-cols-[minmax(16rem,1.6fr)_minmax(12rem,1fr)_10rem_10rem_15rem] xl:items-center xl:gap-4">
              <div className="min-w-0">
                <Link className="font-semibold text-blue-700 hover:underline" to={item.source_route}>{item.source_code}</Link>
                <p className="truncate text-sm font-semibold text-ink" title={item.source_title}>{item.source_title}</p>
                <p className="truncate text-xs text-slate-500">{item.counterparty_name || 'Chưa xác định đối tác'}</p>
              </div>
              <div className="min-w-0">
                <Link className="truncate text-sm font-semibold text-slate-800 hover:text-blue-700 hover:underline" to={`/projects/${item.project_id}`}>{item.project_code} · {item.project_name}</Link>
                <p className="mt-1 text-xs text-slate-500">{item.direction === 'payable' ? 'Hồ sơ phải trả' : 'Hồ sơ phải thu'}</p>
              </div>
              <div className="xl:text-right">
                <CurrencyText value={item.reference_amount} />
                <p className="text-xs text-slate-400">{item.currency} · số liệu nguồn</p>
              </div>
              <div>
                <StatusBadge status={item.status} />
                <p className="mt-1 text-xs text-slate-500">{dateText(item.handed_off_at)}</p>
                {item.return_reason ? <p className="mt-1 line-clamp-2 text-xs text-rose-700">{item.return_reason}</p> : null}
              </div>
              <div className="flex flex-wrap justify-start gap-2 xl:justify-end">
                {item.status === 'handed_off' && canAcknowledge ? (
                  <>
                    <button className="inline-flex items-center gap-1.5 rounded-md bg-emerald-600 px-2.5 py-2 text-xs font-semibold text-white hover:bg-emerald-700" onClick={() => setDialog({ action: 'acknowledge', item })}>
                      <CheckCircle2 className="h-3.5 w-3.5" />Tiếp nhận
                    </button>
                    <button className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setDialog({ action: 'return', item })}>
                      <Undo2 className="h-3.5 w-3.5" />Trả lại
                    </button>
                  </>
                ) : null}
                <Link className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-blue-700 hover:bg-blue-50" to={item.source_route}>
                  <ExternalLink className="h-3.5 w-3.5" />Mở nguồn
                </Link>
              </div>
            </article>
          ))}
        </div>
      )}

      {rows.length ? (
        hasMore ? (
          <button className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={fetchingMore} onClick={onLoadMore}>
            {fetchingMore ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}
            Tải thêm · Đã hiển thị {rows.length}/{selectedTotal}
          </button>
        ) : <p className="text-center text-xs text-slate-500">Đã hiển thị {rows.length}/{selectedTotal}</p>
      ) : null}

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

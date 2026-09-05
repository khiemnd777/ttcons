import { useMemo, useState } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Activity, ArrowRight, Search } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { IdentityReferenceLabel } from '../../components/IdentityReferenceLabel';
import { ApiError, get } from '../../lib/api/client';
import { statusLabel } from '../../lib/utils/status';
import { timeText } from '../../lib/utils/format';
import { activityLabel, workflowFormActivityComment } from '../work-items/workItemPresentation';
import { dashboardActivityDate, dashboardActivityGroups, dashboardActivityHref, dashboardActivityQuery, dashboardActivityRanges, dashboardActivityTypes } from './dashboardActivity';
import type { DashboardActivitiesPage, DashboardActivity } from './dashboardTypes';

const controlClass = 'rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-700 focus-ring';
const buttonClass = 'rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 focus-ring disabled:opacity-50';
const activityColumns = 'lg:grid-cols-[minmax(0,1.1fr)_minmax(0,2fr)_minmax(0,0.9fr)_6rem]';

function activityComment(item: DashboardActivity) {
  return workflowFormActivityComment({ ...item, metadata_json: { form_version_id: item.form_version_id } });
}

export function DashboardActivityPanel({ actorID }: { actorID: string }) {
  const [range, setRange] = useState('7d');
  const [searchDraft, setSearchDraft] = useState('');
  const [search, setSearch] = useState('');
  const [activityType, setActivityType] = useState('');
  const [selectedID, setSelectedID] = useState('');
  const feed = useInfiniteQuery({
    queryKey: ['dashboard-activities', actorID, range, search, activityType],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<DashboardActivitiesPage>(`/reports/dashboard/activities?${dashboardActivityQuery(range, search, activityType, pageParam)}`),
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: Boolean(actorID),
    retry: false,
    refetchInterval: 60_000,
  });
  // A failed permission check must hide cached rows and their detail dialog.
  const rows = useMemo(() => feed.isError ? [] : feed.data?.pages.flatMap((page) => page.items) ?? [], [feed.data, feed.isError]);
  const groups = dashboardActivityGroups(rows);
  const selected = rows.find((item) => item.id === selectedID);
  const denied = (feed.error instanceof ApiError && feed.error.status === 403) || feed.data?.pages[0]?.can_view_activity === false;

  function resetFilters() {
    setRange('7d'); setSearchDraft(''); setSearch(''); setActivityType(''); setSelectedID('');
  }

  return (
    <section className="mt-5" aria-labelledby="dashboard-activity-title">
      <div className="mb-3">
        <h2 id="dashboard-activity-title" className="flex items-center gap-2 text-base font-semibold text-ink"><Activity className="h-4 w-4 text-blue-600" />Hoạt động công trình</h2>
        <p className="mt-1 text-xs leading-5 text-slate-500">Diễn biến mới nhất đã ghi nhận. Khoảng thời gian hoạt động độc lập với kỳ dòng tiền.</p>
      </div>
      <div className="mb-3 flex flex-col gap-2 xl:flex-row">
        <form className="flex min-w-0 flex-1" onSubmit={(event) => { event.preventDefault(); setSearch(searchDraft.trim()); setSelectedID(''); }}>
          <label className="relative min-w-0 flex-1">
            <span className="sr-only">Tìm công trình hoặc nội dung hoạt động</span>
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input className={`${controlClass} w-full rounded-r-none pl-9`} placeholder="Tìm công trình hoặc nội dung…" maxLength={200} value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} />
          </label>
          <button type="submit" className={`${buttonClass} rounded-l-none border-l-0`}>Tìm</button>
        </form>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-[auto_minmax(0,1fr)_auto]">
          <select aria-label="Khoảng thời gian hoạt động" className={controlClass} value={range} onChange={(event) => { setRange(event.target.value); setSelectedID(''); }}>{dashboardActivityRanges.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select>
          <select aria-label="Loại hoạt động" className={`${controlClass} min-w-0`} value={activityType} onChange={(event) => { setActivityType(event.target.value); setSelectedID(''); }}><option value="">Tất cả hoạt động</option>{dashboardActivityTypes.map((value) => <option key={value} value={value}>{activityLabel(value)}</option>)}</select>
          <button type="button" className={buttonClass} onClick={resetFilters}>Đặt lại</button>
        </div>
      </div>
      <div className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
        {!feed.isLoading && !denied && !feed.isError && rows.length > 0 ? (
          <div aria-hidden className={`hidden gap-5 border-b border-line px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500 lg:grid ${activityColumns}`}>
            <span>Công trình</span><span>Nội dung hoạt động</span><span>Người thực hiện</span><span className="text-right">Thời gian</span>
          </div>
        ) : null}
        {feed.isLoading ? <div className="p-8 text-center text-sm text-slate-500" role="status">Đang tải hoạt động…</div> : denied ? (
          <p className="p-8 text-center text-sm text-slate-500">Bạn chưa có quyền xem hoạt động công trình.</p>
        ) : feed.isError ? (
          <div className="p-6 text-center text-sm text-amber-800" role="alert">Không tải được hoạt động. <button type="button" className="font-semibold underline focus-ring" onClick={() => void feed.refetch()}>Thử lại</button></div>
        ) : !rows.length ? (
          <div className="p-8 text-center text-sm text-slate-500"><p>{search || activityType ? 'Không có hoạt động phù hợp với bộ lọc.' : 'Chưa có hoạt động trong khoảng thời gian này.'}</p>{search || activityType ? <button type="button" className="mt-3 font-semibold text-blue-700 focus-ring" onClick={resetFilters}>Đặt lại</button> : range !== 'all' ? <button type="button" className="mt-3 font-semibold text-blue-700 focus-ring" onClick={() => setRange('all')}>Mở rộng thời gian</button> : null}</div>
        ) : groups.map((group) => (
          <div key={group.label}>
            <h3 className="border-b border-line bg-slate-50 px-4 py-2 text-xs font-semibold text-slate-600">{group.label}</h3>
            {group.items.map((item) => <DashboardActivityRow key={item.id} item={item} onOpen={() => setSelectedID(item.id)} />)}
          </div>
        ))}
      </div>
      {!feed.isError && !denied && feed.hasNextPage ? <div className="mt-3 text-center"><button type="button" className={buttonClass} disabled={feed.isFetching} onClick={() => void feed.fetchNextPage()}>{feed.isFetchingNextPage ? 'Đang tải thêm…' : 'Tải thêm hoạt động'}</button></div> : null}
      {selected && !denied ? <DashboardActivityDetail item={selected} onClose={() => setSelectedID('')} /> : null}
    </section>
  );
}

function DashboardActivityRow({ item, onOpen }: { item: DashboardActivity; onOpen: () => void }) {
  const comment = activityComment(item);
  const tone = ['blocked', 'rejected', 'changes_requested', 'evidence_rejected'].includes(item.activity_type) ? 'bg-amber-500' : item.activity_type === 'completed' ? 'bg-emerald-500' : 'bg-blue-500';
  return (
    <article className={`grid grid-cols-1 items-start gap-3 border-b border-line px-4 py-3 last:border-b-0 lg:gap-5 ${activityColumns}`}>
      <div className="min-w-0 text-sm">
        <p className="break-words font-semibold text-ink">{item.project_code}</p>
        <p className="mt-0.5 line-clamp-2 break-words text-slate-600" title={item.project_name}>{item.project_name}</p>
      </div>
      <div className="min-w-0 text-sm">
        <p className="flex items-start gap-2 font-semibold text-slate-800"><span aria-hidden className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${tone}`} />{activityLabel(item.activity_type)}</p>
        {item.work_item_id ? <p className="mt-1 line-clamp-2 break-words font-medium text-slate-700" title={item.target_label}>{item.target_label}</p> : null}
        {comment ? <p className="mt-1 line-clamp-2 break-words text-slate-500">{comment}</p> : null}
      </div>
      <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 break-words text-xs text-slate-500 lg:block lg:text-sm">
        <IdentityReferenceLabel name={item.actor_name || 'Hệ thống'} deleted={item.actor_account_deleted} />
        <time className="tabular-nums lg:hidden" dateTime={item.created_at} title={dashboardActivityDate(item.created_at)}>· {timeText(item.created_at)}</time>
      </div>
      <div className="justify-self-end text-right">
        <time className="hidden text-xs tabular-nums text-slate-500 lg:block" dateTime={item.created_at} title={dashboardActivityDate(item.created_at)}>{timeText(item.created_at)}</time>
        <button type="button" className="rounded text-xs font-semibold text-blue-700 hover:underline focus-ring lg:mt-2" onClick={onOpen} aria-label={`Xem chi tiết ${activityLabel(item.activity_type)} · ${item.project_code}`}>Chi tiết</button>
      </div>
    </article>
  );
}

function DashboardActivityDetail({ item, onClose }: { item: DashboardActivity; onClose: () => void }) {
  const href = dashboardActivityHref(item);
  return (
    <Dialog open manageFocus title="Chi tiết hoạt động" description={`${item.project_code} · ${item.project_name}`} onClose={onClose} footer={(
      <div className="flex flex-wrap items-center justify-between gap-3"><button type="button" className={buttonClass} onClick={onClose}>Đóng</button>{href ? <Link className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 focus-ring" to={href} onClick={onClose}>{item.work_item_id ? 'Mở công việc' : 'Mở hoạt động công trình'}<ArrowRight className="h-4 w-4" /></Link> : null}</div>
    )}>
      <h3 className="break-words font-semibold text-ink">{activityLabel(item.activity_type)}{item.work_item_id ? ` · ${item.target_label}` : ''}</h3>
      <div className="mt-2 flex flex-wrap gap-x-2 text-sm text-slate-500"><IdentityReferenceLabel name={item.actor_name || 'Hệ thống'} deleted={item.actor_account_deleted} /><time dateTime={item.created_at}>· {dashboardActivityDate(item.created_at)}</time></div>
      {item.from_status && item.to_status && item.from_status !== item.to_status ? <p className="mt-4 rounded-md bg-blue-50 p-3 text-sm text-blue-800">Chuyển trạng thái lúc ghi nhận: {statusLabel(item.from_status)} → {statusLabel(item.to_status)}</p> : null}
      <p className="mt-4 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{activityComment(item) || 'Hoạt động này không có ghi chú bổ sung.'}</p>
    </Dialog>
  );
}

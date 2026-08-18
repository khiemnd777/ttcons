import { useMemo, useState, type ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, Clock3, Plus, RotateCcw, Search, ShieldCheck, UserRound, Wrench } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { RowActions } from '../../components/RowActions';
import type { ConstructionActionItem, ConstructionCheck, ConstructionWorkspace } from '../../lib/types/domain';
import { actionStatusLabel, checkStatusLabel, constructionCheckFilterWorkItems, dateTimeText, impactLabel, isConstructionActionEditable, isConstructionCheckEditable, isOverdue, normalizedSearch } from './constructionWorkspaceUtils';

type CheckQueue = 'pending' | 'pass' | 'fail' | 'all';
type ActionQueue = 'open' | 'waiting_confirmation' | 'done' | 'all';

export function ConstructionCheckList({ workspace, canCreate, canEdit, canPerform, onCreate, onView, onEdit, onComplete }: { workspace: ConstructionWorkspace; canCreate: boolean; canEdit: boolean; canPerform: boolean; onCreate: () => void; onView: (check: ConstructionCheck) => void; onEdit: (check: ConstructionCheck) => void; onComplete: (check: ConstructionCheck) => void }) {
  const [queue, setQueue] = useState<CheckQueue>('pending');
  const [query, setQuery] = useState('');
  const [workItemID, setWorkItemID] = useState('');
  const [timeFilter, setTimeFilter] = useState('all');
  const filterWorkItems = useMemo(() => constructionCheckFilterWorkItems(workspace), [workspace.checks, workspace.work_items]);
  const counts = useMemo(() => ({
    pending: workspace.checks.filter((row) => row.status === 'pending').length,
    pass: workspace.checks.filter((row) => row.result === 'pass').length,
    fail: workspace.checks.filter((row) => row.result === 'fail').length,
    all: workspace.checks.length,
  }), [workspace.checks]);
  const rows = useMemo(() => {
    const search = normalizedSearch(query);
    return workspace.checks.filter((row) => {
      const queueMatch = queue === 'all' || (queue === 'pending' ? row.status === 'pending' : row.result === queue);
      const searchMatch = !search || normalizedSearch([row.title, row.location, row.work_item_title, row.template_name, row.note].join(' ')).includes(search);
      const workMatch = !workItemID || row.work_item_id === workItemID;
      return queueMatch && searchMatch && workMatch && checkTimeMatches(row, timeFilter);
    }).sort((left, right) => {
      if (left.status !== right.status) return left.status === 'pending' ? -1 : 1;
      return checkSortTime(left) - checkSortTime(right);
    });
  }, [queue, query, timeFilter, workItemID, workspace.checks]);
  const filtered = Boolean(query || workItemID || timeFilter !== 'all' || queue !== 'pending');

  function reset() {
    setQueue('pending'); setQuery(''); setWorkItemID(''); setTimeFilter('all');
  }

  return <div className="space-y-3">
    <QueueTabs ariaLabel="Lọc trạng thái kiểm tra" value={queue} onChange={(value) => setQueue(value as CheckQueue)} items={[
      { id: 'pending', label: 'Cần kiểm tra', count: counts.pending },
      { id: 'pass', label: 'Đạt', count: counts.pass },
      { id: 'fail', label: 'Không đạt', count: counts.fail },
      { id: 'all', label: 'Tất cả', count: counts.all },
    ]} />
    <FilterToolbar onReset={reset} showReset={filtered}>
      <SearchInput value={query} onChange={setQuery} placeholder="Tìm nội dung, vị trí, công việc" />
      <FilterSelect value={workItemID} onChange={setWorkItemID} ariaLabel="Lọc theo công việc"><option value="">Tất cả hạng mục kiểm tra</option>{filterWorkItems.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</FilterSelect>
      <FilterSelect value={timeFilter} onChange={setTimeFilter} ariaLabel="Lọc theo thời gian"><option value="all">Mọi thời điểm</option><option value="overdue">Đã quá lịch</option><option value="today">Hôm nay</option><option value="upcoming">Sắp tới</option><option value="unscheduled">Chưa đặt lịch</option></FilterSelect>
    </FilterToolbar>
    {rows.length ? <>
      <div className="hidden overflow-x-auto rounded-lg border border-line bg-white lg:block">
        <table className="min-w-full divide-y divide-line">
          <thead className="bg-slate-50"><tr><Th><span className="sr-only">Thao tác</span></Th><Th>Nội dung / vị trí</Th><Th>Công việc</Th><Th>Lịch kiểm tra</Th><Th>Kết quả</Th></tr></thead>
          <tbody className="divide-y divide-line">{rows.map((row) => <tr key={row.id} className="hover:bg-slate-50">
            <Td><div className="flex gap-2"><RowActions onView={() => onView(row)} onEdit={canEdit && isConstructionCheckEditable(row) ? () => onEdit(row) : undefined} />{row.status === 'pending' && canPerform ? <PrimaryButton onClick={() => onComplete(row)} icon={<CheckCircle2 className="h-4 w-4" />}>Ghi kết quả</PrimaryButton> : null}</div></Td>
            <Td><button className="max-w-[24rem] text-left" onClick={() => onView(row)}><span className="block font-semibold text-ink">{row.title}</span><span className="mt-1 block text-xs text-slate-500">{row.location || 'Chưa ghi vị trí'} · {row.criteria_jsonb?.length ?? 0} tiêu chí · {row.context_evidence?.length ?? 0} ảnh hiện trạng</span>{row.source_handoff_item_id ? <PlanningSourceBadge revision={row.handoff_revision_no} /> : row.technical_rfi_id ? <SourceBadge /> : null}</button></Td>
            <Td><span className="line-clamp-2 max-w-[17rem]">{row.work_item_title || 'Công việc liên quan'}</span><span className="mt-1 block text-xs text-slate-500">{row.template_name || 'Không dùng mẫu'}</span></Td>
            <Td><span className={clsx('inline-flex items-center gap-1.5', isOverdue(row.scheduled_at, row.status) && 'font-semibold text-rose-700')}><Clock3 className="h-3.5 w-3.5" />{dateTimeText(row.scheduled_at || row.checked_at || row.created_at)}</span><span className="mt-1 block text-xs text-slate-500">{row.checked_by_name || row.requested_by_name || 'Chưa xác định người thực hiện'}</span></Td>
            <Td><CheckStatusPill row={row} /></Td>
          </tr>)}</tbody>
        </table>
      </div>
      <div className="space-y-3 lg:hidden">{rows.map((row) => <article key={row.id} className="rounded-lg border border-line bg-white p-4 shadow-sm">
        <div className="flex items-start justify-between gap-3"><button className="min-w-0 text-left" onClick={() => onView(row)}><h3 className="font-semibold text-ink">{row.title}</h3><p className="mt-1 line-clamp-2 text-xs text-slate-500">{row.work_item_title || 'Công việc liên quan'} · {row.location || 'Chưa ghi vị trí'}</p>{row.source_handoff_item_id ? <PlanningSourceBadge revision={row.handoff_revision_no} /> : null}</button><CheckStatusPill row={row} /></div>
        <div className="mt-3 grid grid-cols-2 gap-2 rounded-md bg-slate-50 p-3 text-xs"><Info label="Thời điểm" value={dateTimeText(row.scheduled_at || row.checked_at || row.created_at)} /><Info label="Tiêu chí" value={`${row.criteria_jsonb?.length ?? 0} mục`} /><Info label="Hiện trạng" value={`${row.context_evidence?.length ?? 0} ảnh/tệp`} /></div>
        <div className="mt-3 flex items-center justify-between gap-2"><RowActions onView={() => onView(row)} onEdit={canEdit && isConstructionCheckEditable(row) ? () => onEdit(row) : undefined} />{row.status === 'pending' && canPerform ? <PrimaryButton onClick={() => onComplete(row)} icon={<CheckCircle2 className="h-4 w-4" />}>Ghi kết quả</PrimaryButton> : null}</div>
      </article>)}</div>
    </> : <EmptyState title={filtered ? 'Không có kiểm tra phù hợp bộ lọc' : 'Không có kiểm tra đang chờ'} action={filtered ? <ResetButton onClick={reset} /> : canCreate ? <PrimaryButton onClick={onCreate} icon={<Plus className="h-4 w-4" />}>Tạo kiểm tra</PrimaryButton> : undefined} />}
  </div>;
}

export function ConstructionActionList({ workspace, canCreate, canEdit, canResolve, canVerify, onCreate, onView, onEdit, onResolve, onVerify }: { workspace: ConstructionWorkspace; canCreate: boolean; canEdit: boolean; canResolve: boolean; canVerify: boolean; onCreate: () => void; onView: (action: ConstructionActionItem) => void; onEdit: (action: ConstructionActionItem) => void; onResolve: (action: ConstructionActionItem) => void; onVerify: (action: ConstructionActionItem) => void }) {
  const [queue, setQueue] = useState<ActionQueue>('open');
  const [query, setQuery] = useState('');
  const [assignee, setAssignee] = useState('');
  const [priority, setPriority] = useState('');
  const [impact, setImpact] = useState('');
  const counts = useMemo(() => ({ open: workspace.actions.filter((row) => row.status === 'open').length, waiting_confirmation: workspace.actions.filter((row) => row.status === 'waiting_confirmation').length, done: workspace.actions.filter((row) => row.status === 'done').length, all: workspace.actions.length }), [workspace.actions]);
  const rows = useMemo(() => {
    const search = normalizedSearch(query);
    return workspace.actions.filter((row) => {
      const queueMatch = queue === 'all' || row.status === queue;
      const searchMatch = !search || normalizedSearch([row.title, row.description, row.work_item_title, row.source_check_title, row.assigned_user_name].join(' ')).includes(search);
      return queueMatch && searchMatch && (!assignee || row.assigned_user_id === assignee) && (!priority || row.priority === priority) && (!impact || row.impact_scope === impact);
    }).sort((left, right) => actionSortScore(left) - actionSortScore(right));
  }, [assignee, impact, priority, query, queue, workspace.actions]);
  const filtered = Boolean(query || assignee || priority || impact || queue !== 'open');
  function reset() { setQueue('open'); setQuery(''); setAssignee(''); setPriority(''); setImpact(''); }

  return <div className="space-y-3">
    <QueueTabs ariaLabel="Lọc trạng thái việc cần xử lý" value={queue} onChange={(value) => setQueue(value as ActionQueue)} items={[
      { id: 'open', label: 'Cần xử lý', count: counts.open }, { id: 'waiting_confirmation', label: 'Chờ xác nhận', count: counts.waiting_confirmation }, { id: 'done', label: 'Đã xong', count: counts.done }, { id: 'all', label: 'Tất cả', count: counts.all },
    ]} />
    <FilterToolbar onReset={reset} showReset={filtered}>
      <SearchInput value={query} onChange={setQuery} placeholder="Tìm việc, công việc, người xử lý" />
      <FilterSelect value={assignee} onChange={setAssignee} ariaLabel="Lọc người xử lý"><option value="">Tất cả người xử lý</option>{workspace.participants.map((item) => <option key={item.id} value={item.id}>{item.full_name}</option>)}</FilterSelect>
      <FilterSelect value={priority} onChange={setPriority} ariaLabel="Lọc mức ưu tiên"><option value="">Mọi ưu tiên</option><option value="urgent">Gấp</option><option value="normal">Bình thường</option></FilterSelect>
      <FilterSelect value={impact} onChange={setImpact} ariaLabel="Lọc ảnh hưởng"><option value="">Mọi ảnh hưởng</option><option value="none">Không ảnh hưởng</option><option value="timeline">Tiến độ</option><option value="budget">Chi phí</option><option value="design_scope">Thiết kế / phạm vi</option></FilterSelect>
    </FilterToolbar>
    {rows.length ? <>
      <div className="hidden overflow-x-auto rounded-lg border border-line bg-white lg:block"><table className="min-w-full divide-y divide-line">
        <thead className="bg-slate-50"><tr><Th><span className="sr-only">Thao tác</span></Th><Th>Việc / nguồn</Th><Th>Người xử lý</Th><Th>Hạn xử lý</Th><Th>Ảnh hưởng</Th><Th>Trạng thái</Th></tr></thead>
        <tbody className="divide-y divide-line">{rows.map((row) => <tr key={row.id} className={clsx('hover:bg-slate-50', row.priority === 'urgent' && row.status !== 'done' && 'bg-rose-50/30')}>
          <Td><div className="flex gap-2"><RowActions onView={() => onView(row)} onEdit={canEdit && isConstructionActionEditable(row) ? () => onEdit(row) : undefined} />{row.status === 'open' && canResolve ? <PrimaryButton onClick={() => onResolve(row)} icon={<Wrench className="h-4 w-4" />}>Gửi khắc phục</PrimaryButton> : row.status === 'waiting_confirmation' && canVerify ? <PrimaryButton tone="green" onClick={() => onVerify(row)} icon={<ShieldCheck className="h-4 w-4" />}>Xác nhận</PrimaryButton> : null}</div></Td>
          <Td><button className="max-w-[24rem] text-left" onClick={() => onView(row)}><span className="flex items-center gap-2 font-semibold text-ink">{row.priority === 'urgent' ? <AlertTriangle className="h-4 w-4 shrink-0 text-rose-600" /> : null}{row.title}</span><span className="mt-1 block line-clamp-2 text-xs text-slate-500">{row.source_check_title ? `Từ kiểm tra: ${row.source_check_title}` : row.work_item_title || 'Công việc liên quan'}</span>{!isConstructionActionEditable(row) && row.status === 'open' ? <SourceBadge /> : null}</button></Td>
          <Td><span className="inline-flex items-center gap-1.5"><UserRound className="h-3.5 w-3.5 text-slate-400" />{row.assigned_user_name || 'Chưa xác định'}</span></Td>
          <Td><span className={clsx(isOverdue(row.due_at, row.status) && 'font-semibold text-rose-700')}>{isOverdue(row.due_at, row.status) ? 'Quá hạn · ' : ''}{dateTimeText(row.due_at)}</span></Td>
          <Td>{impactLabel(row.impact_scope)}</Td><Td><ActionStatusPill status={row.status} /></Td>
        </tr>)}</tbody>
      </table></div>
      <div className="space-y-3 lg:hidden">{rows.map((row) => <article key={row.id} className={clsx('rounded-lg border bg-white p-4 shadow-sm', row.priority === 'urgent' && row.status !== 'done' ? 'border-rose-200' : 'border-line')}>
        <div className="flex items-start justify-between gap-3"><button className="min-w-0 text-left" onClick={() => onView(row)}><h3 className="font-semibold text-ink">{row.title}</h3><p className="mt-1 line-clamp-2 text-xs text-slate-500">{row.source_check_title || row.work_item_title || 'Công việc liên quan'}</p></button><ActionStatusPill status={row.status} /></div>
        <div className="mt-3 grid grid-cols-2 gap-2 rounded-md bg-slate-50 p-3 text-xs"><Info label="Người xử lý" value={row.assigned_user_name || 'Chưa xác định'} /><Info label="Hạn xử lý" value={`${isOverdue(row.due_at, row.status) ? 'Quá hạn · ' : ''}${dateTimeText(row.due_at)}`} /><Info label="Ưu tiên" value={row.priority === 'urgent' ? 'Gấp' : 'Bình thường'} /><Info label="Ảnh hưởng" value={impactLabel(row.impact_scope)} /></div>
        <div className="mt-3 flex items-center justify-between gap-2"><RowActions onView={() => onView(row)} onEdit={canEdit && isConstructionActionEditable(row) ? () => onEdit(row) : undefined} />{row.status === 'open' && canResolve ? <PrimaryButton onClick={() => onResolve(row)} icon={<Wrench className="h-4 w-4" />}>Gửi khắc phục</PrimaryButton> : row.status === 'waiting_confirmation' && canVerify ? <PrimaryButton tone="green" onClick={() => onVerify(row)} icon={<ShieldCheck className="h-4 w-4" />}>Xác nhận</PrimaryButton> : null}</div>
      </article>)}</div>
    </> : <EmptyState title={filtered ? 'Không có việc phù hợp bộ lọc' : 'Không còn việc cần xử lý'} action={filtered ? <ResetButton onClick={reset} /> : canCreate ? <PrimaryButton onClick={onCreate} icon={<Plus className="h-4 w-4" />}>Tạo việc</PrimaryButton> : undefined} />}
  </div>;
}

function QueueTabs({ items, value, onChange, ariaLabel }: { items: Array<{ id: string; label: string; count: number }>; value: string; onChange: (value: string) => void; ariaLabel: string }) {
  return <div className="overflow-x-auto"><div className="inline-flex min-w-max rounded-lg border border-line bg-slate-50 p-1" role="tablist" aria-label={ariaLabel}>{items.map((item) => <button key={item.id} role="tab" aria-selected={value === item.id} className={clsx('rounded-md px-3 py-1.5 text-sm font-semibold transition', value === item.id ? 'bg-white text-blue-700 shadow-sm ring-1 ring-inset ring-line' : 'text-slate-600 hover:text-ink')} onClick={() => onChange(item.id)}>{item.label} <span className="ml-1 rounded-full bg-slate-100 px-1.5 py-0.5 text-xs tabular-nums">{item.count}</span></button>)}</div></div>;
}

function FilterToolbar({ children, showReset, onReset }: { children: ReactNode; showReset: boolean; onReset: () => void }) {
  return <div className="flex flex-col gap-2 rounded-lg border border-line bg-white p-3 md:flex-row md:flex-wrap md:items-center">{children}{showReset ? <button className="inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100" onClick={onReset}><RotateCcw className="h-4 w-4" /> Xóa lọc</button> : null}</div>;
}
function SearchInput({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder: string }) { return <label className="relative min-w-0 flex-1 md:min-w-64"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} /></label>; }
function FilterSelect({ value, onChange, ariaLabel, children }: { value: string; onChange: (value: string) => void; ariaLabel: string; children: ReactNode }) { return <select aria-label={ariaLabel} className="min-w-40 rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={value} onChange={(event) => onChange(event.target.value)}>{children}</select>; }
function Th({ children, className }: { children: ReactNode; className?: string }) { return <th className={clsx('whitespace-nowrap px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500', className)}>{children}</th>; }
function Td({ children, className }: { children: ReactNode; className?: string }) { return <td className={clsx('px-3 py-3 text-sm text-slate-700', className)}>{children}</td>; }
function Info({ label, value }: { label: string; value: string }) { return <p><span className="block text-[11px] font-semibold uppercase tracking-wide text-slate-400">{label}</span><span className="mt-0.5 block font-medium text-slate-700">{value}</span></p>; }
function SourceBadge() { return <span className="mt-1 inline-flex rounded bg-violet-50 px-2 py-0.5 text-[11px] font-semibold text-violet-700">Nguồn QA/QC · chỉ đọc</span>; }
function PlanningSourceBadge({ revision }: { revision?: number }) { return <span className="mt-1 inline-flex rounded bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-700">Từ kế hoạch{revision ? ` · rev ${revision}` : ''}</span>; }
function CheckStatusPill({ row }: { row: ConstructionCheck }) { const tone = row.result === 'pass' ? 'bg-emerald-50 text-emerald-700' : row.result === 'fail' ? 'bg-rose-50 text-rose-700' : 'bg-amber-50 text-amber-800'; return <span className={clsx('inline-flex shrink-0 rounded-md px-2 py-1 text-xs font-semibold', tone)}>{checkStatusLabel(row)}</span>; }
function ActionStatusPill({ status }: { status: ConstructionActionItem['status'] }) { const tone = status === 'open' ? 'bg-rose-50 text-rose-700' : status === 'waiting_confirmation' ? 'bg-amber-50 text-amber-800' : status === 'done' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'; return <span className={clsx('inline-flex shrink-0 rounded-md px-2 py-1 text-xs font-semibold', tone)}>{actionStatusLabel(status)}</span>; }
function PrimaryButton({ children, icon, onClick, tone = 'blue' }: { children: ReactNode; icon?: ReactNode; onClick: () => void; tone?: 'blue' | 'green' }) { return <button className={clsx('inline-flex shrink-0 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold text-white', tone === 'green' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-blue-600 hover:bg-blue-700')} onClick={onClick}>{icon}{children}</button>; }
function ResetButton({ onClick }: { onClick: () => void }) { return <button className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClick}><RotateCcw className="h-4 w-4" /> Xóa bộ lọc</button>; }

function checkTimeMatches(row: ConstructionCheck, filter: string) {
  if (filter === 'all') return true;
  if (!row.scheduled_at) return filter === 'unscheduled';
  const value = new Date(row.scheduled_at);
  if (Number.isNaN(value.getTime())) return false;
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const end = new Date(start.getTime() + 86_400_000);
  if (filter === 'overdue') return row.status === 'pending' && value < now;
  if (filter === 'today') return value >= start && value < end;
  if (filter === 'upcoming') return value >= end;
  return false;
}
function checkSortTime(row: ConstructionCheck) { const value = new Date(row.scheduled_at || row.checked_at || row.created_at || '').getTime(); return Number.isFinite(value) ? value : Number.MAX_SAFE_INTEGER; }
function actionSortScore(row: ConstructionActionItem) { const due = new Date(row.due_at || '').getTime(); const time = Number.isFinite(due) ? due : Number.MAX_SAFE_INTEGER; const status = row.status === 'open' ? 0 : row.status === 'waiting_confirmation' ? 2_000_000_000_000_000 : 4_000_000_000_000_000; const priority = row.priority === 'urgent' && row.status !== 'done' ? -1_000_000_000_000_000 : 0; return status + priority + time; }

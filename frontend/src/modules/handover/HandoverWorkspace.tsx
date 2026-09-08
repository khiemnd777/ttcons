import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { ArrowRight, CheckCircle2, Eye, FileCheck2, RefreshCw, Send } from 'lucide-react';
import { Link } from 'react-router-dom';
import { RecordLink } from '../_shared/RecordLinks';
import { StatusBadge } from '../../components/StatusBadge';
import { dateText } from '../../lib/utils/format';
import { checklistItemCodeLabel, checklistItemCodeTooltip, checklistSourceLabel, checklistSourceMode } from './handoverSourcePresentation';
import type { ChecklistItem, HandoverDetail } from './handoverTypes';

type WorkspaceTab = 'tasks' | 'checklist' | 'info';

export function HandoverWorkspace({ detail, canVerify, canApprove, pending, onChecklist, onViewSource, onSubmit, onCheck, onComplete }: { detail: HandoverDetail; canVerify: boolean; canApprove: boolean; pending: boolean; onChecklist: (item: ChecklistItem) => void; onViewSource: (item: ChecklistItem) => void; onSubmit: () => void; onCheck: () => void; onComplete: () => void }) {
  const [tab, setTab] = useState<WorkspaceTab>('tasks');
  const snapshot = detail.readiness_snapshot_json ?? {};
  const blockers = snapshot.blockers ?? [];
  const verified = detail.checklist_items.filter((item) => item.status === 'verified').length;
  const groups = useMemo(() => groupChecklist(detail.checklist_items), [detail.checklist_items]);
  const next = nextAction(detail.status, blockers.length, { onSubmit, onCheck, onComplete, onTasks: () => setTab('tasks') });

  useEffect(() => {
    setTab(detail.status === 'draft' ? 'checklist' : detail.status === 'handed_over' ? 'info' : 'tasks');
  }, [detail.id, detail.status]);

  return <div className="overflow-clip rounded-lg border border-line bg-white shadow-panel">
    <div className={`border-b p-4 ${detail.status === 'blocked' ? 'border-amber-200 bg-amber-50' : detail.status === 'ready' || detail.status === 'handed_over' ? 'border-emerald-200 bg-emerald-50' : 'border-line bg-slate-50'}`}>
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2"><h2 className="text-lg font-semibold text-ink">{detail.handover_code}</h2><StatusBadge status={detail.status} /></div>
          <p className="mt-1 text-sm text-slate-600">{next.description}</p>
          <p className="mt-1 text-xs text-slate-500">Nghiệm thu cuối: {resultLabel(detail.final_acceptance_result)} · Dự kiến {dateText(detail.handover_date)}</p>
        </div>
        {next.action && (next.permission === 'approve' ? canApprove : canVerify) ? <button type="button" disabled={pending} className={`inline-flex items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-semibold text-white disabled:opacity-50 ${next.tone === 'green' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-blue-600 hover:bg-blue-700'}`} onClick={next.action}>{next.icon}{pending ? 'Đang xử lý...' : next.label}</button> : null}
      </div>
    </div>

    <div className="grid gap-3 border-b border-line p-4 sm:grid-cols-2 xl:grid-cols-4">
      <Metric label="Checklist" value={`${verified}/${detail.checklist_items.length}`} />
      <Metric label="Readiness" value={snapshot.ready ? 'Đạt' : detail.status === 'draft' ? 'Chưa kiểm tra' : 'Chưa đạt'} tone={snapshot.ready ? 'green' : blockers.length ? 'amber' : 'slate'} />
      <Metric label="Điểm chặn" value={blockers.length} tone={blockers.length ? 'amber' : 'slate'} />
      <Metric label="Kiểm tra gần nhất" value={dateText(snapshot.checked_at)} />
    </div>

    <div className="flex gap-1 overflow-x-auto border-b border-line px-4">
      <WorkspaceTabButton active={tab === 'tasks'} label="Việc cần làm" count={blockers.length} warning={blockers.length > 0} onClick={() => setTab('tasks')} />
      <WorkspaceTabButton active={tab === 'checklist'} label="Checklist" count={detail.checklist_items.length - verified} onClick={() => setTab('checklist')} />
      <WorkspaceTabButton active={tab === 'info'} label="Thông tin hồ sơ" onClick={() => setTab('info')} />
    </div>

    <div className="p-4 sm:p-5">
      {tab === 'tasks' ? <TaskPanel detail={detail} blockers={blockers} canVerify={canVerify} pending={pending} onCheck={onCheck} /> : null}
      {tab === 'checklist' ? <div className="space-y-5">{groups.length ? groups.map(([category, items]) => <ChecklistGroup key={category} category={category} items={items} projectId={detail.project_id} handoverStatus={detail.status} canVerify={canVerify} onChecklist={onChecklist} onViewSource={onViewSource} />) : <div className="rounded-lg border border-dashed border-line p-8 text-center text-sm text-slate-500">Hồ sơ chưa có checklist.</div>}</div> : null}
      {tab === 'info' ? <PackageInfo detail={detail} /> : null}
    </div>
  </div>;
}

function TaskPanel({ detail, blockers, canVerify, pending, onCheck }: { detail: HandoverDetail; blockers: string[]; canVerify: boolean; pending: boolean; onCheck: () => void }) {
  const snapshot = detail.readiness_snapshot_json ?? {};
  if (blockers.length) {
    const groups = groupBlockers(detail.project_id, blockers);
    return <div className="space-y-5">
      <div><h3 className="font-semibold text-ink">{blockers.length} điểm chặn cần xử lý</h3><p className="mt-1 text-sm text-slate-500">Xử lý tại module nguồn, sau đó quay lại chạy kiểm tra readiness mới.</p></div>
      {groups.map(([group, items]) => <section key={group}><h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{group}</h4><div className="divide-y divide-amber-200 rounded-lg border border-amber-200 bg-amber-50 px-4">{items.map(({ blocker, target }) => <div key={blocker} className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between"><span className="text-sm text-amber-900">{blocker}</span><Link className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700" to={target.path}>{target.label}<ArrowRight className="h-3.5 w-3.5" /></Link></div>)}</div></section>)}
      {canVerify ? <div className="flex justify-end"><button type="button" disabled={pending} className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={onCheck}><RefreshCw className="h-4 w-4" />Kiểm tra lại readiness</button></div> : null}
    </div>;
  }
  if (snapshot.ready) return <div className="flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800"><CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0" /><div><div className="font-semibold">Readiness đã đạt</div><p className="mt-1">Không còn điểm chặn trong snapshot gần nhất. Hồ sơ có thể hoàn tất khi người có quyền xác nhận.</p></div></div>;
  return <div className="rounded-lg border border-dashed border-line p-8 text-center"><div className="font-semibold text-ink">Chưa có snapshot readiness</div><p className="mt-1 text-sm text-slate-500">Hoàn thiện checklist rồi gửi hồ sơ để bắt đầu kiểm tra.</p></div>;
}

function ChecklistGroup({ category, items, projectId, handoverStatus, canVerify, onChecklist, onViewSource }: { category: string; items: ChecklistItem[]; projectId: string; handoverStatus: string; canVerify: boolean; onChecklist: (item: ChecklistItem) => void; onViewSource: (item: ChecklistItem) => void }) {
  const verified = items.filter((item) => item.status === 'verified').length;
  return <section>
    <div className="mb-2 flex items-center justify-between gap-3"><h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">{categoryLabel(category)}</h4><span className="text-xs font-semibold text-slate-600">{verified}/{items.length}</span></div>
    <div className="consflow-table-viewport hidden rounded-lg border border-line md:block"><table className="min-w-[760px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-3 py-2"><span className="sr-only">Thao tác</span></th><th className="px-3 py-2">Nội dung</th><th className="px-3 py-2">Nguồn xác minh</th><th className="px-3 py-2">Trạng thái</th></tr></thead><tbody className="divide-y divide-line">{items.map((item) => <ChecklistRow key={item.id} item={item} projectId={projectId} handoverStatus={handoverStatus} canVerify={canVerify} onChecklist={onChecklist} onViewSource={onViewSource} />)}</tbody></table></div>
    <div className="space-y-2 md:hidden">{items.map((item) => <ChecklistMobileCard key={item.id} item={item} projectId={projectId} handoverStatus={handoverStatus} canVerify={canVerify} onChecklist={onChecklist} onViewSource={onViewSource} />)}</div>
  </section>;
}

type ChecklistActionProps = { item: ChecklistItem; projectId: string; handoverStatus: string; canVerify: boolean; onChecklist: (item: ChecklistItem) => void; onViewSource: (item: ChecklistItem) => void };

function ChecklistRow(props: ChecklistActionProps) {
  const { item, projectId } = props;
  const mode = checklistSourceMode(item);
  return <tr><td className="px-3 py-3"><ChecklistAction {...props} /></td><td className="px-3 py-3"><div className="font-semibold text-ink">{item.title}</div><div className="text-xs text-slate-500" title={checklistItemCodeTooltip(item.item_code)}>{checklistItemCodeLabel(item.item_code)}{item.required ? ' · Bắt buộc' : ''}</div></td><td className="px-3 py-3"><div className="text-slate-700">{checklistSourceLabel(item, projectId)}</div><div className="mt-0.5 text-xs text-slate-400">{mode === 'automatic' ? 'Hệ thống tự đối chiếu' : mode === 'linked' ? 'Nguồn do người dùng liên kết' : 'Chưa liên kết nguồn'}</div></td><td className="px-3 py-3"><StatusBadge status={item.status} /></td></tr>;
}

function ChecklistMobileCard(props: ChecklistActionProps) {
  const { item, projectId } = props;
  const mode = checklistSourceMode(item);
  return <div className="rounded-lg border border-line p-3"><div className="flex items-start justify-between gap-3"><div><div className="font-semibold text-ink">{item.title}</div><div className="mt-1 text-xs text-slate-500" title={checklistItemCodeTooltip(item.item_code)}>{checklistItemCodeLabel(item.item_code)}{item.required ? ' · Bắt buộc' : ''}</div></div><StatusBadge status={item.status} /></div><div className="mt-3 text-xs text-slate-600">Nguồn: {checklistSourceLabel(item, projectId)}</div><div className="mt-0.5 text-xs text-slate-400">{mode === 'automatic' ? 'Hệ thống tự đối chiếu' : mode === 'linked' ? 'Nguồn do người dùng liên kết' : 'Chưa liên kết nguồn'}</div><div className="mt-3"><ChecklistAction {...props} /></div></div>;
}

function ChecklistAction({ item, handoverStatus, canVerify, onChecklist, onViewSource }: ChecklistActionProps) {
  const mode = checklistSourceMode(item);
  if (mode === 'automatic' || mode === 'linked') return <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={() => onViewSource(item)}><Eye className="h-3.5 w-3.5" />Xem nguồn</button>;
  if (canVerify && !['handed_over', 'cancelled', 'deleted'].includes(handoverStatus)) return <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold hover:bg-slate-100" onClick={() => onChecklist(item)}><FileCheck2 className="h-3.5 w-3.5" />Xác minh</button>;
  return <span className="text-xs text-slate-400">-</span>;
}

function PackageInfo({ detail }: { detail: HandoverDetail }) {
  return <div className="grid gap-5 lg:grid-cols-2">
    <section className="rounded-lg border border-line p-4"><h3 className="font-semibold text-ink">Nguồn và đại diện</h3><dl className="mt-4 space-y-3 text-sm"><InfoRow label="Nghiệm thu cuối" value={<RecordLink target={detail.final_acceptance_id ? { kind: 'acceptance', id: detail.final_acceptance_id, projectId: detail.project_id } : null}>{detail.final_acceptance_title || '-'}</RecordLink>} /><InfoRow label="Kết quả" value={resultLabel(detail.final_acceptance_result)} /><InfoRow label="Đại diện chủ đầu tư" value={detail.client_representative || '-'} /><InfoRow label="Đại diện nhà thầu" value={detail.company_representative || '-'} /></dl></section>
    <section className="rounded-lg border border-line bg-slate-50 p-4"><h3 className="font-semibold text-ink">Thời gian và ghi chú</h3><dl className="mt-4 space-y-3 text-sm"><InfoRow label="Ngày dự kiến" value={dateText(detail.handover_date)} /><InfoRow label="Hoàn tất" value={dateText(detail.completed_at)} /><InfoRow label="Cập nhật" value={dateText(detail.updated_at)} /></dl><div className="mt-4 rounded-md border border-line bg-white p-3 text-sm leading-6 text-slate-600">{detail.notes || 'Không có ghi chú.'}</div></section>
  </div>;
}

function nextAction(status: string, blockerCount: number, actions: { onSubmit: () => void; onCheck: () => void; onComplete: () => void; onTasks: () => void }) {
  if (status === 'handed_over') return { tone: 'green', description: 'Hồ sơ đã hoàn tất và được khóa theo readiness tại thời điểm bàn giao.', permission: 'approve' as const };
  if (status === 'ready') return { tone: 'green', description: 'Readiness đã đạt. Bước tiếp theo là xác nhận hoàn tất bàn giao.', permission: 'approve' as const, label: 'Hoàn tất bàn giao', icon: <CheckCircle2 className="h-4 w-4" />, action: actions.onComplete };
  if (status === 'submitted') return { tone: 'blue', description: 'Hồ sơ đã gửi. Chạy kiểm tra để tạo snapshot readiness mới.', permission: 'verify' as const, label: 'Kiểm tra readiness', icon: <RefreshCw className="h-4 w-4" />, action: actions.onCheck };
  if (status === 'blocked') return { tone: 'blue', description: `Xử lý ${blockerCount} điểm chặn tại module nguồn rồi kiểm tra lại.`, permission: 'verify' as const, label: 'Xử lý điểm chặn', icon: <ArrowRight className="h-4 w-4" />, action: actions.onTasks };
  return { tone: 'blue', description: 'Hoàn thiện checklist và gửi hồ sơ để kiểm tra readiness.', permission: 'verify' as const, label: 'Gửi kiểm tra', icon: <Send className="h-4 w-4" />, action: actions.onSubmit };
}

function blockerTarget(projectId: string, blocker: string) {
  const value = blocker.toLocaleLowerCase('vi');
  if (value.includes('chi phí vật tư') || value.includes('đề nghị thu') || value.includes('đề nghị chi') || value.includes('công nợ') || value.includes('phiếu chi') || value.includes('phiếu thu')) return { group: 'Kế toán', path: `/projects/${projectId}/accounting`, label: 'Mở Bàn giao kế toán' };
  if (value.includes('vật tư') || value.includes('kho')) return { group: 'Kho và vật tư', path: `/projects/${projectId}/warehouse`, label: 'Mở Kho vật tư' };
  if (value.includes('nghiệm thu')) return { group: 'Chất lượng', path: `/projects/${projectId}/acceptance`, label: 'Mở Nghiệm thu' };
  if (value.includes('lỗi') || value.includes('ncr') || value.includes('rework') || value.includes('công trường')) return { group: 'Thi công và chất lượng', path: `/projects/${projectId}/operations`, label: 'Mở Thi công' };
  return { group: 'Hồ sơ', path: `/projects/${projectId}?tab=files`, label: 'Mở Hồ sơ' };
}

function groupBlockers(projectId: string, blockers: string[]) {
  const groups = new Map<string, Array<{ blocker: string; target: ReturnType<typeof blockerTarget> }>>();
  blockers.forEach((blocker) => { const target = blockerTarget(projectId, blocker); groups.set(target.group, [...(groups.get(target.group) ?? []), { blocker, target }]); });
  return Array.from(groups.entries());
}

function groupChecklist(items: ChecklistItem[]) {
  const groups = new Map<string, ChecklistItem[]>();
  items.forEach((item) => groups.set(item.category || 'other', [...(groups.get(item.category || 'other') ?? []), item]));
  return Array.from(groups.entries());
}

function WorkspaceTabButton({ active, label, count, warning, onClick }: { active: boolean; label: string; count?: number; warning?: boolean; onClick: () => void }) {
  return <button type="button" className={`whitespace-nowrap border-b-2 px-3 py-3 text-sm font-semibold ${active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink'}`} onClick={onClick}>{label}{count !== undefined ? <span className={`ml-1.5 rounded-full px-1.5 py-0.5 text-[11px] ${warning ? 'bg-amber-100 text-amber-800' : active ? 'bg-blue-100 text-blue-700' : 'bg-slate-100 text-slate-600'}`}>{count}</span> : null}</button>;
}

function Metric({ label, value, tone = 'slate' }: { label: string; value: ReactNode; tone?: 'slate' | 'amber' | 'green' }) {
  const color = tone === 'amber' ? 'border-amber-200 bg-amber-50' : tone === 'green' ? 'border-emerald-200 bg-emerald-50' : 'border-line bg-white';
  return <div className={`rounded-md border px-3 py-2 ${color}`}><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 text-lg font-semibold text-ink">{value}</div></div>;
}

function InfoRow({ label, value }: { label: string; value: ReactNode }) {
  return <div className="flex items-start justify-between gap-3"><dt className="text-slate-500">{label}</dt><dd className="max-w-[65%] text-right font-medium text-ink">{value}</dd></div>;
}

function categoryLabel(value: string) {
  return ({ quality: 'Chất lượng', operation: 'Vận hành', as_built: 'Hồ sơ hoàn công', warranty: 'Bảo hành', finance: 'Tài chính', legal: 'Pháp lý', other: 'Khác' } as Record<string, string>)[value] ?? value;
}

function resultLabel(value?: string) {
  if (!value) return '-';
  return value === 'conditional_pass' ? 'Đạt có điều kiện' : value === 'pass' ? 'Đạt' : value;
}

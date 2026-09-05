import { Bell, History, MessageSquare } from 'lucide-react';
import type { ProjectUpdateKind, ProjectUpdateSummary } from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { notificationRelativeTime } from '../notifications/notificationPresentation';
import { activityLabel } from '../work-items/workItemPresentation';
import { projectUpdateCount, projectUpdateGroups } from './projectUpdates';

const icons = { notification: Bell, discussion: MessageSquare, system: History };

export function ProjectUpdateIndicators({ summary, loading, error, onRetry, onOpen }: {
  summary?: ProjectUpdateSummary;
  loading: boolean;
  error: boolean;
  onRetry: () => void;
  onOpen: (kind: ProjectUpdateKind) => void;
}) {
  if (error) return <p className="mt-2 text-xs text-amber-700">Chưa tải được cập nhật <button type="button" className="font-semibold underline focus-ring" onClick={onRetry}>Thử lại</button></p>;
  if (loading) return <p className="mt-2 animate-pulse text-xs text-slate-500" role="status">Đang tải cập nhật…</p>;
  if (!summary) return <p className="mt-2 text-xs text-slate-500">Không có quyền xem cập nhật</p>;
  const groups = projectUpdateGroups.filter((group) => summary[group.count] > 0);
  const latest = summary.latest;
  const transition = latest?.from_status && latest.to_status && latest.from_status !== latest.to_status
    ? `${latest.target_label ? `${latest.target_label}: ` : ''}${statusLabel(latest.from_status)} → ${statusLabel(latest.to_status)}`
    : latest?.comment || (latest?.activity_type ? activityLabel(latest.activity_type) : 'Cập nhật dự án');
  return (
    <div className="mt-2 min-w-0 space-y-1.5">
      <div className="flex flex-wrap gap-1.5" aria-label="Cập nhật công trình">
        {groups.length ? groups.map(({ kind, label, count }) => {
          const Icon = icons[kind];
          return <button key={kind} type="button" className="inline-flex items-center gap-1 rounded-md border border-blue-100 bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-100 focus-ring" aria-label={`${label}: ${summary[count]} chưa đọc`} onClick={() => onOpen(kind)}><Icon className="h-3.5 w-3.5" aria-hidden="true" />{label} <span className="tabular-nums">{projectUpdateCount(summary[count])}</span></button>;
        }) : <button type="button" className="text-xs text-slate-500 hover:text-blue-700 focus-ring" onClick={() => onOpen(latest?.kind ?? 'notification')}>Không có cập nhật mới · Xem lịch sử</button>}
      </div>
      {latest ? <button type="button" className="block max-w-full truncate text-left text-xs text-slate-500 hover:text-blue-700 focus-ring" title={`${transition} · ${latest.created_at}`} onClick={() => onOpen(latest.kind)}>Gần nhất: {transition} · {notificationRelativeTime(latest.created_at)}</button> : null}
    </div>
  );
}

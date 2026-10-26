import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { ClipboardList, ChevronDown, MessageSquareText } from 'lucide-react';
import { IconAction } from '../../components/IconAction';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { StatusBadge } from '../../components/StatusBadge';
import { dateTimeText } from '../../lib/utils/format';
import type { WorkItem } from '../../lib/types/domain';
import { discussionWorkActionLabel, discussionWorkPath } from './discussionWorkDomain';

export function DiscussionWorkRail({ projectId, onOpen, onDiscuss }: { projectId: string; onOpen: (item: WorkItem) => void; onDiscuss: (item: WorkItem) => void }) {
  const { user } = useAuth();
  const [expanded, setExpanded] = useState(false);
  const work = useQuery({
    queryKey: ['my-work-items', 'discussion', user?.id, projectId],
    queryFn: () => get<{ items: WorkItem[]; total: number }>(discussionWorkPath(projectId)),
    enabled: Boolean(user?.id && projectId), retry: false, refetchInterval: 60_000,
  });
  return <section className="rounded-lg border border-line bg-white p-3 shadow-panel" aria-label="Việc của tôi trong công trình">
    <button type="button" className="flex w-full items-center gap-2 text-left font-semibold text-ink xl:cursor-default" aria-expanded={expanded} onClick={() => setExpanded(value => !value)}>
      <ClipboardList className="h-4 w-4 text-blue-600" /><span className="flex-1">Việc của tôi</span>
      {work.data ? <span className="rounded bg-blue-50 px-2 text-sm text-blue-700">{work.data.total}</span> : null}
      <ChevronDown className="h-4 w-4 xl:hidden" />
    </button>
    <div className={`${expanded ? 'block' : 'hidden'} mt-3 xl:block`}>
      {work.isPending ? <p role="status" className="text-sm text-slate-500">Đang tải việc của bạn…</p> : null}
      {work.isError ? <div role="alert" className="text-sm text-rose-700">Không tải được công việc. <button className="font-semibold underline" onClick={() => void work.refetch()}>Thử lại</button></div> : null}
      {!work.isError && work.data?.total === 0 ? <p className="text-sm text-slate-500">Chưa có việc cần xử lý.</p> : null}
      <div className="divide-y divide-line">{work.data?.items.map(item => <div key={item.id} className="space-y-2 py-3 first:pt-0">
        <p className="break-words text-sm font-semibold text-ink">{item.title}</p>
        <StatusBadge status={item.status} />
        {item.due_at ? <p className="text-xs text-slate-500">Hạn: {dateTimeText(item.due_at)}</p> : null}
        <div className="flex flex-wrap gap-2">
          <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-xs font-semibold text-white hover:bg-blue-700" onClick={() => onOpen(item)}>{discussionWorkActionLabel(item)}</button>
          <IconAction label={`Xem trao đổi: ${item.title}`} icon={<MessageSquareText className="h-4 w-4" />} onClick={() => onDiscuss(item)} />
        </div>
      </div>)}</div>
      <Link className="mt-3 block border-t border-line pt-3 text-sm font-semibold text-blue-700" to={`/my-work?project=${encodeURIComponent(projectId)}`}>Xem tất cả việc</Link>
    </div>
  </section>;
}

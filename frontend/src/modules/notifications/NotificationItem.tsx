import { Archive, CircleCheck } from 'lucide-react';
import clsx from 'clsx';
import type { UserNotification } from '../../lib/types/domain';
import { notificationDisplayText, notificationPresentation, notificationRelativeTime } from './notificationPresentation';

export function NotificationItem({
  notification,
  busy = false,
  compact = false,
  onOpen,
  onArchive,
}: {
  notification: UserNotification;
  busy?: boolean;
  compact?: boolean;
  onOpen: () => void;
  onArchive?: () => void;
}) {
  const presentation = notificationPresentation(notification);
  const { Icon } = presentation;
  return (
    <article className={clsx('group flex items-start gap-3 border-b border-line last:border-0', compact ? 'px-4 py-3' : 'px-4 py-4 sm:px-5', !notification.read_at && 'bg-blue-50/60')}>
      <span className={clsx('mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full', presentation.className)}>
        <Icon className="h-4 w-4" aria-hidden="true" />
      </span>
      <button type="button" className="min-w-0 flex-1 text-left disabled:opacity-60" disabled={busy} onClick={onOpen}>
        <span className="flex items-start gap-2">
          <strong className="min-w-0 flex-1 text-sm text-ink">{notificationDisplayText(notification.title_snapshot)}</strong>
          {!notification.read_at ? <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-blue-600" aria-label="Chưa đọc" /> : null}
        </span>
        <span className="mt-1 block text-sm leading-5 text-slate-600">{notificationDisplayText(notification.message_snapshot)}</span>
        <span className="mt-1.5 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs text-slate-400">
          <span className="truncate">{[notification.project_code, presentation.label].filter(Boolean).join(' · ')}</span>
          <time dateTime={notification.created_at}>{notificationRelativeTime(notification.created_at)}</time>
        </span>
        {notification.intent === 'action_required' ? (
          <span className={clsx('mt-2 inline-flex items-center gap-1 rounded-full px-2 py-1 text-[11px] font-semibold', notification.resolved_at ? 'bg-slate-100 text-slate-600' : 'bg-amber-50 text-amber-700')}>
            <CircleCheck className="h-3.5 w-3.5" />{notification.resolved_at ? 'Đã xử lý' : 'Cần xử lý'}
          </span>
        ) : null}
      </button>
      {onArchive ? (
        <button type="button" className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-slate-400 opacity-100 hover:bg-slate-100 hover:text-slate-700 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100" aria-label="Lưu trữ thông báo" disabled={busy} onClick={onArchive}>
          <Archive className="h-4 w-4" />
        </button>
      ) : null}
    </article>
  );
}

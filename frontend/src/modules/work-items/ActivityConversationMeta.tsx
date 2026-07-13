import { useState } from 'react';
import { SmilePlus } from 'lucide-react';
import clsx from 'clsx';
import type { ActivityReactionSummary, ActivityReplyPreview } from '../../lib/types/domain';
import { activityLabel } from './workItemPresentation';

export const activityReactionOptions: ActivityReactionSummary['reaction'][] = ['👍', '👀', '🙏', '🎉'];

export function ActivityReplyQuote({ replyTo, onOpen }: { replyTo?: ActivityReplyPreview; onOpen?: (id: string) => void }) {
  if (!replyTo) return null;
  const body = (
    <>
      <span className="block truncate text-[11px] font-semibold text-slate-600">↩ {replyTo.actor_name ?? 'Hệ thống'}</span>
      <span className="mt-0.5 block line-clamp-2 text-xs leading-5 text-slate-500">
        {replyTo.withdrawn ? 'Tin nhắn đã được thu hồi' : replyTo.comment || activityLabel(replyTo.activity_type)}
      </span>
    </>
  );
  if (onOpen) {
    return (
      <button type="button" className="mb-2 block w-full rounded-md border-l-2 border-blue-300 bg-white/70 px-2 py-1.5 text-left hover:bg-white" onClick={() => onOpen(replyTo.id)}>
        {body}
      </button>
    );
  }
  return <div className="mb-2 rounded-md border-l-2 border-blue-300 bg-slate-50 px-2 py-1.5">{body}</div>;
}

export function ActivityReactionBar({
  reactions = [],
  pendingReaction,
  onToggle,
}: {
  reactions?: ActivityReactionSummary[];
  pendingReaction?: ActivityReactionSummary['reaction'];
  onToggle?: (reaction: ActivityReactionSummary['reaction'], active: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-wrap items-center gap-1">
      {reactions.map((item) => (
        <button
          key={item.reaction}
          type="button"
          className={clsx(
            'rounded-full border px-2 py-0.5 text-xs font-semibold transition',
            item.reacted_by_me ? 'border-blue-300 bg-blue-50 text-blue-700' : 'border-line bg-white/80 text-slate-600',
          )}
          disabled={!onToggle || pendingReaction === item.reaction}
          onClick={() => onToggle?.(item.reaction, !item.reacted_by_me)}
          aria-label={`${item.reacted_by_me ? 'Gỡ' : 'Thêm'} phản ứng ${item.reaction}`}
        >
          {item.reaction} {pendingReaction === item.reaction ? '…' : item.count}
        </button>
      ))}
      {onToggle ? (
        <div className="relative">
          <button
            type="button"
            className={clsx(
              'inline-flex h-7 w-7 items-center justify-center rounded-full border text-blue-700 shadow-sm transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-300 focus-visible:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-50',
              open ? 'border-blue-200 bg-blue-50 ring-2 ring-white/70' : 'border-slate-200 bg-white/95 hover:border-blue-200 hover:bg-white',
            )}
            disabled={Boolean(pendingReaction)}
            onClick={() => setOpen((value) => !value)}
            aria-label="Thêm phản ứng"
            aria-expanded={open}
          >
            <SmilePlus className="h-4 w-4" />
          </button>
          {open ? (
            <div className="absolute bottom-8 left-0 z-20 flex gap-1 rounded-full border border-line bg-white p-1.5 shadow-lg">
              {activityReactionOptions.map((reaction) => {
                const current = reactions.find((item) => item.reaction === reaction);
                return (
                  <button
                    key={reaction}
                    type="button"
                    className="rounded-full p-1.5 text-base hover:bg-slate-100 disabled:opacity-50"
                    disabled={pendingReaction === reaction}
                    onClick={() => {
                      onToggle(reaction, !current?.reacted_by_me);
                      setOpen(false);
                    }}
                    aria-label={`${current?.reacted_by_me ? 'Gỡ' : 'Thêm'} phản ứng ${reaction}`}
                  >
                    {reaction}
                  </button>
                );
              })}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

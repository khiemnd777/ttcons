import { Paperclip } from 'lucide-react';
import type { ActivityAttachmentSummary } from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { asArray, attachmentTypeLabel } from './workItemPresentation';

export function ActivityAttachments({ attachments, compact = false }: { attachments?: ActivityAttachmentSummary[]; compact?: boolean }) {
  const rows = asArray<ActivityAttachmentSummary>(attachments);
  if (!rows.length) return null;

  return (
    <div className="mt-2 flex flex-wrap gap-1">
      {rows.map((attachment) => {
        const label = attachment.label || attachment.id;
        const body = (
          <>
            <Paperclip className="h-3 w-3 shrink-0" />
            <span className="truncate">{attachmentTypeLabel(attachment.type)}: {label}</span>
            {!compact && attachment.status ? <span className="shrink-0 text-slate-400">· {statusLabel(attachment.status)}</span> : null}
          </>
        );
        const className = "inline-flex max-w-full items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700";
        if (attachment.file_url) {
          return (
            <a key={`${attachment.type}:${attachment.id}`} className={className} href={attachment.file_url} target="_blank" rel="noreferrer">
              {body}
            </a>
          );
        }
        return (
          <span key={`${attachment.type}:${attachment.id}`} className={className}>
            {body}
          </span>
        );
      })}
    </div>
  );
}

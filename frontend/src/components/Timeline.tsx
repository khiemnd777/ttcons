import { EnumChip } from './EnumChip';
import { EmptyState } from './EmptyState';
import { IdentityReferenceLabel } from './IdentityReferenceLabel';

export function Timeline({ events, onSelect }: { events: Record<string, any>[]; onSelect?: (event: Record<string, any>) => void }) {
  if (!events.length) return <EmptyState title="Chưa có sự kiện audit" />;
  return (
    <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="space-y-4">
        {events.map((event) => (
          <div key={event.id} className="border-l-2 border-blue-200 pl-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <EnumChip group="event_type" value={event.event_type} />
              <span className="text-xs text-slate-500">{timelineDateTime(event.created_at)}</span>
            </div>
            <p className="mt-1 text-sm text-slate-600">{event.description}</p>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
              {event.actor_name ? <span>Người thực hiện: <IdentityReferenceLabel name={event.actor_name} deleted={event.actor_account_deleted} /></span> : null}
              {event.target_type ? <EnumChip group="target_type" value={event.target_type} /> : null}
              {onSelect ? (
                <button className="ml-auto font-semibold text-blue-700 hover:underline" onClick={() => onSelect(event)} type="button">
                  Xem chi tiết
                </button>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function timelineDateTime(value: unknown) {
  if (!value) return '-';
  const date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat('vi-VN', {
    hour: '2-digit',
    minute: '2-digit',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).format(date);
}

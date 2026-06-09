import { dateText } from '../lib/utils/format';
import { EmptyState } from './EmptyState';

export function Timeline({ events }: { events: Record<string, any>[] }) {
  if (!events.length) return <EmptyState title="Chưa có sự kiện audit" />;
  return (
    <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="space-y-4">
        {events.map((event) => (
          <div key={event.id} className="border-l-2 border-blue-200 pl-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-semibold text-ink">{event.event_type}</p>
              <span className="text-xs text-slate-500">{dateText(event.created_at)}</span>
            </div>
            <p className="mt-1 text-sm text-slate-600">{event.description}</p>
            {event.actor_name ? <p className="mt-1 text-xs text-slate-500">Actor: {event.actor_name}</p> : null}
          </div>
        ))}
      </div>
    </div>
  );
}

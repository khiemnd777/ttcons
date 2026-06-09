import { Check, X } from 'lucide-react';
import { StatusBadge } from './StatusBadge';

export function ApprovalPanel({
  request,
  onApprove,
  onReject,
}: {
  request?: Record<string, any>;
  onApprove?: () => void;
  onReject?: () => void;
}) {
  if (!request) return null;
  return (
    <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-ink">Yêu cầu duyệt</h3>
          <p className="mt-1 text-sm text-slate-600">{request.reason}</p>
        </div>
        <StatusBadge status={request.status} />
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <button className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700" onClick={onApprove}>
          <Check className="h-4 w-4" />
          Duyệt
        </button>
        <button className="inline-flex items-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700" onClick={onReject}>
          <X className="h-4 w-4" />
          Từ chối
        </button>
      </div>
    </div>
  );
}

import { Trash2, X } from 'lucide-react';

export function BulkSelectionBar({
  count,
  itemLabel,
  actionLabel,
  onClear,
  onDelete,
}: {
  count: number;
  itemLabel: string;
  actionLabel: string;
  onClear: () => void;
  onDelete: () => void;
}) {
  if (!count) return null;
  return (
    <div className="fixed inset-x-3 bottom-[calc(0.75rem+env(safe-area-inset-bottom))] z-30 flex flex-col gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 shadow-xl sm:static sm:mb-3 sm:flex-row sm:items-center sm:justify-between sm:shadow-none">
      <div className="flex items-center gap-2 text-sm font-semibold text-blue-900">
        <button type="button" className="rounded-md p-1 text-blue-700 hover:bg-blue-100" aria-label="Bỏ chọn tất cả" onClick={onClear}>
          <X className="h-4 w-4" />
        </button>
        Đã chọn {count} {itemLabel}
      </div>
      <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700" onClick={onDelete}>
        <Trash2 className="h-4 w-4" />{actionLabel}
      </button>
    </div>
  );
}

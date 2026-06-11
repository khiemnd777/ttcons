import { ReactNode } from 'react';
import { Eye, Pencil } from 'lucide-react';

export function RowActions({
  onView,
  onEdit,
  children,
}: {
  onView?: () => void;
  onEdit?: () => void;
  children?: ReactNode;
}) {
  return (
    <div className="flex items-center gap-2">
      {onView ? (
        <button title="Xem chi tiết" aria-label="Xem chi tiết" className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100" onClick={onView}>
          <Eye className="h-4 w-4" />
        </button>
      ) : null}
      {onEdit ? (
        <button title="Chỉnh sửa" aria-label="Chỉnh sửa" className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100" onClick={onEdit}>
          <Pencil className="h-4 w-4" />
        </button>
      ) : null}
      {children}
    </div>
  );
}

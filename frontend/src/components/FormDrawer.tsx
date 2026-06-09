import { ReactNode } from 'react';
import { X } from 'lucide-react';

export function FormDrawer({
  open,
  title,
  children,
  onClose,
}: {
  open: boolean;
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-40">
      <button className="absolute inset-0 bg-ink/30" aria-label="Đóng drawer" onClick={onClose} />
      <aside className="absolute right-0 top-0 h-full w-full max-w-xl overflow-auto border-l border-line bg-white shadow-2xl">
        <div className="sticky top-0 z-10 flex items-center justify-between border-b border-line bg-white px-5 py-4">
          <h2 className="text-lg font-semibold text-ink">{title}</h2>
          <button className="rounded-md p-2 text-slate-500 hover:bg-slate-100" onClick={onClose} aria-label="Đóng">
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="p-5">{children}</div>
      </aside>
    </div>
  );
}

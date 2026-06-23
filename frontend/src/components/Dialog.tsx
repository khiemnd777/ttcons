import { ReactNode, useEffect } from 'react';
import { X } from 'lucide-react';
import clsx from 'clsx';

export type DialogSize = 'sm' | 'md' | 'lg' | 'xl';

const sizeClass: Record<DialogSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
};

export function Dialog({
  open,
  title,
  description,
  children,
  footer,
  onClose,
  size = 'md',
}: {
  open: boolean;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  size?: DialogSize;
}) {
  useEffect(() => {
    if (!open) return undefined;
    const previousOverflow = document.body.style.overflow;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.body.style.overflow = 'hidden';
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-40 grid place-items-center p-4">
      <button className="absolute inset-0 bg-ink/35" aria-label="Đóng dialog" onClick={onClose} />
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialog-title"
        className={clsx('relative flex max-h-[calc(100vh-2rem)] w-full flex-col overflow-hidden rounded-lg border border-line bg-white shadow-2xl', sizeClass[size])}
      >
        <div className="flex items-start justify-between gap-4 border-b border-line px-5 py-4">
          <div className="min-w-0">
            <h2 id="dialog-title" className="text-lg font-semibold text-ink">{title}</h2>
            {description ? <p className="mt-1 text-sm leading-5 text-slate-600">{description}</p> : null}
          </div>
          <button className="rounded-md p-2 text-slate-500 hover:bg-slate-100" onClick={onClose} aria-label="Đóng">
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="min-h-0 overflow-y-auto p-5">{children}</div>
        {footer ? <div className="border-t border-line bg-slate-50 px-5 py-3">{footer}</div> : null}
      </section>
    </div>
  );
}

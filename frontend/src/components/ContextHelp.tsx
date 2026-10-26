import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Info } from 'lucide-react';

/** Optional explanations only; required instructions and warnings stay visible. */
export function ContextHelp({ label, children }: { label: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const container = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const dismissOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target)) setOpen(false);
    };
    const dismissEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return;
      const dialogs = document.querySelectorAll('[role="dialog"][aria-modal="true"]');
      const topDialog = dialogs[dialogs.length - 1];
      if (topDialog && !topDialog.contains(container.current)) return;
      // Close this disclosure before an enclosing dialog handles Escape.
      event.preventDefault();
      event.stopPropagation();
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener('pointerdown', dismissOutside);
    window.addEventListener('keydown', dismissEscape, true);
    return () => {
      document.removeEventListener('pointerdown', dismissOutside);
      window.removeEventListener('keydown', dismissEscape, true);
    };
  }, [open]);

  return (
    <div ref={container} className="min-w-0 max-w-md" onBlur={event => {
      if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) setOpen(false);
    }}>
      <button ref={trigger} type="button" aria-label={label} title={label} aria-expanded={open} aria-controls={id}
        className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        onClick={() => setOpen(value => !value)}>
        <Info className="h-4 w-4" aria-hidden="true" />
      </button>
      <div id={id} hidden={!open} className="mt-1 rounded-md border border-blue-100 bg-blue-50 p-3 text-sm font-normal leading-6 text-slate-700">
        {children}
      </div>
    </div>
  );
}

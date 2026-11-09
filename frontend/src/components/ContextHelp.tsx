import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Info } from 'lucide-react';

/** Optional explanations only; required instructions and warnings stay visible. */
export function ContextHelp({ label, children, tooltip = false }: { label: string; children: ReactNode; tooltip?: boolean }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const container = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({ left: 12, top: 12 });
  const pinned = useRef(false);
  const dismissTimer = useRef<ReturnType<typeof setTimeout>>();
  const Container = tooltip ? 'span' : 'div';
  const show = () => { clearTimeout(dismissTimer.current); setOpen(true); };
  const dismiss = () => { clearTimeout(dismissTimer.current); pinned.current = false; setOpen(false); };
  const leave = () => { if (!pinned.current) dismissTimer.current = setTimeout(dismiss, 180); };
  useEffect(() => () => clearTimeout(dismissTimer.current), []);
  useEffect(() => {
    if (!tooltip || !open) return;
    const place = () => {
      const r = trigger.current?.getBoundingClientRect();
      const p = panel.current?.getBoundingClientRect();
      if (!r || !p) return;
      setPosition({ left: Math.max(12, Math.min(r.left, window.innerWidth - p.width - 12)), top: Math.max(12, r.bottom + 8 + p.height > window.innerHeight ? r.top - p.height - 8 : r.bottom + 8) });
    };
    place();
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', place, true); };
  }, [tooltip, open, children]);

  useEffect(() => {
    if (!open) return;
    const dismissOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target) && !panel.current?.contains(event.target)) dismiss();
    };
    const dismissEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return;
      const dialogs = document.querySelectorAll('[role="dialog"][aria-modal="true"]');
      const topDialog = dialogs[dialogs.length - 1];
      if (topDialog && !topDialog.contains(container.current)) return;
      // Close this disclosure before an enclosing dialog handles Escape.
      event.preventDefault();
      event.stopPropagation();
      dismiss();
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
    <Container ref={container} className={tooltip ? 'inline-flex align-middle' : 'min-w-0 max-w-md'} onBlur={event => {
      if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) dismiss();
    }}>
      <button ref={trigger} type="button" aria-label={label} title={tooltip ? undefined : label} aria-expanded={open} aria-controls={id} aria-describedby={tooltip && open ? id : undefined}
        className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        onPointerEnter={tooltip ? event => { if (event.pointerType === 'mouse') show(); } : undefined}
        onPointerLeave={tooltip ? leave : undefined} onFocus={tooltip ? show : undefined}
        onClick={() => { if (!tooltip) setOpen(value => !value); else if (pinned.current) dismiss(); else { pinned.current = true; show(); } }}>
        <Info className="h-4 w-4" aria-hidden="true" />
      </button>
      {tooltip ? open && createPortal(<div ref={panel} id={id} role="tooltip" onPointerEnter={show} onPointerLeave={leave}
        style={{ position: 'fixed', ...position, zIndex: 100, maxWidth: 'min(360px, calc(100vw - 24px))', maxHeight: 'calc(100vh - 24px)', overflowY: 'auto' }}
        className="rounded-md border border-blue-100 bg-blue-50 p-3 text-sm font-normal leading-6 text-slate-700 shadow-lg">{children}</div>, document.body) : <div id={id} hidden={!open} className="mt-1 rounded-md border border-blue-100 bg-blue-50 p-3 text-sm font-normal leading-6 text-slate-700">
        {children}
      </div>}
    </Container>
  );
}

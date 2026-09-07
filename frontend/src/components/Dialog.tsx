import { ReactNode, useEffect, useId, useRef } from 'react';
import { X } from 'lucide-react';
import clsx from 'clsx';
import './dialog.css';

export type DialogSize = 'sm' | 'md' | 'lg' | 'xl' | 'report' | 'workspace';

const sizeClass: Record<DialogSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-[min(800px,calc(100vw-2rem))]',
  xl: 'max-w-[min(1120px,calc(100vw-2rem))]',
  report: 'max-w-[min(1320px,calc(100vw-2rem))]',
  workspace: 'max-w-[min(1480px,calc(100vw-2rem))]',
};

export function Dialog({
  open,
  title,
  description,
  children,
  footer,
  onClose,
  size = 'md',
  manageFocus = false,
  mobileFullscreen = false,
}: {
  open: boolean;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  size?: DialogSize;
  manageFocus?: boolean;
  mobileFullscreen?: boolean;
}) {
  const titleId = useId();
  const dialogRef = useRef<HTMLElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  useEffect(() => {
    if (!open) return undefined;
    const previousOverflow = document.body.style.overflow;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    let focusFrame = 0;
    const onKeyDown = (event: KeyboardEvent) => {
      // A nested source/evidence dialog owns keyboard input while it is open.
      const dialogs = document.querySelectorAll('[role="dialog"][aria-modal="true"]');
      if (dialogs[dialogs.length - 1] !== dialogRef.current) return;
      if (event.key === 'Escape') {
        if (manageFocus) {
          event.preventDefault();
          event.stopImmediatePropagation();
        }
        closeRef.current();
      }
      if (!manageFocus || event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [tabindex]:not([tabindex="-1"])'))
        .filter((element) => element.getClientRects().length > 0);
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first) {
        event.preventDefault();
        dialogRef.current.focus();
      } else if (event.shiftKey && (document.activeElement === first || !dialogRef.current.contains(document.activeElement))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && (document.activeElement === last || !dialogRef.current.contains(document.activeElement))) {
        event.preventDefault();
        first.focus();
      }
    };
    document.body.style.overflow = 'hidden';
    document.addEventListener('keydown', onKeyDown, manageFocus);
    if (manageFocus) focusFrame = window.requestAnimationFrame(() => {
      const dialogs = document.querySelectorAll('[role="dialog"][aria-modal="true"]');
      if (dialogs[dialogs.length - 1] === dialogRef.current) dialogRef.current?.querySelector<HTMLElement>('[data-dialog-close]')?.focus();
    });
    return () => {
      window.cancelAnimationFrame(focusFrame);
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', onKeyDown, manageFocus);
      if (manageFocus && previousFocus?.isConnected) previousFocus.focus();
    };
  }, [open, manageFocus]);

  useEffect(() => {
    if (!open || !mobileFullscreen) return;
    const viewport = window.visualViewport;
    const updateViewport = () => {
      viewportRef.current?.style.setProperty('--dialog-viewport-height', `${viewport?.height ?? window.innerHeight}px`);
      viewportRef.current?.style.setProperty('--dialog-viewport-top', `${viewport?.offsetTop ?? 0}px`);
    };
    updateViewport();
    viewport?.addEventListener('resize', updateViewport);
    viewport?.addEventListener('scroll', updateViewport);
    window.addEventListener('resize', updateViewport);
    return () => {
      viewport?.removeEventListener('resize', updateViewport);
      viewport?.removeEventListener('scroll', updateViewport);
      window.removeEventListener('resize', updateViewport);
    };
  }, [open, mobileFullscreen]);

  if (!open) return null;

  return (
    <div ref={viewportRef} className={clsx('fixed inset-0 z-40 grid place-items-center p-4', mobileFullscreen && 'dialog-mobile-fullscreen')}>
      <button type="button" className="absolute inset-0 bg-ink/35" aria-label="Đóng dialog" onClick={onClose} />
      <section
        ref={dialogRef}
        tabIndex={manageFocus ? -1 : undefined}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={clsx('dialog-surface relative flex max-h-[calc(100vh-2rem)] w-full flex-col overflow-hidden rounded-lg border border-line bg-white shadow-2xl', sizeClass[size])}
      >
        <div className="dialog-header flex items-start justify-between gap-4 border-b border-line px-5 py-4">
          <div className="min-w-0">
            <h2 id={titleId} className="text-lg font-semibold text-ink">{title}</h2>
            {description ? <p className="mt-1 text-sm leading-5 text-slate-600">{description}</p> : null}
          </div>
          <button type="button" data-dialog-close className="rounded-md p-2 text-slate-500 hover:bg-slate-100" onClick={onClose} aria-label="Đóng">
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="dialog-body min-h-0 overflow-y-auto p-5">{children}</div>
        {footer ? <div className="dialog-footer border-t border-line bg-slate-50 px-5 py-3">{footer}</div> : null}
      </section>
    </div>
  );
}

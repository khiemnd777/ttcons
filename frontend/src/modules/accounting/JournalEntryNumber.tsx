import { type KeyboardEvent as ReactKeyboardEvent, useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Copy } from 'lucide-react';
import { useToast } from '../../components/ToastProvider';
import { copyJournalEntryNumber, isJournalEntryCopyKey } from './journalEntryClipboard';
import { journalEntryNoPresentation } from './journalEntryPresentation';

type TooltipPosition = {
  top: number;
  left: number;
  maxWidth: number;
};

export function JournalEntryNumber({ value, className = 'font-semibold text-ink' }: { value: unknown; className?: string }) {
  const presentation = journalEntryNoPresentation(value);
  const toast = useToast();
  const tooltipId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<TooltipPosition>({ top: 0, left: 0, maxWidth: 384 });

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger || typeof window === 'undefined') return;
    const rect = trigger.getBoundingClientRect();
    const viewportGap = 8;
    const maxWidth = Math.min(384, Math.max(0, window.innerWidth - viewportGap * 2));
    const tooltipHeight = tooltipRef.current?.offsetHeight ?? 64;
    const belowTop = rect.bottom + viewportGap;
    const canPlaceBelow = belowTop + tooltipHeight <= window.innerHeight - viewportGap;
    const top = canPlaceBelow ? belowTop : Math.max(viewportGap, rect.top - tooltipHeight - viewportGap);
    const left = Math.min(Math.max(viewportGap, rect.left), Math.max(viewportGap, window.innerWidth - maxWidth - viewportGap));
    setPosition({ top, left, maxWidth });
  }, []);

  useLayoutEffect(() => {
    if (!open) return;
    updatePosition();
  }, [open, updatePosition]);

  useEffect(() => {
    if (!open) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setOpen(false);
      triggerRef.current?.blur();
    };
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && triggerRef.current?.contains(target)) return;
      setOpen(false);
      triggerRef.current?.blur();
    };
    window.addEventListener('resize', updatePosition);
    window.addEventListener('scroll', updatePosition, true);
    document.addEventListener('keydown', closeOnEscape);
    document.addEventListener('pointerdown', closeOnOutsidePointer, true);
    return () => {
      window.removeEventListener('resize', updatePosition);
      window.removeEventListener('scroll', updatePosition, true);
      document.removeEventListener('keydown', closeOnEscape);
      document.removeEventListener('pointerdown', closeOnOutsidePointer, true);
    };
  }, [open, updatePosition]);

  const copyFullNumber = useCallback(async () => {
    try {
      if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
        throw new Error('Trình duyệt chưa cấp quyền truy cập clipboard.');
      }
      const copied = await copyJournalEntryNumber(presentation.full, (text) => navigator.clipboard.writeText(text));
      toast.success({ title: 'Đã sao chép số CTGS', description: copied });
    } catch (error) {
      toast.error({
        title: 'Không sao chép được số CTGS',
        description: 'Trình duyệt không cho phép ghi vào clipboard. Vui lòng thử lại.',
      }, error);
    }
  }, [presentation.full, toast]);

  const copyOnKeyboard = useCallback((event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (!isJournalEntryCopyKey(event.key)) return;
    event.preventDefault();
    void copyFullNumber();
  }, [copyFullNumber]);

  if (!presentation.full) {
    return <span className={`inline-block max-w-full whitespace-nowrap ${className}`} aria-label="Chưa có số CTGS">{presentation.display}</span>;
  }

  const copyContent = (
    <>
      <span>{presentation.display}</span>
      <Copy className="h-3.5 w-3.5 shrink-0 opacity-60" aria-hidden="true" />
    </>
  );

  if (!presentation.compacted) {
    return (
      <button
        type="button"
        className={`inline-flex max-w-full cursor-pointer items-center gap-1 whitespace-nowrap rounded-sm border-b border-dotted border-slate-400 bg-transparent p-0 text-left focus:outline-none focus:ring-2 focus:ring-blue-200 ${className}`}
        aria-label={`Số CTGS: ${presentation.full}. Bấm để sao chép`}
        data-ctgs-copy-trigger="true"
        onClick={() => void copyFullNumber()}
        onKeyDown={copyOnKeyboard}
      >
        {copyContent}
      </button>
    );
  }

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={`inline-flex max-w-full cursor-pointer items-center gap-1 whitespace-nowrap rounded-sm border-b border-dotted border-slate-400 bg-transparent p-0 text-left focus:outline-none focus:ring-2 focus:ring-blue-200 ${className}`}
        aria-label={`Số CTGS: ${presentation.full}. Bấm để sao chép`}
        aria-describedby={open ? tooltipId : undefined}
        data-ctgs-copy-trigger="true"
        data-ctgs-tooltip-trigger="true"
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => {
          if (document.activeElement !== triggerRef.current) setOpen(false);
        }}
        onPointerUp={(event) => {
          if (event.pointerType === 'mouse') return;
          triggerRef.current?.focus({ preventScroll: true });
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onClick={() => void copyFullNumber()}
        onKeyDown={copyOnKeyboard}
      >
        {copyContent}
      </button>
      {open && typeof document !== 'undefined' ? createPortal(
        <div
          ref={tooltipRef}
          id={tooltipId}
          role="tooltip"
          data-ctgs-tooltip="true"
          className="pointer-events-none fixed z-[200] rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-left text-xs leading-5 text-white shadow-lg"
          style={{ top: position.top, left: position.left, maxWidth: position.maxWidth }}
        >
          <p className="text-[10px] font-semibold uppercase tracking-wide text-slate-300">Số CTGS đầy đủ</p>
          <p className="mt-0.5 break-all font-mono font-semibold text-white">{presentation.full}</p>
          <p className="mt-1 text-[11px] text-slate-300">Bấm để sao chép</p>
        </div>,
        document.body,
      ) : null}
    </>
  );
}

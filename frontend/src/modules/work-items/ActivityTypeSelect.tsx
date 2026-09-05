import { useEffect, useId, useRef, useState } from 'react';
import { BellRing, Check, ChevronDown, CircleHelp, FileText, ListChecks, MessageSquare, RefreshCw, ShieldAlert } from 'lucide-react';
import type { ActivityPayload } from '../../lib/types/domain';
import clsx from 'clsx';

type ActivityType = ActivityPayload['activity_type'];

const activityIcons = {
  comment: MessageSquare,
  status: RefreshCw,
  blocker: ShieldAlert,
  question: CircleHelp,
  decision: ListChecks,
  reminder: BellRing,
  file_note: FileText,
};

export function ActivityTypeSelect({ value, options, onChange, compact, disabled, onOpen }: {
  value: ActivityType;
  options: Array<{ value: ActivityType; label: string }>;
  onChange: (value: ActivityType) => void;
  compact: boolean;
  disabled: boolean;
  onOpen: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const id = useId();
  const Icon = activityIcons[value as keyof typeof activityIcons] ?? MessageSquare;
  const close = () => { setOpen(false); trigger.current?.focus(); };

  useEffect(() => {
    if (!open) return;
    menu.current?.focus();
    const outside = (event: PointerEvent) => {
      if (event.target instanceof Node && !root.current?.contains(event.target)) setOpen(false);
    };
    document.addEventListener('pointerdown', outside);
    return () => document.removeEventListener('pointerdown', outside);
  }, [open]);

  useEffect(() => { if (disabled) setOpen(false); }, [disabled]);

  return (
    <div ref={root} className="relative min-w-0" onBlur={(event) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false);
    }}>
      <button ref={trigger} type="button" aria-label="Loại trao đổi" aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? id : undefined}
        disabled={disabled} className={clsx('inline-flex max-w-full items-center gap-1 rounded-md bg-slate-100 font-semibold text-slate-600 hover:bg-slate-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500 disabled:opacity-60', compact ? 'h-7 px-1.5 text-[11px]' : 'px-2 py-1.5 text-xs')}
        onClick={() => { if (!open) onOpen(); setActive(Math.max(0, options.findIndex((option) => option.value === value))); setOpen((current) => !current); }}
        onKeyDown={(event) => {
          if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
          event.preventDefault();
          onOpen();
          setActive(Math.max(0, options.findIndex((option) => option.value === value)));
          setOpen(true);
        }}>
        <Icon aria-hidden="true" className="h-3.5 w-3.5 shrink-0" /><span className="truncate">{options.find((option) => option.value === value)?.label}</span><ChevronDown aria-hidden="true" className="h-3 w-3 shrink-0" />
      </button>
      {open ? (
        <div ref={menu} id={id} role="listbox" tabIndex={-1} aria-label="Loại trao đổi" aria-activedescendant={`${id}-${active}`}
          className="absolute bottom-full left-0 z-30 mb-2 w-56 max-w-[calc(100vw-3rem)] rounded-lg border border-line bg-white p-1 shadow-xl outline-none"
          onKeyDown={(event) => {
            if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); }
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); setActive((current) => (current + (event.key === 'ArrowDown' ? 1 : options.length - 1)) % options.length); }
            if (event.key === 'Home' || event.key === 'End') { event.preventDefault(); setActive(event.key === 'Home' ? 0 : options.length - 1); }
            if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onChange(options[active].value); close(); }
          }}>
          {options.map((option, index) => {
            const OptionIcon = activityIcons[option.value as keyof typeof activityIcons] ?? MessageSquare;
            return <div key={option.value} id={`${id}-${index}`} role="option" aria-selected={option.value === value}
              className={clsx('flex cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-xs font-medium', active === index ? 'bg-blue-50 text-blue-700' : 'text-slate-700')}
              onMouseEnter={() => setActive(index)} onMouseDown={(event) => event.preventDefault()} onClick={() => { onChange(option.value); close(); }}>
              <OptionIcon aria-hidden="true" className="h-4 w-4 shrink-0" /><span className="flex-1">{option.label}</span>{option.value === value ? <Check aria-hidden="true" className="h-3.5 w-3.5" /> : null}
            </div>;
          })}
        </div>
      ) : null}
    </div>
  );
}

import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { AtSign, Loader2, Send, Users, X } from 'lucide-react';
import clsx from 'clsx';
import type { ActivityMentionPayload, ActivityPayload } from '../../lib/types/domain';
import { ActivityTypeSelect } from './ActivityTypeSelect';
import {
  chatKeyAction, chatMentionSuggestions, filterMentionMembers, inlineMentionPayload, insertInlineMention,
  mentionQueryAt, reconcileInlineMentions,
  type ActivityMentionOption, type ChatMentionOption, type InlineMention, type MentionQuery,
} from './activityChatInputDomain';

export function ActivityChatInput({
  value, onChange, options, optionsLoading = false, optionsError = false, onRetryOptions,
  activityType, activityOptions, onActivityTypeChange, compact, editing, replying,
  editUnchanged, busy, invalid, placeholder, onBlur,
}: {
  value: string;
  onChange: (value: string, mentions: ActivityMentionPayload[]) => void;
  options: ActivityMentionOption[];
  optionsLoading?: boolean;
  optionsError?: boolean;
  onRetryOptions?: () => void;
  activityType: ActivityPayload['activity_type'];
  activityOptions: Array<{ value: ActivityPayload['activity_type']; label: string }>;
  onActivityTypeChange: (value: ActivityPayload['activity_type']) => void;
  compact: boolean;
  editing: boolean;
  replying: boolean;
  editUnchanged: boolean;
  busy: boolean;
  invalid: boolean;
  placeholder: string;
  onBlur: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLTextAreaElement>(null);
  const composing = useRef(false);
  const draft = useRef(value);
  const mentions = useRef<InlineMention[]>([]);
  const selection = useRef({ start: value.length, end: value.length });
  const [query, setQuery] = useState<MentionQuery | null>(null);
  const [active, setActive] = useState(0);
  const id = useId();
  const candidates = chatMentionSuggestions(options, query?.query ?? '');
  const selectedIndex = Math.min(active, Math.max(0, candidates.length - 1));
  const canSelect = !optionsLoading && !optionsError && candidates.length > 0;

  useEffect(() => {
    // Success/reset/edit hydration is external; never infer recipients from loaded text.
    if (value !== draft.current) {
      draft.current = value;
      mentions.current = [];
      selection.current = { start: value.length, end: value.length };
      setQuery(null);
    }
  }, [value]);

  useEffect(() => {
    if (!input.current || !compact) return;
    input.current.style.height = 'auto';
    input.current.style.height = `${Math.min(Math.max(input.current.scrollHeight, 44), 112)}px`;
    input.current.style.overflowY = input.current.scrollHeight > 112 ? 'auto' : 'hidden';
  }, [value, compact]);

  useEffect(() => {
    if (!query) return;
    const outside = (event: PointerEvent) => {
      if (event.target instanceof Node && !root.current?.contains(event.target)) setQuery(null);
    };
    document.addEventListener('pointerdown', outside);
    return () => document.removeEventListener('pointerdown', outside);
  }, [Boolean(query)]);

  useEffect(() => {
    if (query && canSelect) document.getElementById(`${id}-option-${selectedIndex}`)?.scrollIntoView({ block: 'nearest' });
  }, [id, query, selectedIndex, canSelect]);

  function focusAt(caret: number) {
    selection.current = { start: caret, end: caret };
    requestAnimationFrame(() => {
      input.current?.focus();
      input.current?.setSelectionRange(caret, caret);
    });
  }

  function updateValue(next: string, start: number, end = start) {
    const nextMentions = reconcileInlineMentions(draft.current, next, mentions.current, start);
    draft.current = next;
    mentions.current = nextMentions;
    selection.current = { start, end };
    onChange(next, inlineMentionPayload(nextMentions, next));
    setActive(0);
    setQuery(editing || composing.current ? null : mentionQueryAt(next, start, end, nextMentions));
  }

  function selectMember(option: ChatMentionOption) {
    if (!query || busy || editing || !canSelect) return;
    const next = insertInlineMention(draft.current, query, option, mentions.current);
    draft.current = next.value;
    mentions.current = next.mentions;
    onChange(next.value, inlineMentionPayload(next.mentions, next.value));
    setQuery(null);
    focusAt(next.caret);
  }

  function openMentions() {
    if (query) { setQuery(null); input.current?.focus(); return; }
    const { start, end } = selection.current;
    const current = mentionQueryAt(value, start, end, mentions.current);
    if (current) { setQuery(current); setActive(0); focusAt(start); return; }
    const prefix = start && !/\s/.test(value[start - 1]) ? ' @' : '@';
    const next = value.slice(0, start) + prefix + value.slice(end);
    updateValue(next, start + prefix.length);
    focusAt(start + prefix.length);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    const action = chatKeyAction({
      key: event.key, shiftKey: event.shiftKey, ctrlKey: event.ctrlKey, altKey: event.altKey, metaKey: event.metaKey,
      isComposing: composing.current || event.nativeEvent.isComposing, keyCode: event.nativeEvent.keyCode,
    }, Boolean(query), busy);
    if (action === 'none') return;
    event.preventDefault();
    if (action === 'dismiss') { event.stopPropagation(); setQuery(null); }
    if (action === 'previous' || action === 'next') {
      if (canSelect) setActive((selectedIndex + (action === 'next' ? 1 : candidates.length - 1)) % candidates.length);
    }
    if (action === 'select' && canSelect) selectMember(candidates[selectedIndex]);
    if (action === 'newline') {
      const { selectionStart: start, selectionEnd: end } = event.currentTarget;
      updateValue(value.slice(0, start) + '\n' + value.slice(end), start + 1);
      setQuery(null);
      focusAt(start + 1);
    }
    if (action === 'submit' && !editUnchanged) event.currentTarget.form?.requestSubmit();
  }

  function refreshQuery() {
    if (!input.current || editing || composing.current) return;
    const { selectionStart, selectionEnd } = input.current;
    selection.current = { start: selectionStart, end: selectionEnd };
    setActive(0);
    setQuery(mentionQueryAt(value, selectionStart, selectionEnd, mentions.current));
  }

  return (
    <div ref={root} className="relative min-w-0" onBlur={(event) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setQuery(null);
    }} onKeyDown={(event) => {
      if (query && event.key === 'Escape' && !composing.current) { event.preventDefault(); event.stopPropagation(); setQuery(null); input.current?.focus(); }
    }}>
      {query && !editing ? (
        <div className="absolute bottom-full left-0 z-30 mb-2 w-80 max-w-full overflow-hidden rounded-lg border border-line bg-white text-left shadow-xl">
          <div className="flex items-center justify-between border-b border-line px-3 py-2">
            <span className="text-xs font-semibold text-slate-600">Nhắc đến thành viên</span>
            <button type="button" aria-label="Đóng gợi ý nhắc đến" className="rounded p-1 text-slate-500 hover:bg-slate-100" onClick={() => { setQuery(null); input.current?.focus(); }}><X className="h-3.5 w-3.5" /></button>
          </div>
          {optionsLoading ? <p role="status" className="flex items-center gap-2 px-3 py-3 text-xs text-slate-500"><Loader2 className="h-3.5 w-3.5 animate-spin" />Đang tải thành viên…</p> : optionsError ? (
            <div role="status" className="px-3 py-3 text-xs text-rose-700">Không tải được thành viên. {onRetryOptions ? <button type="button" className="font-semibold underline" onClick={onRetryOptions}>Thử lại</button> : null}</div>
          ) : !candidates.length ? <p role="status" className="px-3 py-3 text-xs text-slate-500">{filterMentionMembers(options, '').length ? 'Không tìm thấy thành viên phù hợp.' : 'Chưa có thành viên để nhắc đến.'}</p> : null}
          <div id={`${id}-members`} role="listbox" aria-label="Thành viên được nhắc đến" className="max-h-48 overflow-y-auto p-1">
            {canSelect ? candidates.map((option, index) => (
              <div key={option.id} id={`${id}-option-${index}`} role="option" aria-selected={selectedIndex === index}
                className={clsx('cursor-pointer rounded-md px-2 py-2 text-xs', selectedIndex === index ? 'bg-blue-50 text-blue-700' : 'text-slate-700')}
                onPointerDown={(event) => event.preventDefault()} onMouseEnter={() => setActive(index)} onClick={() => selectMember(option)}>
                <span className="flex items-center gap-1.5 break-words font-semibold">{option.type === 'all' ? <Users aria-hidden="true" className="h-3.5 w-3.5 shrink-0" /> : null}{option.type === 'all' ? '@all' : option.label}</span>
                {option.description ? <span className="mt-0.5 block break-words text-[11px] text-slate-500">{option.description}</span> : null}
              </div>
            )) : null}
          </div>
        </div>
      ) : null}
      <textarea ref={input} aria-label={editing ? 'Nội dung chỉnh sửa' : 'Nội dung trao đổi'}
        aria-invalid={invalid} aria-autocomplete={editing ? undefined : 'list'} aria-haspopup={editing ? undefined : 'listbox'}
        aria-controls={query ? `${id}-members` : undefined} aria-expanded={editing ? undefined : Boolean(query)}
        aria-activedescendant={query && canSelect ? `${id}-option-${selectedIndex}` : undefined} aria-describedby={`${id}-keys`}
        className={clsx('block w-full rounded-t-xl bg-transparent px-3 text-sm outline-none disabled:opacity-60', compact ? 'min-h-11 max-h-28 resize-none py-2 leading-5' : 'max-h-40 min-h-16 resize-y py-2.5 leading-6')}
        value={value} disabled={busy} placeholder={placeholder} onBlur={onBlur}
        onChange={(event) => updateValue(event.target.value, event.target.selectionStart, event.target.selectionEnd)}
        onSelect={(event) => { selection.current = { start: event.currentTarget.selectionStart, end: event.currentTarget.selectionEnd }; }}
        onClick={refreshQuery} onKeyDown={handleKeyDown}
        onKeyUp={(event) => { if (['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) refreshQuery(); }}
        onCompositionStart={() => { composing.current = true; setQuery(null); }}
        onCompositionEnd={() => { composing.current = false; refreshQuery(); }} />
      <div className={clsx('flex min-w-0 items-center gap-1.5 border-t border-line', compact ? 'h-10 px-1.5' : 'flex-wrap px-2 py-2')}>
        {editing || replying ? <span className={clsx('rounded-md px-2 py-1 text-xs font-semibold', editing ? 'bg-amber-50 text-amber-700' : 'bg-blue-50 text-blue-700')}>{editing ? 'Chỉnh sửa' : 'Phản hồi'}</span> : (
          <ActivityTypeSelect value={activityType} options={activityOptions} onChange={onActivityTypeChange} compact={compact} disabled={busy} onOpen={() => setQuery(null)} />
        )}
        {!editing ? <button type="button" disabled={busy} aria-label="Nhắc đến thành viên" aria-expanded={Boolean(query)} aria-controls={query ? `${id}-members` : undefined}
          className={clsx('inline-flex shrink-0 items-center gap-1 rounded-md font-semibold text-slate-600 hover:bg-slate-100 disabled:opacity-60', compact ? 'h-7 px-1.5 text-[11px]' : 'px-2 py-1.5 text-xs')}
          onClick={openMentions}><AtSign className="h-3.5 w-3.5" /><span>Nhắc đến</span></button> : null}
        <button type="submit" disabled={busy || editUnchanged} aria-label={editing ? 'Lưu chỉnh sửa' : 'Gửi trao đổi'} title={editing ? 'Enter để lưu chỉnh sửa' : 'Enter để gửi trao đổi'}
          className={clsx('ml-auto inline-flex shrink-0 items-center justify-center rounded-full bg-blue-600 text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60', editing ? 'h-8 gap-1.5 px-3 text-xs font-semibold' : compact ? 'h-8 w-8' : 'h-9 w-9')}>
          {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : editing ? 'Lưu' : <Send className="h-4 w-4" />}
        </button>
      </div>
      <p id={`${id}-keys`} className={compact ? 'sr-only' : 'px-3 pb-2 text-[11px] text-slate-400'}>
        Enter {editing ? 'lưu' : 'gửi'} · Ctrl/Shift+Enter xuống dòng{query ? ' · ↑↓ chọn thành viên, Enter chọn, Esc đóng' : ''}
      </p>
    </div>
  );
}

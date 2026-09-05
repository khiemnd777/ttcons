import type { ActivityMentionPayload } from '../../lib/types/domain/activityEvidence';

export type ActivityMentionOption = {
  type: 'staff' | 'role';
  id: string;
  label: string;
  description?: string;
};

export type ChatMentionOption = ActivityMentionOption | { type: 'all'; id: 'all'; label: 'all'; description: string };
export type InlineMention = { start: number; end: number; option: ChatMentionOption };
export type MentionQuery = { start: number; end: number; query: string };

const wordCharacter = /[\p{L}\p{N}_]/u;

/** A reserved chat token, not a person-name lookup; emails and longer identifiers never broadcast. */
export function hasAllMention(value: string) {
  return /(?:^|[\s([{])@all(?=$|[\s)\]},!?;:]|[.](?:$|\s))/iu.test(value);
}

export function mentionQueryAt(value: string, start: number, end = start, mentions: InlineMention[] = []): MentionQuery | null {
  if (start !== end) return null;
  const match = value.slice(0, start).match(/(?:^|[\s([{])@([^@\n\r]{0,60})$/u);
  if (!match) return null;
  const mentionStart = start - match[1].length - 1;
  if (match[1].length > 3 && hasAllMention(`@${match[1]}`)) return null;
  if (mentions.some((mention) => mention.start <= start && mention.end > mentionStart)) return null;
  const suffix = value.slice(start).match(/^[\p{L}\p{N}_.-]*/u)?.[0] ?? '';
  return { start: mentionStart, end: start + suffix.length, query: match[1] };
}

function searchText(value: string) {
  return value.normalize('NFD').replace(/\p{M}/gu, '').replace(/[đĐ]/g, 'd').toLocaleLowerCase('vi').trim();
}

export function filterMentionMembers(options: ActivityMentionOption[], query: string) {
  const words = searchText(query).split(/\s+/).filter(Boolean);
  const seen = new Set<string>();
  return options.filter((option) => {
    if (option.type !== 'staff' || seen.has(option.id)) return false;
    seen.add(option.id);
    const text = searchText(`${option.label} ${option.description ?? ''}`);
    return words.every((word) => text.includes(word));
  });
}

export function chatMentionSuggestions(options: ActivityMentionOption[], query: string): ChatMentionOption[] {
  const members = filterMentionMembers(options, query);
  const search = searchText(query);
  if (filterMentionMembers(options, '').length && ('all'.startsWith(search) || 'tat ca thanh vien du an'.includes(search))) {
    return [{ type: 'all', id: 'all', label: 'all', description: 'Tất cả thành viên dự án' }, ...members];
  }
  return members;
}

/** Only explicitly selected, untouched ranges retain recipient identity. Plain text never resolves names to IDs. */
export function reconcileInlineMentions(previous: string, next: string, mentions: InlineMention[], caretAfter?: number): InlineMention[] {
  if (previous === next) return mentions;
  let start = 0;
  let oldEnd = previous.length;
  let newEnd = next.length;
  // The input caret disambiguates deletion/replacement of equal text belonging to different people.
  const candidateOldEnd = caretAfter === undefined ? -1 : caretAfter + previous.length - next.length;
  if (caretAfter !== undefined && candidateOldEnd >= 0 && candidateOldEnd <= previous.length && previous.slice(candidateOldEnd) === next.slice(caretAfter)) {
    oldEnd = candidateOldEnd;
    newEnd = caretAfter;
    while (start < oldEnd && start < newEnd && previous[start] === next[start]) start += 1;
  } else {
    while (start < previous.length && start < next.length && previous[start] === next[start]) start += 1;
    while (oldEnd > start && newEnd > start && previous[oldEnd - 1] === next[newEnd - 1]) {
      oldEnd -= 1;
      newEnd -= 1;
    }
  }
  const shift = newEnd - oldEnd;
  return mentions.flatMap((mention) => {
    let updated = mention;
    if (oldEnd <= mention.start) updated = { ...mention, start: mention.start + shift, end: mention.end + shift };
    else if (start < mention.end) return [];
    const token = `@${mention.option.label}`;
    if (next.slice(updated.start, updated.end) !== token) return [];
    if (wordCharacter.test(next[updated.end] ?? '') || wordCharacter.test(next[updated.start - 1] ?? '')) return [];
    return [updated];
  });
}

export function insertInlineMention(value: string, query: MentionQuery, option: ChatMentionOption, mentions: InlineMention[]) {
  const token = `@${option.label}`;
  const replacement = value[query.end] === ' ' ? token : `${token} `;
  const next = value.slice(0, query.start) + replacement + value.slice(query.end);
  const retained = reconcileInlineMentions(value, next, mentions, query.start + replacement.length);
  return {
    value: next,
    caret: query.start + token.length + 1,
    mentions: [...retained, { start: query.start, end: query.start + token.length, option }].sort((a, b) => a.start - b.start),
  };
}

export function inlineMentionPayload(mentions: InlineMention[], value?: string) {
  const seen = new Set<string>();
  const payload = mentions.flatMap(({ option }) => {
    const key = `${option.type}:${option.id}`;
    if (seen.has(key)) return [];
    seen.add(key);
    return [{ type: option.type, id: option.id, label: option.label }];
  });
  return value === undefined ? payload : chatMentionPayload(value, payload);
}

export function chatMentionPayload(value: string, mentions: ActivityMentionPayload[]): ActivityMentionPayload[] {
  const payload = mentions.filter((mention) => mention.type !== 'all');
  if (hasAllMention(value)) payload.push({ type: 'all', id: 'all', label: 'all' });
  return payload;
}

export function chatKeyAction(event: {
  key: string;
  shiftKey?: boolean;
  ctrlKey?: boolean;
  altKey?: boolean;
  metaKey?: boolean;
  isComposing?: boolean;
  keyCode?: number;
}, popupOpen: boolean, busy: boolean): 'none' | 'dismiss' | 'previous' | 'next' | 'select' | 'newline' | 'submit' | 'block' {
  if (event.isComposing || event.keyCode === 229) return 'none';
  if (popupOpen && event.key === 'Escape') return 'dismiss';
  if (event.key === 'Enter') {
    if (busy) return 'block';
    if (event.shiftKey || event.ctrlKey) return 'newline';
    if (event.altKey || event.metaKey) return 'none';
    return popupOpen ? 'select' : 'submit';
  }
  if (popupOpen && event.key === 'ArrowUp') return 'previous';
  if (popupOpen && event.key === 'ArrowDown') return 'next';
  return 'none';
}

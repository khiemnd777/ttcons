import { journalEntryNoPresentation } from './journalEntryPresentation.ts';

export type JournalEntryClipboardWriter = (text: string) => Promise<void>;

export function isJournalEntryCopyKey(key: string) {
  return key === 'Enter' || key === ' ';
}

export async function copyJournalEntryNumber(value: unknown, writeText: JournalEntryClipboardWriter) {
  const { full } = journalEntryNoPresentation(value);
  if (!full) throw new Error('Chưa có số CTGS để sao chép.');
  await writeText(full);
  return full;
}

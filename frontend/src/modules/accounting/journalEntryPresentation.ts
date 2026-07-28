export type JournalEntryNoPresentation = {
  full: string;
  display: string;
  compacted: boolean;
};

const maxReadableLength = 24;
const backfillEntryPattern = /^(BF-(?:IN|OUT|PAY)-)([a-f0-9]{32})$/i;

export function journalEntryNoPresentation(value: unknown): JournalEntryNoPresentation {
  const full = String(value ?? '').trim();
  if (!full) return { full: '', display: '—', compacted: false };
  if (full.length <= maxReadableLength) return { full, display: full, compacted: false };

  const backfillMatch = full.match(backfillEntryPattern);
  if (backfillMatch) {
    const [, prefix, identifier] = backfillMatch;
    return {
      full,
      display: `${prefix}${identifier.slice(0, 6)}…${identifier.slice(-6)}`,
      compacted: true,
    };
  }

  return {
    full,
    display: `${full.slice(0, 16)}…${full.slice(-6)}`,
    compacted: true,
  };
}

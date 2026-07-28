import assert from 'node:assert/strict';
import test from 'node:test';
import { copyJournalEntryNumber, isJournalEntryCopyKey } from '../src/modules/accounting/journalEntryClipboard.ts';
import { journalEntryNoPresentation } from '../src/modules/accounting/journalEntryPresentation.ts';

test('keeps a normal sequential CTGS number fully visible', () => {
  assert.deepEqual(journalEntryNoPresentation('CTGS-202607-000001'), {
    full: 'CTGS-202607-000001',
    display: 'CTGS-202607-000001',
    compacted: false,
  });
});

test('compacts a converted payment CTGS while preserving its immutable full number', () => {
  const full = 'BF-PAY-ffd22b0318964e3eb0822bb0c658faeb';
  assert.deepEqual(journalEntryNoPresentation(full), {
    full,
    display: 'BF-PAY-ffd22b…58faeb',
    compacted: true,
  });
});

test('compacts other long CTGS formats without discarding the full search value', () => {
  const full = 'IMPORT-2026-ACCOUNTING-JOURNAL-000000123456';
  const presentation = journalEntryNoPresentation(full);
  assert.equal(presentation.full, full);
  assert.equal(presentation.display, 'IMPORT-2026-ACCO…123456');
  assert.equal(presentation.compacted, true);
});

test('renders a neutral placeholder for a missing CTGS number', () => {
  assert.deepEqual(journalEntryNoPresentation(undefined), {
    full: '',
    display: '—',
    compacted: false,
  });
});

test('copies the immutable full CTGS number instead of its compact presentation', async () => {
  const full = 'BF-PAY-ffd22b0318964e3eb0822bb0c658faeb';
  let copied = '';
  const result = await copyJournalEntryNumber(full, async (text) => {
    copied = text;
  });

  assert.equal(result, full);
  assert.equal(copied, full);
  assert.notEqual(copied, journalEntryNoPresentation(full).display);
});

test('does not call the clipboard writer when the CTGS number is missing', async () => {
  let writeCount = 0;
  await assert.rejects(
    () => copyJournalEntryNumber(undefined, async () => {
      writeCount += 1;
    }),
    /Chưa có số CTGS/,
  );
  assert.equal(writeCount, 0);
});

test('propagates a denied clipboard write so the UI can report an error', async () => {
  await assert.rejects(
    () => copyJournalEntryNumber('CTGS-202607-000001', async () => {
      throw new Error('clipboard denied');
    }),
    /clipboard denied/,
  );
});

test('recognizes the accessible keyboard activation keys for copying a CTGS number', () => {
  assert.equal(isJournalEntryCopyKey('Enter'), true);
  assert.equal(isJournalEntryCopyKey(' '), true);
  assert.equal(isJournalEntryCopyKey('Escape'), false);
});

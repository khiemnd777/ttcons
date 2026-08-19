import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import type { EvidenceRecord } from '../src/lib/types/domain.ts';
import {
  evidenceMediaType,
  isImageEvidence,
  isInlineSpreadsheetEvidence,
  isInlineWordEvidence,
  isLegacySpreadsheetEvidence,
  isLegacyWordEvidence,
} from '../src/modules/_shared/evidenceMedia.ts';
import { spreadsheetColumnLabel } from '../src/modules/_shared/evidenceSpreadsheetPreview.ts';
import {
  inspectOfficePreviewArchive,
  MAX_OFFICE_PREVIEW_ENTRY_BYTES,
} from '../src/modules/_shared/officePreviewArchive.ts';

function evidence(overrides: Partial<EvidenceRecord>): EvidenceRecord {
  return {
    id: 'evidence-office-1',
    evidence_type: 'document',
    name: 'Tệp văn phòng',
    status: 'submitted',
    work_item_id: 'work-item-1',
    ...overrides,
  } as EvidenceRecord;
}

test('protected DOCX and XLSX records are classified for inline readers', () => {
  const word = evidence({
    mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    storage_provider: 'local',
    storage_key: 'evidence/brief.docx',
  });
  const spreadsheet = evidence({
    document_name: 'Bang khoi luong.XLSX',
    storage_provider: 's3',
    storage_key: 'evidence/boq-object',
  });

  assert.equal(evidenceMediaType(word), 'word');
  assert.equal(isInlineWordEvidence(word), true);
  assert.equal(evidenceMediaType(spreadsheet), 'spreadsheet');
  assert.equal(isInlineSpreadsheetEvidence(spreadsheet), true);
});

test('Office MIME takes precedence over a stale photo evidence type', () => {
  const word = evidence({
    evidence_type: 'photo',
    mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    storage_provider: 'local',
    storage_key: 'evidence/word-object',
  });

  assert.equal(isImageEvidence(word), false);
  assert.equal(evidenceMediaType(word), 'word');
});

test('legacy and external Office files remain file-first', () => {
  const legacyWord = evidence({ document_name: 'bien-ban.doc', mime_type: 'application/msword', storage_provider: 'local', storage_key: 'legacy/doc' });
  const legacySpreadsheet = evidence({ document_name: 'du-toan.xls', mime_type: 'application/vnd.ms-excel', storage_provider: 'local', storage_key: 'legacy/xls' });
  const externalWord = evidence({ document_name: 'brief.docx', storage_provider: 'external', file_url: 'https://example.com/brief.docx' });

  assert.equal(isLegacyWordEvidence(legacyWord), true);
  assert.equal(isInlineWordEvidence(legacyWord), false);
  assert.equal(evidenceMediaType(legacyWord), 'document');
  assert.equal(isLegacySpreadsheetEvidence(legacySpreadsheet), true);
  assert.equal(isInlineSpreadsheetEvidence(legacySpreadsheet), false);
  assert.equal(evidenceMediaType(legacySpreadsheet), 'document');
  assert.equal(evidenceMediaType(externalWord), 'word');
  assert.equal(isInlineWordEvidence(externalWord), false);
});

test('spreadsheet column labels stay deterministic beyond Z', () => {
  assert.equal(spreadsheetColumnLabel(0), 'A');
  assert.equal(spreadsheetColumnLabel(25), 'Z');
  assert.equal(spreadsheetColumnLabel(26), 'AA');
  assert.equal(spreadsheetColumnLabel(701), 'ZZ');
});

test('Office preview archive guard bounds expanded ZIP content', () => {
  const safeArchive = previewArchive(MAX_OFFICE_PREVIEW_ENTRY_BYTES);
  assert.deepEqual(inspectOfficePreviewArchive(safeArchive), {
    entryCount: 1,
    uncompressedBytes: MAX_OFFICE_PREVIEW_ENTRY_BYTES,
  });
  assert.throws(
    () => inspectOfficePreviewArchive(previewArchive(MAX_OFFICE_PREVIEW_ENTRY_BYTES + 1)),
    /quá lớn để xem trước an toàn/,
  );
  assert.throws(() => inspectOfficePreviewArchive(new ArrayBuffer(64)), /không có cấu trúc ZIP hợp lệ/);
});

test('Word and Excel readers parse lazily with defensive rendering contracts', async () => {
  const wordViewer = await readFile(new URL('../src/modules/_shared/EvidenceWordViewer.tsx', import.meta.url), 'utf8');
  const spreadsheetViewer = await readFile(new URL('../src/modules/_shared/EvidenceSpreadsheetViewer.tsx', import.meta.url), 'utf8');
  const spreadsheetWorker = await readFile(new URL('../src/modules/_shared/evidenceSpreadsheet.worker.ts', import.meta.url), 'utf8');
  const recordViewer = await readFile(new URL('../src/modules/_shared/EvidenceRecordViewerDialog.tsx', import.meta.url), 'utf8');

  assert.match(wordViewer, /import\('docx-preview'\)/);
  assert.match(wordViewer, /renderAltChunks: false/);
  assert.match(wordViewer, /inspectOfficePreviewArchive\(buffer\)/);
  assert.match(wordViewer, /querySelectorAll\('script, iframe, object, embed, form'\)/);
  assert.match(wordViewer, /noopener noreferrer/);
  assert.match(spreadsheetViewer, /new Worker\(new URL\('\.\/evidenceSpreadsheet\.worker\.ts'/);
  assert.match(spreadsheetViewer, /useVirtualizer/);
  assert.match(spreadsheetViewer, /inspectOfficePreviewArchive\(buffer\)/);
  assert.match(spreadsheetWorker, /import xlsxMini from 'xlsx\/dist\/xlsx\.mini\.min\.js'/);
  assert.match(spreadsheetWorker, /cellFormula: false/);
  assert.doesNotMatch(spreadsheetWorker, /sheet_to_html/);
  assert.match(recordViewer, /Mở tệp gốc/);
  assert.match(recordViewer, /Định dạng Word đời cũ/);
  assert.match(recordViewer, /Định dạng Excel đời cũ/);
  assert.match(recordViewer, /blob: normalizedBlob/);
});

function previewArchive(uncompressedBytes: number) {
  const centralDirectoryBytes = 46;
  const endRecordBytes = 22;
  const buffer = new ArrayBuffer(centralDirectoryBytes + endRecordBytes);
  const view = new DataView(buffer);
  view.setUint32(0, 0x02014b50, true);
  view.setUint32(24, uncompressedBytes, true);
  const endOffset = centralDirectoryBytes;
  view.setUint32(endOffset, 0x06054b50, true);
  view.setUint16(endOffset + 8, 1, true);
  view.setUint16(endOffset + 10, 1, true);
  view.setUint32(endOffset + 12, centralDirectoryBytes, true);
  view.setUint32(endOffset + 16, 0, true);
  return buffer;
}

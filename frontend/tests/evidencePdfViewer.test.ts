import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import type { EvidenceRecord } from '../src/lib/types/domain.ts';
import {
  evidenceMediaType,
  isImageEvidence,
  isInlinePdfEvidence,
  isPdfEvidence,
} from '../src/modules/_shared/evidenceMedia.ts';

function evidence(overrides: Partial<EvidenceRecord>): EvidenceRecord {
  return {
    id: 'evidence-1',
    evidence_type: 'document',
    status: 'submitted',
    ...overrides,
  } as EvidenceRecord;
}

test('PDF MIME takes precedence over a legacy photo evidence type', () => {
  const record = evidence({
    evidence_type: 'photo',
    mime_type: 'application/pdf',
    storage_provider: 'local',
    storage_key: 'evidence/legacy-photo.pdf',
  });

  assert.equal(isPdfEvidence(record), true);
  assert.equal(isImageEvidence(record), false);
  assert.equal(isInlinePdfEvidence(record), true);
  assert.equal(evidenceMediaType(record), 'pdf');
});

test('protected PDF filename fallback is previewable while external PDF stays file-first', () => {
  const protectedRecord = evidence({
    mime_type: '',
    document_name: 'Hồ sơ nghiệm thu.PDF',
    storage_provider: 's3',
    storage_key: 'project/evidence-1',
  });
  const externalRecord = evidence({
    mime_type: 'application/pdf; charset=binary',
    storage_provider: 'external',
    file_url: 'https://example.com/evidence.pdf',
  });

  assert.equal(isPdfEvidence(protectedRecord), true);
  assert.equal(isInlinePdfEvidence(protectedRecord), true);
  assert.equal(isPdfEvidence(externalRecord), true);
  assert.equal(isInlinePdfEvidence(externalRecord), false);
});

test('ordinary image and non-PDF document classification remains unchanged', () => {
  assert.equal(evidenceMediaType(evidence({ evidence_type: 'photo', mime_type: 'image/jpeg' })), 'photo');
  assert.equal(evidenceMediaType(evidence({ evidence_type: 'document', mime_type: 'text/plain' })), 'document');
});

test('shared evidence viewer renders protected PDFs in-app with lifecycle-safe blob URLs', async () => {
  const pdfViewer = await readFile(new URL('../src/modules/_shared/EvidencePdfViewer.tsx', import.meta.url), 'utf8');
  const recordViewer = await readFile(new URL('../src/modules/_shared/EvidenceRecordViewerDialog.tsx', import.meta.url), 'utf8');
  const gallery = await readFile(new URL('../src/modules/work-items/EvidenceGallery.tsx', import.meta.url), 'utf8');

  assert.match(pdfViewer, /<iframe/);
  assert.match(pdfViewer, /Đang tải PDF/);
  assert.match(pdfViewer, /Trình duyệt không thể hiển thị tệp PDF này/);
  assert.match(pdfViewer, /Hồ sơ chưa có tệp PDF để đọc/);
  assert.match(recordViewer, /downloadBlob\(protectedFilePath \|\| `\/evidence\/\$\{recordId\}\/file`\)/);
  assert.match(recordViewer, /URL\.createObjectURL\(normalizedBlob\)/);
  assert.match(recordViewer, /URL\.revokeObjectURL\(objectUrl\)/);
  assert.match(recordViewer, /mediaType=\{protectedMediaType \?\?/);
  assert.match(gallery, /canPreviewEvidenceInApp\(record\) \? setSelected\(record\)/);
  assert.match(gallery, /return 'Xem PDF'/);
});

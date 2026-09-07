import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  classifyEvidenceFile,
  evidenceFileAccept,
  evidenceFileKindLabel,
} from '../src/modules/_shared/evidenceFileTypes.ts';

test('evidence files are classified into Image, PDF, Word and Excel business types', () => {
  const cases = [
    [{ name: 'site.jpeg', type: 'image/jpeg' }, 'photo', 'Ảnh', 'photo'],
    [{ name: 'site.gif', type: 'image/gif' }, 'photo', 'Ảnh', 'photo'],
    [{ name: 'survey.pdf', type: 'application/pdf' }, 'pdf', 'PDF', 'document'],
    [{ name: 'survey.doc', type: 'application/msword' }, 'word', 'Word', 'document'],
    [{ name: 'survey.docx', type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' }, 'word', 'Word', 'document'],
    [{ name: 'survey.xls', type: 'application/vnd.ms-excel' }, 'excel', 'Excel', 'document'],
    [{ name: 'survey.xlsx', type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }, 'excel', 'Excel', 'document'],
  ] as const;

  cases.forEach(([file, kind, label, evidenceType]) => {
    assert.deepEqual(classifyEvidenceFile(file), { kind, label, evidenceType });
    assert.equal(evidenceFileKindLabel(kind), label);
  });
});

test('filename fallback is limited to missing or generic browser MIME values', () => {
  assert.equal(classifyEvidenceFile({ name: 'survey.DOCX', type: '' })?.kind, 'word');
  assert.equal(classifyEvidenceFile({ name: 'survey.xlsx', type: 'application/zip' })?.kind, 'excel');
  assert.equal(classifyEvidenceFile({ name: 'survey.doc', type: 'application/CDFV2' })?.kind, 'word');
  assert.equal(classifyEvidenceFile({ name: 'survey.pdf', type: 'application/octet-stream' })?.kind, 'pdf');
  assert.equal(classifyEvidenceFile({ name: 'survey.docx', type: 'text/plain' }), null);
  assert.equal(classifyEvidenceFile({ name: 'site.svg', type: 'image/svg+xml' }), null);
  assert.equal(classifyEvidenceFile({ name: 'site.heic', type: '' }), null);
  assert.equal(classifyEvidenceFile({ name: 'survey.docm', type: '' }), null);
  assert.equal(classifyEvidenceFile({ name: 'survey.xlsm', type: '' }), null);
});

test('file chooser advertises every supported non-macro format', () => {
  for (const extension of ['.png', '.jpg', '.jpeg', '.webp', '.gif', '.pdf', '.doc', '.docx', '.xls', '.xlsx']) {
    assert.match(evidenceFileAccept, new RegExp(`(?:^|,)\\${extension}(?:,|$)`));
  }
  assert.doesNotMatch(evidenceFileAccept, /\.docm|\.xlsm|svg|heic|tiff/);
});

test('all work-item evidence upload surfaces derive the immutable business type from the selected file', async () => {
  const workItemPanel = await readFile(new URL('../src/modules/work-items/EvidencePanel.tsx', import.meta.url), 'utf8');
  const sharedCapture = await readFile(new URL('../src/modules/_shared/EvidenceCapture.tsx', import.meta.url), 'utf8');
  const constructionReport = await readFile(new URL('../src/modules/operations/ConstructionReportFormDialog.tsx', import.meta.url), 'utf8');

  assert.match(workItemPanel, /const classification = classifyEvidenceFile\(file\)/);
  assert.match(workItemPanel, /formData\.append\('evidence_type', row\.classification\.evidenceType\)/);
  const uploader = await readFile(new URL('../src/modules/_shared/EvidenceUploader.tsx', import.meta.url), 'utf8');
  assert.match(uploader, /accept=\{evidenceFileAccept\}/);
  assert.doesNotMatch(workItemPanel, /<option value="photo">/);
  assert.match(sharedCapture, /fileKind: classification\.kind/);
  assert.match(sharedCapture, /accept=\{evidenceFileAccept\}/);
  assert.match(sharedCapture, /classification\?\.evidenceType \?\? draft\.evidenceType/);
  assert.match(constructionReport, /uploads\.some\(\(item\) => item\.evidenceType === 'photo'\)/);
});

test('all evidence upload surfaces fix new records to internal visibility without a chooser', async () => {
  const workItemPanel = await readFile(new URL('../src/modules/work-items/EvidencePanel.tsx', import.meta.url), 'utf8');
  const sharedCapture = await readFile(new URL('../src/modules/_shared/EvidenceCapture.tsx', import.meta.url), 'utf8');

  assert.match(workItemPanel, /formData\.append\('visibility', 'internal'\)/);
  assert.doesNotMatch(workItemPanel, /Phạm vi hiển thị|setVisibility|EvidenceVisibility/);
  assert.match(sharedCapture, /formData\.append\('visibility', 'internal'\)/);
  assert.doesNotMatch(sharedCapture, /formData\.append\('visibility', 'project'\)/);
});

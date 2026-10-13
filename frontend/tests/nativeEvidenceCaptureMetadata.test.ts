import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { renderToStaticMarkup } from 'react-dom/server';
import { captureDateTimeInput, captureDateTimeValue, appendEvidenceCaptureMetadata } from '../src/modules/_shared/evidenceCaptureMetadata.ts';

const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export { EvidenceUploader } from './modules/_shared/EvidenceUploader'; export { ConstructionEvidenceCapture, constructionEvidenceFormData } from './modules/operations/ConstructionEvidenceCapture'; export { uploadEvidenceDraftBatch } from './modules/_shared/evidenceUploadSession';` }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' }, plugins: [{ name: 'toast-boundary', setup(builder) { builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast() { return { warn() {}, error() {} }; }', loader: 'js' })); } }] });

// Actual component callbacks and receipt store; controlled hooks avoid a browser.
function fixture() {
  const state: any[] = []; let cursor = 0;
  function useState(initial: any) { const index = cursor++; if (!(index in state)) state[index] = typeof initial === 'function' ? initial() : initial; return [state[index], (next: any) => { state[index] = typeof next === 'function' ? next(state[index]) : next; }]; }
  const hooks = { ...react, useState, useRef: (value: any) => useState(() => ({ current: value }))[0], useEffect() {}, useCallback: (fn: any) => fn, useSyncExternalStore: (_subscribe: any, snapshot: any) => snapshot() };
  const module = { exports: {} as Record<string, any> };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === 'react' ? hooks : require(name), module, module.exports);
  return { api: module.exports, render(name: string, props: any) { cursor = 0; return module.exports[name](props); } };
}
function nodes(tree: any, matches: any[] = []): any[] { if (!tree || typeof tree !== 'object') return matches; matches.push(tree); for (const child of [tree.props?.children].flat(Infinity)) nodes(child, matches); return matches; }
function field(tree: any, label: string) { const input = nodes(tree).find(node => node.props?.['aria-label'] === label); assert.ok(input, label); return input; }
const file = () => new File(['photo-original'], 'site.jpg', { type: 'image/jpeg', lastModified: Date.parse('1999-01-01T00:00:00Z') });
function draft(metadata: any = {}) { return { id: 'draft', file: file(), evidenceType: 'photo', fileKind: 'photo', name: 'Site', caption: 'Caption', captureMetadata: metadata }; }
function row(metadata: any = {}) { return { ...draft(metadata), classification: { kind: 'photo', label: 'Hình ảnh', evidenceType: 'photo' }, fileIssue: '', status: 'queued', progress: 0, error: '' }; }
function uploaderProps(uploadRows: any[], updateUploadRow: any = () => {}) { return { uploadRows, batchUploading: false, updateUploadRow, onFileChange() {}, addUploadRow() {}, removeUploadRow() {} }; }

test('photo source with empty metadata renders optional controls; document and design-only rows do not', () => {
  const hooks = fixture();
  const photo = hooks.render('EvidenceUploader', uploaderProps([row()]));
  const html = renderToStaticMarkup(photo);
  assert.match(html, /Thông tin ảnh hiện trường/);
  for (const label of ['Thời điểm chụp ảnh 1', 'Vị trí ảnh 1', 'Giai đoạn ảnh 1', 'Ghi chú bàn giao ảnh 1']) { const input = field(photo, label); assert.equal(input.props.value, ''); assert.ok(!input.props.required); }
  for (const other of [{ ...row(), classification: { kind: 'pdf', label: 'PDF', evidenceType: 'document' } }, { ...row(), captureMetadata: undefined }]) assert.doesNotMatch(renderToStaticMarkup(hooks.render('EvidenceUploader', uploaderProps([other]))), /Thông tin ảnh hiện trường|datetime-local/);
});

test('actual uploader callbacks retain other photo metadata and row identity', () => {
  const hooks = fixture(); let current = row({ captured_at: '2026-10-08T01:30:00Z', location: 'Zone A', photo_phase: 'before', handover_note: 'Original note' });
  const updates = [['Vị trí ảnh 1', 'Zone B', 'location'], ['Giai đoạn ảnh 1', 'after', 'photo_phase'], ['Ghi chú bàn giao ảnh 1', 'New note', 'handover_note'], ['Thời điểm chụp ảnh 1', '', 'captured_at']];
  for (const [label, value, key] of updates) {
    const previous = current; const tree = hooks.render('EvidenceUploader', uploaderProps([current], (id: string, update: any) => { assert.equal(id, previous.id); current = update(current); }));
    field(tree, label).props.onChange({ target: { value } });
    assert.deepEqual(current.captureMetadata, { ...previous.captureMetadata, [key]: value });
    assert.equal(current.file, previous.file); assert.equal(current.caption, previous.caption);
  }
});

test('immutable and in-flight rows disable every metadata field', () => {
  for (const status of ['success', 'uploading', 'processing']) {
    const tree = fixture().render('EvidenceUploader', uploaderProps([{ ...row(), status }]));
    for (const label of ['Thời điểm chụp ảnh 1', 'Vị trí ảnh 1', 'Giai đoạn ảnh 1', 'Ghi chú bàn giao ảnh 1']) assert.equal(field(tree, label).props.disabled, true);
  }
  const tree = fixture().render('EvidenceUploader', { ...uploaderProps([row()]), batchUploading: true });
  assert.equal(field(tree, 'Vị trí ảnh 1').props.disabled, true);
});

test('date helper round trips local wall time and RFC instant across time zones; blank never substitutes a time', () => {
  const previous = process.env.TZ;
  try {
    for (const zone of ['Asia/Ho_Chi_Minh', 'America/New_York', 'UTC']) {
      process.env.TZ = zone;
      const wall = '2026-10-08T08:45'; const instant = captureDateTimeValue(wall);
      assert.equal(captureDateTimeInput(instant), wall);
      assert.equal(new Date(instant).getTime(), new Date(wall).getTime());
      const rfc = '2026-10-08T08:45:00+07:00';
      assert.equal(new Date(captureDateTimeValue(captureDateTimeInput(rfc))).getTime(), new Date(rfc).getTime());
      assert.equal(captureDateTimeInput(''), ''); assert.equal(captureDateTimeValue(''), '');
      assert.equal(captureDateTimeInput('invalid'), ''); assert.equal(captureDateTimeValue('invalid'), '');
    }
  } finally { if (previous === undefined) delete process.env.TZ; else process.env.TZ = previous; }
});

test('append metadata keeps original payload fields and one payload; blank metadata leaves payload unchanged', () => {
  const data = new FormData(); data.append('payload', JSON.stringify({ source_id: 'source', approvals: ['actor'], nested: { value: 2 } }));
  appendEvidenceCaptureMetadata(data, { location: ' Zone A ', photo_phase: 'during' });
  assert.deepEqual(JSON.parse(String(data.get('payload'))), { source_id: 'source', approvals: ['actor'], nested: { value: 2 }, capture_metadata: { location: 'Zone A', photo_phase: 'during' } });
  assert.equal(data.getAll('payload').length, 1);
  const original = data.get('payload'); appendEvidenceCaptureMetadata(data, { captured_at: '', location: '  ' }); assert.equal(data.get('payload'), original);
});

test('actual construction multipart sends explicit photo metadata and caption; never derives capture time from file mtime', () => {
  const { api } = fixture();
  const photo = draft({ captured_at: '2026-10-08T01:45:00Z', location: 'Area', photo_phase: 'defect', handover_note: 'Note' });
  const data = api.constructionEvidenceFormData(photo, 'work-item');
  assert.equal(data.get('work_item_id'), 'work-item'); assert.equal(data.get('caption'), 'Caption'); assert.equal(data.get('visibility'), 'internal');
  assert.deepEqual(JSON.parse(String(data.get('payload'))).capture_metadata, photo.captureMetadata);
  assert.equal(data.getAll('file').length, 1); assert.equal(data.getAll('payload').length, 1);
  assert.equal(api.constructionEvidenceFormData(draft(), 'work-item').has('payload'), false);
  const document = { ...photo, file: new File(['pdf'], 'doc.pdf', { type: 'application/pdf' }) };
  assert.equal(api.constructionEvidenceFormData(document, 'work-item').has('payload'), false);
});

test('replacing a selected file clears capture metadata and renews receipt identity while preserving caption', () => {
  const hooks = fixture(); let drafts = [draft({ location: 'Old place', captured_at: '2026-10-08T01:45:00Z', photo_phase: 'before' })];
  const original = drafts[0];
  const tree = hooks.render('ConstructionEvidenceCapture', { drafts, onChange(next: any) { drafts = next; }, hasSource: true, uploadScope: 'replace' });
  const uploader = nodes(tree).find(node => node.type?.name === 'EvidenceUploader'); assert.ok(uploader);
  const replacement = file(); uploader.props.onFileChange(original.id, replacement, { value: 'chosen' });
  assert.equal(drafts[0].file, replacement); assert.notEqual(drafts[0].id, original.id); assert.deepEqual(drafts[0].captureMetadata, {}); assert.equal(drafts[0].caption, original.caption);
});

test('parent-save retry reuses successful receipt, freezes metadata callbacks and never uploads originals twice', async () => {
  const hooks = fixture(); const savedDraft = draft({ location: 'Original place', captured_at: '2026-10-08T01:45:00Z' }); let uploaded = 0; let changed = 0;
  const worker = async (current: any) => { uploaded++; const data = hooks.api.constructionEvidenceFormData(current, 'work-item'); assert.equal(JSON.parse(String(data.get('payload'))).capture_metadata.location, 'Original place'); return 'saved-evidence'; };
  const upload = () => hooks.api.uploadEvidenceDraftBatch([savedDraft], 'parent-save-retry', worker);
  assert.deepEqual(await upload(), ['saved-evidence']);
  await assert.rejects(async () => { await upload(); throw new Error('parent save failed'); }, /parent save failed/);
  const tree = hooks.render('ConstructionEvidenceCapture', { drafts: [savedDraft], onChange() { changed++; }, hasSource: true, uploadScope: 'parent-save-retry' });
  const uploader = nodes(tree).find(node => node.type?.name === 'EvidenceUploader'); assert.ok(uploader);
  assert.equal(uploader.props.uploadRows[0].status, 'success');
  uploader.props.updateUploadRow(savedDraft.id, (current: any) => ({ ...current, captureMetadata: { location: 'Illegal change' } }));
  assert.equal(changed, 0); assert.deepEqual(await upload(), ['saved-evidence']); assert.equal(uploaded, 1);
});

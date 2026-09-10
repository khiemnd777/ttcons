import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile, readdir } from 'node:fs/promises';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const require = createRequire(import.meta.url);
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
const src = fileURLToPath(new URL('../src/', import.meta.url));
const bundle = await build({
  stdin: { resolveDir: src, contents: `
    export { EvidenceUploader } from './modules/_shared/EvidenceUploader';
    export { EvidenceRecordViewerDialog, EvidenceDraftViewerDialog } from './modules/_shared/EvidenceRecordViewerDialog';
    export { EvidenceReferenceViewer } from './modules/_shared/EvidenceReferenceViewer';
    export { CheckFormDialog, CompleteCheckDialog, ExistingContextEvidence } from './modules/operations/ConstructionCheckDialogs';
    export { ConstructionEvidenceButton } from './modules/operations/ConstructionEvidenceButton';
    export { EvidenceCard } from './modules/work-items/EvidenceGallery';
    export { ActionFormDialog, ResolutionDialog } from './modules/operations/ConstructionActionDialogs';
    export { ConstructionReportFormDialog } from './modules/operations/ConstructionReportFormDialog';
    export { ConstructionReportEvidenceDialog } from './modules/operations/ConstructionReportEvidenceDialog';
    export { ConstructionReportLinkedComposer, createReportLinkedDraft } from './modules/operations/ConstructionReportLinkedComposer';
    export { AcceptanceEditor, createAcceptanceDraft } from './modules/acceptance/AcceptanceEditor';
    export { ToastProvider } from './components/ToastProvider';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'private-dialog-exports', setup(builder) {
    builder.onLoad({ filter: /ConstructionCheckDialogs\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { CheckFormDialog, CompleteCheckDialog, ExistingContextEvidence };', loader: 'tsx' }));
    builder.onLoad({ filter: /ConstructionActionDialogs\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { ActionFormDialog, ResolutionDialog };', loader: 'tsx' }));
  } }],
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
const components = module.exports;
function render(name: string, props: object, seed?: (client: any) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  seed?.(client);
  try { return renderToStaticMarkup(createElement(QueryClientProvider, { client }, createElement(components.ToastProvider, null, createElement(components[name], props)))); }
  finally { client.clear(); }
}

test('all eight business entrypoints render the same upload table without nested upload forms', () => {
  const workspace = { work_items: [], participants: [], check_templates: [], templates: [], checks: [], action_items: [] };
  const common = { projectId: 'project', workspace, onClose() {}, onSaved() {}, onSubmit: async () => {} };
  const check = { id: 'check', title: 'Check', work_item_id: 'item', criteria: [] };
  const action = { id: 'action', title: 'Action', work_item_id: 'item' };
  const draft = components.createAcceptanceDraft({ workItemId: 'item' });
  const cases: [string, object][] = [
    ['CheckFormDialog', { ...common, state: { type: 'create' } }],
    ['CompleteCheckDialog', { ...common, check }],
    ['ActionFormDialog', { ...common, state: { type: 'create' } }],
    ['ResolutionDialog', { ...common, action }],
    ['ConstructionReportEvidenceDialog', { ...common, open: true, record: { id: 'report', work_item_id: 'item', photos: [] } }],
    ['ConstructionReportLinkedComposer', { ...common, mode: 'immediate', drafts: [components.createReportLinkedDraft('check')], onChange() {}, canCreateCheck: true, canCreateAction: true }],
    ['AcceptanceEditor', { draft, evidence: [], workItems: [], onChange() {} }],
    ['ConstructionReportFormDialog', { ...common, open: true }],
  ];
  for (const [name, props] of cases) {
    const html = render(name, props);
    assert.equal((html.match(/data-component="evidence-uploader"/g) ?? []).length, 1, name);
    assert.match(html, /Tên minh chứng/);
    assert.match(html, /Tiến độ|tiến độ/);
    assert.doesNotMatch(html, /Tệp và ảnh hiện trường|report-photo-grid/, name);
    assert.ok((html.match(/<form\b/g) ?? []).length <= 1, name + ': nested form');
  }
});

const record = { id: 'ev', work_item_id: 'item', project_id: 'project', name: 'Site evidence', evidence_type: 'photo', status: 'submitted', created_at: '2026-09-07T10:00:00Z', submitted_by_name: 'Người gửi thật', reviewed_by_name: 'Người rà soát thật', work_item_title: 'Đầu việc thật', run_node_name: 'Thi công', storage_provider: 'local', storage_key: 'evidence/file', caption: 'Ghi chú thật' };

test('saved context evidence renders the existing gallery card before opening a viewer', () => {
  const html = render('ExistingContextEvidence', { evidence: [record], onRemove() {} });
  assert.match(html, /data-component="evidence-card"/);
  assert.match(html, /aspect-\[4\/3\]/, 'the shared thumbnail region must be visible before clicking');
  assert.match(html, />Xem ảnh<|>\s*Xem ảnh\s*</);
  assert.match(html, /Ghi chú thật/);
  assert.match(html, /aria-label="Gỡ Site evidence"/);
  assert.doesNotMatch(html, /lucide-file-image/);
});

test('construction reference cards use typed actions and resolve ID-only records without invented metadata', () => {
  for (const [name, mime, action] of [
    ['site.png', 'image/png', 'Xem ảnh'], ['site.pdf', 'application/pdf', 'Xem PDF'],
    ['site.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'Xem Word'],
    ['site.xlsx', 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 'Xem Excel'],
  ]) {
    const item = { ...record, name, mime_type: mime, evidence_type: mime === 'image/png' ? 'photo' : 'document' };
    const html = render('ConstructionEvidenceButton', { evidenceId: item.id, name, label: 'Kết quả khắc phục' }, (client) => {
      // A settled transport fixture: SSR does not run the query's HTTP effects.
      client.setQueryDefaults(['evidence-detail', item.id], { refetchOnMount: false });
      client.setQueryData(['evidence-detail', item.id], item);
    });
    assert.match(html, /data-component="evidence-card"/);
    assert.ok(html.includes(action), name);
    assert.ok(html.includes('Ghi chú thật'), name);
  }
  const pending = render('ConstructionEvidenceButton', { evidenceId: 'unknown', name: 'Unknown', label: 'Minh chứng sự kiện' });
  assert.match(pending, /Đang tải thông tin minh chứng/);
  assert.doesNotMatch(pending, /data-component="evidence-card"|Ghi chú thật/);
  const empty = render('ConstructionEvidenceButton', { label: 'Kết quả khắc phục' });
  assert.match(empty, /Chưa có kết quả khắc phục/);
  assert.doesNotMatch(empty, /data-component="evidence-card"/);
});

test('shared stored card renders photo content, loading and retry; legacy Office retains original-file access', () => {
  const props = { record, showWorkItem: false, onSelect() {}, onOpen() {}, onRetry() {} };
  const photo = render('EvidenceCard', { ...props, previewUrl: 'blob:authorized-photo' });
  assert.match(photo, /<img[^>]*src="blob:authorized-photo"[^>]*alt="Site evidence"/);
  assert.match(photo, /aria-label="Xem chi tiết hồ sơ Site evidence"/);
  const loading = render('EvidenceCard', { ...props, previewLoading: true });
  assert.match(loading, /Đang tải ảnh xem trước/);
  const failed = render('EvidenceCard', { ...props, previewError: '403', previewUrl: 'blob:stale' });
  assert.match(failed, /Không tải được ảnh xem trước/);
  assert.match(failed, /Thử lại ảnh/);
  assert.doesNotMatch(failed, /<img/);
  for (const [name, mime] of [['legacy.doc', 'application/msword'], ['legacy.xls', 'application/vnd.ms-excel']]) {
    const html = render('EvidenceReferenceViewer', { reference: { id: record.id, record: { ...record, name, mime_type: mime, evidence_type: 'document' } }, onClose() {} });
    assert.match(html, />Mở tệp</);
  }
});

test('stored context removal stays a draft unlink and metadata denial never renders a preview', () => {
  const removed: string[] = [];
  const evidence = [record];
  const tree = components.ExistingContextEvidence({ evidence, onRemove: (id: string) => removed.push(id) });
  const removeButton = tree.props.children[0].props.children[1];
  assert.equal(removeButton.props['aria-label'], 'Gỡ Site evidence');
  removeButton.props.onClick();
  assert.deepEqual(removed, [record.id]);
  assert.deepEqual(evidence, [record], 'the card must not mutate/delete the record');
  const denied = render('ConstructionEvidenceButton', { evidenceId: 'denied', label: 'Minh chứng sự kiện' }, (client) => {
    client.setQueryDefaults(['evidence-detail', 'denied'], { refetchOnMount: false, retryOnMount: false });
    client.getQueryCache().build(client, { queryKey: ['evidence-detail', 'denied'] }).setState({ status: 'error', error: new Error('Không có quyền truy cập'), fetchStatus: 'idle' });
  });
  assert.match(denied, /Không có quyền truy cập/);
  assert.match(denied, /Thử lại/);
  assert.doesNotMatch(denied, /data-component="evidence-card"|<img/);
});

test('reference with a supplied record renders all real metadata and never grants review actions', () => {
  const html = render('EvidenceReferenceViewer', { reference: { id: record.id, record }, onClose() {} });
  for (const text of ['Nguồn hồ sơ', 'Gửi và rà soát', 'Người gửi thật', 'Người rà soát thật', 'Đầu việc thật', 'Ghi chú thật', 'Chi tiết kỹ thuật']) assert.ok(html.includes(text), text);
  assert.doesNotMatch(html, />Chấp nhận<|>Từ chối<|Rà soát minh chứng/);
});

test('ID-only and legacy external references keep honest loading/source states', () => {
  const props = { reference: { id: 'unknown', name: 'Unknown' }, onClose() {} };
  const loading = render('EvidenceReferenceViewer', props);
  assert.match(loading, /Đang tải thông tin minh chứng/);
  assert.doesNotMatch(loading, /Người gửi thật|Chưa upload|Chi tiết kỹ thuật/);
  const thinRecord = render('EvidenceReferenceViewer', { reference: { id: 'thin', record: { id: 'thin', created_at: record.created_at, name: 'Summary', status: 'submitted', work_item_id: 'item' } }, onClose() {} });
  assert.match(thinRecord, /Đang tải thông tin minh chứng/);
  assert.doesNotMatch(thinRecord, /Gửi và rà soát/);
  const external = render('EvidenceReferenceViewer', { reference: { name: 'Legacy', fileUrl: 'https://example.com/photo.jpg', evidenceType: 'photo' }, onClose() {} });
  assert.match(external, /Liên kết ngoài được lưu trong dữ liệu lịch sử/);
  assert.match(external, /src="https:\/\/example.com\/photo.jpg"/);
  const unsafe = render('EvidenceReferenceViewer', { reference: { name: 'Legacy', fileUrl: 'javascript:alert(1)', evidenceType: 'photo' }, onClose() {} });
  assert.doesNotMatch(unsafe, /javascript:/);
});

test('drafts and persisted records select the existing photo/PDF/Word/Excel readers and legacy fallback', () => {
  for (const [name, mime, mediaLabel] of [
    ['photo.png', 'image/png', 'Ảnh'], ['file.pdf', 'application/pdf', 'PDF'],
    ['file.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'Word'],
    ['file.xlsx', 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 'Excel'],
    ['file.doc', 'application/msword', 'Word'], ['file.xls', 'application/vnd.ms-excel', 'Excel'],
  ]) {
    const file = new File(['fixture'], name, { type: mime });
    const draft = render('EvidenceDraftViewerDialog', { open: true, name, originalName: name, file, fileSize: file.size, mimeType: mime, onClose() {} });
    assert.match(draft, /Thông tin tệp/);
    assert.ok(draft.includes(mediaLabel), name);
    assert.match(draft, /Chưa upload/);
    assert.doesNotMatch(draft, /Gửi và rà soát|Người gửi thật/);
    const persisted = render('EvidenceRecordViewerDialog', { record: { ...record, name, mime_type: mime }, showWorkItem: true, onClose() {} });
    assert.match(persisted, /Người gửi thật/);
    assert.ok(persisted.includes(mediaLabel), name);
    if (!name.endsWith('.png')) assert.doesNotMatch(draft, /Đang tải ảnh minh chứng|Không đọc được ảnh/);
    if (name.endsWith('.doc') || name.endsWith('.xls')) assert.match(draft, /Office đời cũ/);
  }
});

test('upload table locks persisted metadata and exposes bounded progress, retry and parent-save behavior', () => {
  const file = new File(['pdf'], 'file.pdf', { type: 'application/pdf' });
  const base = { id: 'row', file, name: 'Tên đã lưu', caption: 'Ghi chú', classification: { kind: 'pdf', label: 'PDF', evidenceType: 'document' }, fileIssue: '', progress: 100, error: '' };
  const props = { batchUploading: false, updateUploadRow() {}, onFileChange() {}, addUploadRow() {}, removeUploadRow() {}, retryUploadRow() {}, maxFiles: 1 };
  const successful = render('EvidenceUploader', { ...props, uploadRows: [{ ...base, status: 'success' }] });
  assert.match(successful, /<input(?=[^>]*value="Tên đã lưu")(?=[^>]*disabled="")[^>]*>/);
  assert.match(successful, /aria-valuenow="100"/);
  assert.doesNotMatch(successful, /Tải lên 1/);
  const failed = render('EvidenceUploader', { ...props, uploadRows: [{ ...base, status: 'error', error: 'Mất kết nối' }] });
  assert.match(failed, /Thử lại tệp 1/);
  assert.match(failed, /Mất kết nối/);
  assert.match(failed, /value="Tên đã lưu"/);
});

test('inventory locks capture, evidence references and the one legacy document entrypoint to shared owners', async () => {
  const root = new URL('../src/modules/', import.meta.url);
  const files = (await readdir(root, { recursive: true })).filter((path) => path.endsWith('.tsx'));
  const captureCounts: Record<string, number> = {};
  const referenceCounts: Record<string, number> = {};
  const documentCounts: Record<string, number> = {};
  const storedCardAdapter = await readFile(new URL('operations/ConstructionEvidenceButton.tsx', root), 'utf8');
  assert.match(storedCardAdapter, /<EvidenceCard\b/);
  assert.doesNotMatch(storedCardAdapter, /FileImage|<img\b|aspect-\[/, 'reference adapter must not own another card presentation');
  for (const path of files) {
    const source = await readFile(new URL(path, root), 'utf8');
    if (path !== '_shared/EvidenceRecordViewerDialog.tsx') assert.doesNotMatch(source, /<EvidenceViewerDialog\b/, path);
    if (path === '_shared/EvidenceCapture.tsx' || /<(?:ConstructionEvidenceCapture|EvidenceCapture)\b/.test(source)) assert.doesNotMatch(source, /reportMode|ReportEvidenceDraftEditor|EvidenceDraftCard/, path);
    if (path.startsWith('_shared/')) continue;
    const captures = (source.match(/<(?:ConstructionEvidenceCapture|EvidenceCapture)\b/g) ?? []).length;
    const references = (source.match(/<(?:ConstructionEvidenceButton|SelectedEvidenceViewButton|EvidenceReferenceViewButton|EvidenceReferenceViewer)\b/g) ?? []).length;
    const documents = (source.match(/<DocumentReferenceViewButton\b/g) ?? []).length;
    if (captures) captureCounts[path] = captures;
    if (references && path !== 'operations/ConstructionEvidenceButton.tsx') referenceCounts[path] = references;
    if (documents) documentCounts[path] = documents;
  }
  assert.deepEqual(captureCounts, {
    'acceptance/AcceptanceEditor.tsx': 1, 'operations/ConstructionActionDialogs.tsx': 2, 'operations/ConstructionCheckDialogs.tsx': 2,
    'operations/ConstructionReportEvidenceDialog.tsx': 1, 'operations/ConstructionReportFormDialog.tsx': 1, 'operations/ConstructionReportLinkedComposer.tsx': 1,
  });
  assert.deepEqual(referenceCounts, {
    'acceptance/AcceptanceEditor.tsx': 1, 'acceptance/AcceptancePage.tsx': 1, 'handover/HandoverChecklistSourceDialog.tsx': 1,
    'operations/ConstructionActionDialogs.tsx': 3, 'operations/ConstructionCheckDialogs.tsx': 2,
    'work-items/IncomingAcceptancePanel.tsx': 1, 'work-items/MaterialSnapshotPanel.tsx': 2, 'work-items/OwnerClientReconciliationPanel.tsx': 1,
    'work-items/SubcontractorReconciliationPanel.tsx': 1, 'work-items/SurplusHandlingPanel.tsx': 1,
    'work-items/DesignReleasePanel.tsx': 1, 'work-items/DiscussionWorkUpdateCard.tsx': 3, 'work-items/WorkflowFormPanel.tsx': 1,
  });
  assert.deepEqual(documentCounts, { 'work-items/DiscussionWorkUpdateCard.tsx': 1 });
});

test('new design evidence uses the evidence viewer while historical snapshots retain the document adapter', async () => {
  const discussion = await readFile(new URL('../src/modules/work-items/DiscussionWorkUpdateCard.tsx', import.meta.url), 'utf8');
  const design = await readFile(new URL('../src/modules/work-items/DesignReleasePanel.tsx', import.meta.url), 'utf8');
  for (const [path, source] of [['DiscussionWorkUpdateCard.tsx', discussion], ['DesignReleasePanel.tsx', design]]) {
    assert.doesNotMatch(source, /openProtectedBlob|window\.open/, path);
    assert.match(source, /<EvidenceReferenceViewButton\b/, path);
  }
  assert.match(discussion, /<DocumentReferenceViewButton\b/, 'legacy document snapshots remain readable');
  assert.doesNotMatch(design, /<DocumentReferenceViewButton\b/, 'canonical design files are evidence records');
});

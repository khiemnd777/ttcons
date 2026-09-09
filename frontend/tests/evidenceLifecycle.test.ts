import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';
import { renderToStaticMarkup } from 'react-dom/server';

const require = createRequire(import.meta.url);
const react = require('react');
const code = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export { useProtectedEvidencePreview, EvidenceDraftViewerDialog, ProtectedDocumentViewerDialog } from './modules/_shared/EvidenceRecordViewerDialog';
    export { DocumentReferenceViewButton } from './modules/_shared/DocumentReferenceViewer';
    export { DesignReleaseFiles } from './modules/work-items/DesignReleasePanel';
    export { DiscussionWorkUpdateCard } from './modules/work-items/DiscussionWorkUpdateCard';
    export { ConstructionEvidenceCapture } from './modules/_shared/EvidenceCapture';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'controlled-transport', setup(builder) {
    builder.onLoad({ filter: /\/lib\/api\/client\.ts$/ }, async ({ path }) => ({ contents: (await readFile(path, 'utf8')).replace('export async function downloadBlob(path: string) {', 'export async function downloadBlob(path: string) { return globalThis.__evidenceTestDownload(path);'), loader: 'ts' }));
    builder.onLoad({ filter: /DesignReleasePanel\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { DesignReleaseFiles };', loader: 'tsx' }));
    builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export function useAuth() { return { user: globalThis.__previewActor ?? { id: "actor" }, hasPermission: () => false }; }', loader: 'js' }));
    builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast() { return { warn() {}, error() {} }; }', loader: 'js' }));
  } }],
});

// Deterministic hook lifecycle harness: runs the actual hook/effect code with
// controlled HTTP and object URLs. It does not assert DOM/browser behavior.
function fixture() {
  const state: any[] = [];
  const effects: { deps?: any[]; cleanup?: () => void; pending?: () => any }[] = [];
  let cursor = 0;
  let effectCursor = 0;
  function useState(initial: any) {
    const index = cursor++;
    if (!(index in state)) state[index] = typeof initial === 'function' ? initial() : initial;
    return [state[index], (next: any) => { state[index] = typeof next === 'function' ? next(state[index]) : next; }];
  }
  const hooks = { ...react, useState,
    useRef: (value: any) => useState(() => ({ current: value }))[0],
    useCallback: (callback: any) => callback,
    useSyncExternalStore: (_subscribe: any, snapshot: any) => snapshot(),
    useEffect(callback: () => any, deps: any[]) {
      const index = effectCursor++;
      const current = effects[index];
      if (!current || !deps || deps.some((value, at) => !Object.is(value, current.deps?.[at]))) effects[index] = { ...current, deps, pending: callback };
    },
  };
  const module = { exports: {} as Record<string, any> };
  new Function('require', 'module', 'exports', code.outputFiles[0].text)((name: string) => {
    if (name === 'react') return hooks;
    if (name === 'react-dom') return { ...require(name), createPortal: (children: any) => children };
    if (name === '@tanstack/react-query') return { ...require(name), useQuery: () => (globalThis as any).__previewQuery ?? {} };
    return require(name);
  }, module, module.exports);
  return {
    render(name: string, ...args: any[]) { cursor = 0; effectCursor = 0; return module.exports[name](...args); },
    flush() { for (const effect of effects) if (effect.pending) { effect.cleanup?.(); effect.cleanup = effect.pending(); effect.pending = undefined; } },
    close() { for (const effect of effects) effect.cleanup?.(); },
  };
}

function deferred() {
  let resolve!: (blob: Blob) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<Blob>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
const record = (id: string) => ({ id, storage_provider: 'local', storage_key: `file/${id}`, updated_at: '2026-09-07' });
const settle = async () => { await new Promise(resolve => setImmediate(resolve)); };

test('protected preview discards a late response, hides previous files immediately and revokes URLs on close', async (t) => {
  const requests = new Map<string, ReturnType<typeof deferred>>();
  (globalThis as any).__evidenceTestDownload = (path: string) => { const pending = deferred(); requests.set(path, pending); return pending.promise; };
  const urls: string[] = [];
  const revoked: string[] = [];
  t.mock.method(URL, 'createObjectURL', () => { const url = `blob:fixture-${urls.length}`; urls.push(url); return url; });
  t.mock.method(URL, 'revokeObjectURL', (url: string) => revoked.push(url));
  const hooks = fixture();
  hooks.render('useProtectedEvidencePreview', record('first'), 'photo'); hooks.flush();
  hooks.render('useProtectedEvidencePreview', record('second'), 'photo'); hooks.flush();
  requests.get('/evidence/first/file')!.resolve(new Blob(['first'], { type: 'image/png' })); await settle();
  assert.equal(urls.length, 0, 'late response must never create a visible URL');
  requests.get('/evidence/second/file')!.resolve(new Blob(['second'], { type: 'image/png' })); await settle();
  const ready = hooks.render('useProtectedEvidencePreview', record('second'), 'photo');
  assert.equal(ready.previewUrl, 'blob:fixture-0');
  const switched = hooks.render('useProtectedEvidencePreview', record('third'), 'photo');
  assert.equal(switched.previewUrl, undefined);
  hooks.flush();
  assert.deepEqual(revoked, ['blob:fixture-0']);
  hooks.close();
  requests.get('/evidence/third/file')!.resolve(new Blob(['third'])); await settle();
  assert.equal(urls.length, 1);
});

test('revision paths are part of preview identity; forbidden/missing file errors can retry without stale content', async () => {
  const paths: string[] = [];
  const pending: ReturnType<typeof deferred>[] = [];
  (globalThis as any).__evidenceTestDownload = (path: string) => { paths.push(path); const request = deferred(); pending.push(request); return request.promise; };
  const hooks = fixture();
  const revision1 = '/work-reports/report/revisions/1/evidence/ev/file';
  const revision2 = '/work-reports/report/revisions/2/evidence/ev/file';
  hooks.render('useProtectedEvidencePreview', record('ev'), 'pdf', revision1); hooks.flush();
  pending[0].reject(new Error('403 forbidden')); await settle();
  const denied = hooks.render('useProtectedEvidencePreview', record('ev'), 'pdf', revision1);
  assert.match(denied.error, /403 forbidden/);
  assert.equal(denied.previewUrl, undefined);
  denied.retry();
  hooks.render('useProtectedEvidencePreview', record('ev'), 'pdf', revision1); hooks.flush();
  pending[1].resolve(new Blob(['%PDF-1.4'], { type: 'application/octet-stream' })); await settle();
  const ready = hooks.render('useProtectedEvidencePreview', record('ev'), 'pdf', revision1);
  assert.equal(ready.blob.type, 'application/pdf');
  assert.equal(hooks.render('useProtectedEvidencePreview', record('ev'), 'pdf', revision2).previewUrl, undefined);
  hooks.flush();
  pending[2].reject(new Error('404 missing file')); await settle();
  assert.match(hooks.render('useProtectedEvidencePreview', record('ev'), 'pdf', revision2).error, /404 missing file/);
  assert.deepEqual(paths, [revision1, revision1, revision2]);
  hooks.close();
});

test('draft URL belongs to its File and is released when replaced or closed', (t) => {
  const revoked: string[] = [];
  let number = 0;
  t.mock.method(URL, 'createObjectURL', () => `blob:draft-${++number}`);
  t.mock.method(URL, 'revokeObjectURL', (url: string) => revoked.push(url));
  const hooks = fixture();
  const props = { open: true, file: new File(['one'], 'one.pdf'), name: 'one', originalName: 'one.pdf', fileSize: 3, onClose() {} };
  hooks.render('EvidenceDraftViewerDialog', props); hooks.flush();
  assert.equal(hooks.render('EvidenceDraftViewerDialog', props).props.previewUrl, 'blob:draft-1');
  const next = { ...props, file: new File(['two'], 'two.docx'), originalName: 'two.docx' };
  assert.equal(hooks.render('EvidenceDraftViewerDialog', next).props.previewUrl, undefined);
  hooks.flush();
  const viewer = hooks.render('EvidenceDraftViewerDialog', next);
  assert.equal(viewer.props.mediaType, 'word');
  assert.equal(viewer.props.fileBlob, next.file);
  hooks.render('EvidenceDraftViewerDialog', { ...next, open: false }); hooks.flush();
  assert.deepEqual(revoked, ['blob:draft-1', 'blob:draft-2']);
  hooks.close();
});

test('capture preserves source selection intent, enforces file caps, and never uploads during selection', (t) => {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { requestAnimationFrame: () => 1, cancelAnimationFrame() {} } });
  t.after(() => { if (previousWindow) Object.defineProperty(globalThis, 'window', previousWindow); else delete (globalThis as any).window; });
  let drafts: any[] = [];
  let resume!: () => void;
  let requests = 0;
  (globalThis as any).__evidenceTestDownload = () => { requests++; throw new Error('unexpected transport'); };
  const hooks = fixture();
  const props = () => ({ drafts, hasSource: false, uploadScope: '/work-items/item/evidence/upload', maxFiles: 1,
    onSourceRequired(_intent: string, callback: () => void) { resume = callback; }, onChange(next: any[]) { drafts = next; } });
  const uploader = () => hooks.render('ConstructionEvidenceCapture', props()).props.children.find((child: any) => child?.type?.name === 'EvidenceUploader');
  const file = new File(['pdf'], 'file.pdf', { type: 'application/pdf' });
  uploader().props.onFileChange('evidence_files', file, { value: 'selection' });
  assert.equal(drafts.length, 0);
  resume();
  assert.equal(drafts[0].file, file);
  uploader().props.onFileChange('not-an-existing-row', file, { value: 'selection' });
  resume();
  assert.equal(drafts.length, 1);
  assert.equal(requests, 0);
  assert.equal(uploader().props.maxFiles, 1);
  uploader().props.removeUploadRow(drafts[0].id);
  assert.deepEqual(drafts, []);
  hooks.close();
});

function findElement(node: any, predicate: (element: any) => boolean): any {
  if (!node || typeof node !== 'object') return undefined;
  if (predicate(node)) return node;
  for (const child of [node.props?.children].flat(Infinity)) {
    const match = findElement(child, predicate);
    if (match) return match;
  }
}

test('clicking historical and canonical design files opens the standard viewer with the exact document and closes back', (t) => {
  const previousDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  Object.defineProperty(globalThis, 'document', { configurable: true, value: { body: {} } });
  t.after(() => {
    if (previousDocument) Object.defineProperty(globalThis, 'document', previousDocument); else delete (globalThis as any).document;
    delete (globalThis as any).__previewQuery;
    delete (globalThis as any).__previewActor;
  });
  const historical = { id: 'document-r01', name: 'old.pdf', mime_type: 'application/pdf', file_size: 1024 };
  const current = { ...historical, id: 'document-r02', name: 'new.pdf' };
  (globalThis as any).__previewQuery = { data: { source_type: 'design_release', snapshot: { revision_label: 'R01', files: [historical] } } };
  const cardHooks = fixture();
  const props = { item: { id: 'activity', activity_type: 'work_update', metadata_json: { work_update: { source_type: 'design_release', revision_label: 'R01' } } } };
  findElement(cardHooks.render('DiscussionWorkUpdateCard', props), node => node.type === 'button').props.onClick();
  const oldEntry = findElement(cardHooks.render('DiscussionWorkUpdateCard', props), node => node.type?.name === 'DocumentReferenceViewButton');
  const fileHooks = fixture();
  const newEntry = findElement(fileHooks.render('DesignReleaseFiles', { files: [current], revisionLabel: 'R02' }), node => node.type?.name === 'DocumentReferenceViewButton');
  for (const [entry, file, revision] of [[oldEntry, historical, 'R01'], [newEntry, current, 'R02']] as const) {
    assert.ok(entry, 'actual consumer must render the shared document entrypoint');
    assert.deepEqual(entry.props.file, file);
    const hooks = fixture();
    let tree = hooks.render('DocumentReferenceViewButton', entry.props);
    assert.equal(findElement(tree, node => node.type?.name === 'ProtectedDocumentViewerDialog'), undefined);
    findElement(tree, node => node.type === 'button').props.onClick();
    tree = hooks.render('DocumentReferenceViewButton', entry.props);
    const viewer = findElement(tree, node => node.type?.name === 'ProtectedDocumentViewerDialog');
    assert.equal(viewer.props.protectedFilePath, `/documents/${file.id}/file`);
    assert.equal(viewer.props.mimeType, file.mime_type);
    assert.match(viewer.props.sourceLabel, new RegExp(revision));
    (globalThis as any).__previewActor = { id: 'another-actor' };
    const changed = findElement(hooks.render('DocumentReferenceViewButton', entry.props), node => node.type?.name === 'ProtectedDocumentViewerDialog');
    assert.equal(changed.key, `another-actor:${file.id}`, 'actor changes remount and reauthorize the protected preview');
    delete (globalThis as any).__previewActor;
    viewer.props.onClose();
    assert.equal(findElement(hooks.render('DocumentReferenceViewButton', entry.props), node => node.type?.name === 'ProtectedDocumentViewerDialog'), undefined);
    hooks.close();
  }
  assert.equal(findElement(cardHooks.render('DiscussionWorkUpdateCard', props), node => node.type?.name === 'DocumentReferenceViewButton').props.file.id, historical.id);
  cardHooks.close(); fileHooks.close();
});

test('protected documents render through existing photo/PDF/Office readers without fabricated evidence metadata', async () => {
  const paths: string[] = [];
  (globalThis as any).__evidenceTestDownload = async (path: string) => { paths.push(path); return new Blob(['file'], { type: 'application/octet-stream' }); };
  for (const [name, mediaType] of [['photo.png', 'photo'], ['drawing.pdf', 'pdf'], ['spec.docx', 'word'], ['schedule.xlsx', 'spreadsheet'], ['drawing.dwg', 'document']]) {
    const hooks = fixture();
    const props = { name, protectedFilePath: `/documents/${name}/file`, sourceLabel: 'Hồ sơ thiết kế · R01', onClose() {} };
    const initial = hooks.render('ProtectedDocumentViewerDialog', props);
    assert.equal(initial.props.loading, true);
    assert.equal(initial.props.mediaType, mediaType);
    hooks.flush(); await settle();
    const ready = hooks.render('ProtectedDocumentViewerDialog', props);
    assert.equal(ready.type.name, 'EvidenceViewerDialog');
    assert.equal(ready.props.loading, false);
    assert.ok(ready.props.previewUrl?.startsWith('blob:'));
    assert.ok(ready.props.fileBlob instanceof Blob);
    const html = renderToStaticMarkup(ready);
    assert.match(html, /Xem tệp/);
    assert.match(html, /Hồ sơ thiết kế · R01/);
    assert.doesNotMatch(html, /Người rà soát|Thời điểm rà soát|Chưa upload/);
    if (mediaType === 'photo') assert.match(html, /<img/);
    if (mediaType === 'pdf') assert.match(html, /<iframe/);
    if (mediaType === 'document') assert.match(html, /Định dạng này chưa hỗ trợ đọc trực tiếp/);
    hooks.close();
  }
  assert.deepEqual(paths, ['photo.png', 'drawing.pdf', 'spec.docx', 'schedule.xlsx', 'drawing.dwg'].map(name => `/documents/${name}/file`));
});

test('document preview ignores stale downloads, revokes old URLs and retries denied files through the same source', async (t) => {
  const requests: ReturnType<typeof deferred>[] = [];
  const paths: string[] = [];
  (globalThis as any).__evidenceTestDownload = (path: string) => { paths.push(path); const request = deferred(); requests.push(request); return request.promise; };
  const revoked: string[] = [];
  let urls = 0;
  t.mock.method(URL, 'createObjectURL', () => `blob:document-${++urls}`);
  t.mock.method(URL, 'revokeObjectURL', (url: string) => revoked.push(url));
  const hooks = fixture();
  const old = { name: 'old.pdf', protectedFilePath: '/documents/old/file', onClose() {} };
  const next = { ...old, name: 'new.pdf', protectedFilePath: '/documents/new/file' };
  hooks.render('ProtectedDocumentViewerDialog', old); hooks.flush();
  hooks.render('ProtectedDocumentViewerDialog', next); hooks.flush();
  requests[0].resolve(new Blob(['old'])); await settle();
  assert.equal(urls, 0);
  requests[1].reject(new Error('403 forbidden')); await settle();
  const denied = hooks.render('ProtectedDocumentViewerDialog', next);
  assert.match(denied.props.error, /403 forbidden/);
  assert.equal(denied.props.previewUrl, undefined);
  assert.equal(denied.props.mediaAction, undefined);
  denied.props.onRetry(); hooks.render('ProtectedDocumentViewerDialog', next); hooks.flush();
  requests[2].resolve(new Blob(['new'])); await settle();
  assert.equal(hooks.render('ProtectedDocumentViewerDialog', next).props.previewUrl, 'blob:document-1');
  assert.equal(hooks.render('ProtectedDocumentViewerDialog', old).props.previewUrl, undefined);
  hooks.flush();
  assert.deepEqual(revoked, ['blob:document-1']);
  hooks.close();
  requests[3].resolve(new Blob(['late'])); await settle();
  assert.equal(urls, 1);
  assert.deepEqual(paths, ['/documents/old/file', '/documents/new/file', '/documents/new/file', '/documents/old/file']);
});

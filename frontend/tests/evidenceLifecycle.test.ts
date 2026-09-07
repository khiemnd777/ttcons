import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';

const require = createRequire(import.meta.url);
const react = require('react');
const code = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export { useProtectedEvidencePreview, EvidenceDraftViewerDialog } from './modules/_shared/EvidenceRecordViewerDialog';
    export { ConstructionEvidenceCapture } from './modules/_shared/EvidenceCapture';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'controlled-transport', setup(builder) {
    builder.onLoad({ filter: /\/lib\/api\/client\.ts$/ }, () => ({ contents: 'export function downloadBlob(path) { return globalThis.__evidenceTestDownload(path); }', loader: 'js' }));
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
  new Function('require', 'module', 'exports', code.outputFiles[0].text)((name: string) => name === 'react' ? hooks : require(name), module, module.exports);
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
const settle = async () => { await Promise.resolve(); await Promise.resolve(); };

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

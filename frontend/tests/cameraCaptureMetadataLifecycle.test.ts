import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';

const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export { ConstructionEvidenceCapture, CameraCaptureModal, constructionEvidenceFormData } from './modules/_shared/EvidenceCapture';
    export { EvidenceUploader } from './modules/_shared/EvidenceUploader';
    export { uploadEvidenceDraftBatch } from './modules/_shared/evidenceUploadSession';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///camera-fixture"' },
  plugins: [{ name: 'camera-boundary', setup(builder) {
    builder.onLoad({ filter: /\/EvidenceCapture\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { CameraCaptureModal };', loader: 'tsx' }));
    builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast() { return { warn() {}, error() {} }; }', loader: 'js' }));
  } }],
});

// Run production hooks, effects and callbacks with deferred encoding and a
// controlled camera boundary. This fixture does not prove device/browser UX.
function fixture() {
  const state: any[] = [];
  const effects: { deps?: any[]; cleanup?: () => void; pending?: () => any }[] = [];
  let cursor = 0; let effectCursor = 0;
  function useState(initial: any) {
    const index = cursor++;
    if (!(index in state)) state[index] = typeof initial === 'function' ? initial() : initial;
    return [state[index], (next: any) => { state[index] = typeof next === 'function' ? next(state[index]) : next; }];
  }
  const hooks = { ...react, useState, useRef: (value: any) => useState(() => ({ current: value }))[0],
    useCallback: (callback: any) => callback,
    useSyncExternalStore: (_subscribe: any, snapshot: any) => snapshot(),
    useEffect(callback: () => any, deps: any[]) {
      const index = effectCursor++; const current = effects[index];
      if (!current || !deps || deps.some((value, at) => !Object.is(value, current.deps?.[at]))) effects[index] = { ...current, deps, pending: callback };
    },
  };
  const module = { exports: {} as Record<string, any> };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === 'react' ? hooks : require(name), module, module.exports);
  return {
    api: module.exports,
    render(name: string, props: any) { cursor = 0; effectCursor = 0; return module.exports[name](props); },
    flush() { for (const effect of effects) if (effect.pending) { effect.cleanup?.(); effect.cleanup = effect.pending(); effect.pending = undefined; } },
    close() { for (const effect of effects) effect.cleanup?.(); },
  };
}

function nodes(tree: any, result: any[] = []): any[] {
  if (!tree || typeof tree !== 'object') return result;
  result.push(tree);
  for (const child of [tree.props?.children, tree.props?.footer].flat(Infinity)) nodes(child, result);
  return result;
}
function text(tree: any): string {
  if (typeof tree === 'string') return tree;
  if (!tree || typeof tree !== 'object') return '';
  return [tree.props?.children].flat(Infinity).map(text).join('');
}
function click(tree: any, label: string) {
  const button = nodes(tree).find(node => node.type === 'button' && text(node).trim() === label);
  assert.ok(button, label); assert.ok(!button.props.disabled, label);
  button.props.onClick();
}
function child(tree: any, name: string) { const found = nodes(tree).find(node => node.type?.name === name); assert.ok(found, name); return found; }
const settle = async () => { await new Promise(resolve => setImmediate(resolve)); };

async function cameraFixture(t: any, onUse: (file: File, capturedAt: string) => boolean) {
  const start = Date.parse('2026-10-09T01:02:03.456Z');
  t.mock.timers.enable({ apis: ['Date'], now: start });
  const pending: ((blob: Blob | null) => void)[] = [];
  const frameTimes: string[] = [];
  const video = { videoWidth: 640, videoHeight: 480, play: async () => {}, srcObject: null };
  let stops = 0;
  const restoreNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');
  const restoreDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { mediaDevices: { getUserMedia: async () => ({ getTracks: () => [{ stop() { stops++; } }] }) } } });
  Object.defineProperty(globalThis, 'document', { configurable: true, value: { createElement(tag: string) {
    assert.equal(tag, 'canvas');
    return { getContext: () => ({ drawImage() { frameTimes.push(new Date().toISOString()); } }), toBlob(callback: (blob: Blob | null) => void) { pending.push(callback); } };
  } } });
  t.after(() => {
    if (restoreNavigator) Object.defineProperty(globalThis, 'navigator', restoreNavigator); else delete (globalThis as any).navigator;
    if (restoreDocument) Object.defineProperty(globalThis, 'document', restoreDocument); else delete (globalThis as any).document;
  });
  t.mock.method(URL, 'createObjectURL', () => 'blob:camera-photo');
  t.mock.method(URL, 'revokeObjectURL', () => {});
  const hooks = fixture(); let closed = 0;
  const props = { open: true, onUse, onClose() { closed++; }, onFallback() {} };
  const render = () => hooks.render('CameraCaptureModal', props);
  const initial = render(); nodes(initial).find(node => node.type === 'video').ref.current = video;
  hooks.flush(); await settle();
  t.after(() => hooks.close());
  return { render, hooks, pending, frameTimes, start, closed: () => closed, stops: () => stops };
}

test('deferred encoding, review, continuous shots and multipart retain each actual shutter instant', async (t) => {
  const queue = fixture(); let drafts: any[] = [];
  const props = () => ({ drafts, onChange(next: any[]) { drafts = next; }, hasSource: true, uploadScope: 'live-camera' });
  const capture = await cameraFixture(t, (file, capturedAt) => child(queue.render('ConstructionEvidenceCapture', props()), 'CameraCaptureModal').props.onUse(file, capturedAt));
  for (let index = 0; index < 2; index++) {
    t.mock.timers.setTime(capture.start + index * 60_000);
    click(capture.render(), 'Chụp ảnh');
    const shutter = new Date().toISOString();
    t.mock.timers.tick(12_000); capture.pending.shift()!(new Blob([`photo-${index}`], { type: 'image/jpeg' }));
    t.mock.timers.tick(20_000);
    click(capture.render(), index === 0 ? 'Dùng & chụp tiếp' : 'Xong');
    assert.equal(drafts[index].captureMetadata.captured_at, shutter);
    assert.equal(capture.frameTimes[index], shutter);
    assert.ok(drafts[index].file.name.includes(shutter.replace(/[:.]/g, '-')));
    const form = queue.api.constructionEvidenceFormData(drafts[index], 'work-item');
    assert.equal(JSON.parse(String(form.get('payload'))).capture_metadata.captured_at, shutter);
    capture.render(); capture.hooks.flush();
  }
  assert.notEqual(drafts[0].captureMetadata.captured_at, drafts[1].captureMetadata.captured_at);
  assert.equal(capture.closed(), 1);
  const attempts = new Map<string, string[]>();
  const worker = async (draft: any) => {
    const instant = JSON.parse(String(queue.api.constructionEvidenceFormData(draft, 'work-item').get('payload'))).capture_metadata.captured_at;
    const times = [...(attempts.get(draft.id) ?? []), instant]; attempts.set(draft.id, times);
    if (times.length === 1) throw new Error('retry each camera upload');
    return draft.id;
  };
  t.mock.timers.tick(600_000);
  await assert.rejects(queue.api.uploadEvidenceDraftBatch(drafts, 'live-camera', worker), /retry each camera upload/);
  const uploader = child(queue.render('ConstructionEvidenceCapture', props()), 'EvidenceUploader');
  for (const draft of drafts) uploader.props.retryUploadRow(draft.id);
  await settle();
  for (const draft of drafts) assert.deepEqual(attempts.get(draft.id), [draft.captureMetadata.captured_at, draft.captureMetadata.captured_at]);
  assert.ok(child(queue.render('ConstructionEvidenceCapture', props()), 'EvidenceUploader').props.uploadRows.every((row: any) => row.status === 'success'));
});

test('retake discards previous photo time; a rejected use preserves the same frame and instant for retry', async (t) => {
  const used: { file: File; capturedAt: string }[] = []; let allowUse = false;
  const capture = await cameraFixture(t, (file, capturedAt) => { used.push({ file, capturedAt }); return allowUse; });
  click(capture.render(), 'Chụp ảnh'); capture.pending.shift()!(new Blob(['discard']));
  click(capture.render(), 'Chụp lại'); capture.render(); capture.hooks.flush();
  t.mock.timers.tick(45_000);
  click(capture.render(), 'Chụp ảnh'); const shutter = new Date().toISOString();
  t.mock.timers.tick(9_000); capture.pending.shift()!(new Blob(['keep']));
  click(capture.render(), 'Dùng & chụp tiếp');
  t.mock.timers.tick(30_000); allowUse = true;
  click(capture.render(), 'Dùng & chụp tiếp');
  assert.equal(used.length, 2); assert.equal(used[0].file, used[1].file);
  assert.equal(used[0].capturedAt, shutter); assert.equal(used[1].capturedAt, shutter);
  assert.notEqual(shutter, capture.frameTimes[0]);
});

test('encoding failure and close invalidate pending frames without creating timestamped drafts', async (t) => {
  const used: any[] = [];
  const capture = await cameraFixture(t, (...values) => { used.push(values); return true; });
  click(capture.render(), 'Chụp ảnh'); capture.pending.shift()!(null);
  assert.match(text(capture.render()), /Không tạo được ảnh/);
  click(capture.render(), 'Chụp ảnh');
  capture.render().props.onClose();
  capture.pending.shift()!(new Blob(['late']));
  assert.equal(nodes(capture.render()).some(node => node.type === 'img'), false);
  assert.deepEqual(used, []); assert.equal(capture.closed(), 1); assert.ok(capture.stops() > 0);
});

test('imports and system-camera fallback never invent capture time; replacement resets earlier metadata', () => {
  const queue = fixture(); let drafts: any[] = [];
  const props = () => ({ drafts, onChange(next: any[]) { drafts = next; }, hasSource: true, uploadScope: 'imports' });
  const imported = () => new File(['photo'], 'hien-truong-2020-01-01T00-00-00-000Z.jpg', { type: 'image/jpeg', lastModified: Date.parse('2001-01-01') });
  const tree = queue.render('ConstructionEvidenceCapture', props());
  for (const input of nodes(tree).filter(node => node.type === 'input' && node.props.type === 'file')) input.props.onChange({ target: { files: [imported()], value: 'selected' } });
  assert.equal(drafts.length, 2);
  for (const draft of drafts) { assert.deepEqual(draft.captureMetadata, {}); assert.equal(queue.api.constructionEvidenceFormData(draft, 'item').has('payload'), false); }
  drafts[0].captureMetadata = { captured_at: '2026-10-09T01:02:03.456Z', location: 'Site' }; drafts[0].caption = 'Keep caption';
  const originalId = drafts[0].id;
  child(queue.render('ConstructionEvidenceCapture', props()), 'EvidenceUploader').props.onFileChange(originalId, imported(), { value: 'selected' });
  assert.notEqual(drafts[0].id, originalId); assert.deepEqual(drafts[0].captureMetadata, {}); assert.equal(drafts[0].caption, 'Keep caption');
});

test('manual edit and explicit clear survive upload retry without restoring the camera time', async () => {
  const queue = fixture(); const editor = fixture(); let drafts: any[] = [];
  const scope = 'camera-clear-retry';
  const props = () => ({ drafts, onChange(next: any[]) { drafts = next; }, hasSource: true, uploadScope: scope });
  child(queue.render('ConstructionEvidenceCapture', props()), 'CameraCaptureModal').props.onUse(new File(['photo'], 'camera.jpg', { type: 'image/jpeg' }), '2026-10-09T01:02:03.456Z');
  for (const value of ['2026-10-10T09:30', '']) {
    const uploader = child(queue.render('ConstructionEvidenceCapture', props()), 'EvidenceUploader');
    const tree = editor.render('EvidenceUploader', uploader.props);
    nodes(tree).find(node => node.props?.['aria-label'] === 'Thời điểm chụp ảnh 1').props.onChange({ target: { value } });
    if (value) assert.equal(drafts[0].captureMetadata.captured_at, new Date(value).toISOString());
  }
  let attempts = 0;
  const worker = async (draft: any) => {
    attempts++;
    assert.equal(draft.captureMetadata.captured_at, '');
    assert.equal(queue.api.constructionEvidenceFormData(draft, 'item').has('payload'), false);
    if (attempts === 1) throw new Error('transient upload failure');
    return 'evidence';
  };
  await assert.rejects(queue.api.uploadEvidenceDraftBatch(drafts, scope, worker), /transient upload failure/);
  assert.deepEqual(await queue.api.uploadEvidenceDraftBatch(drafts, scope, worker), ['evidence']);
  assert.equal(attempts, 2); assert.equal(drafts[0].captureMetadata.captured_at, '');
});

import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { createCameraCaptureSession } from '../src/modules/_shared/cameraCaptureSession.ts';

// Use the same CJS context as the in-memory component bundle, not a second ESM context.
const { QueryClient, QueryClientProvider } = createRequire(import.meta.url)('@tanstack/react-query');

// Real components and fixed query-cache fixtures. No DOM layout, browser, camera or HTTP.
const bundle = await build({
  stdin: {
    contents: `export { ConstructionReportFormDialog, CrewEditor, MaterialEditor } from './modules/operations/ConstructionReportFormDialog';
      export { ConstructionEvidenceCapture, CameraCaptureModal } from './modules/_shared/EvidenceCapture';
      export { ToastProvider } from './components/ToastProvider';
      export { Dialog } from './components/Dialog';`,
    resolveDir: fileURLToPath(new URL('../src/', import.meta.url)),
  },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///non-browser-test"' },
  plugins: [{ name: 'test-private-editors', setup(builder) {
    builder.onLoad({ filter: /ConstructionReportFormDialog\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { CrewEditor, MaterialEditor };', loader: 'tsx' }));
    builder.onLoad({ filter: /\/EvidenceCapture\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { CameraCaptureModal };', loader: 'tsx' }));
  } }],
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(createRequire(import.meta.url), module, module.exports);
const components = module.exports;
function render(component: any, props: object, client = new QueryClient()) {
  try {
    return renderToStaticMarkup(createElement(QueryClientProvider, { client }, createElement(components.ToastProvider, null, createElement(component, props))));
  } finally { client.clear(); }
}

test('late camera callbacks after close cannot create a ghost photo or finish the next capture', () => {
  const session = createCameraCaptureSession();
  const beforeClose = session.begin()!;
  session.invalidate();
  const afterReopen = session.begin()!;
  assert.notEqual(afterReopen, beforeClose);
  assert.equal(session.finish(beforeClose), false);
  assert.equal(session.begin(), null);
  assert.equal(session.finish(afterReopen), true);
});

test('continuous camera capture serializes shutter presses and consumes each result once', () => {
  const session = createCameraCaptureSession();
  for (let index = 0; index < 10; index++) {
    const ticket = session.begin()!;
    assert.notEqual(ticket, null);
    assert.equal(session.begin(), null);
    assert.equal(session.finish(ticket), true);
    assert.equal(session.finish(ticket), false);
  }
});

test('ordinary dialogs retain opt-in fullscreen while all capture consumers share the existing upload table', () => {
  const dialog = { open: true, title: 'Default', onClose() {}, children: 'Body' };
  assert.doesNotMatch(render(components.Dialog, dialog), /dialog-mobile-fullscreen/);
  assert.match(render(components.Dialog, { ...dialog, mobileFullscreen: true }), /dialog-mobile-fullscreen/);
  const draft = { id: 'photo', file: new File(['image'], 'site.jpg', { type: 'image/jpeg' }), evidenceType: 'photo', fileKind: 'photo', name: 'Site', caption: '' };
  const props = { drafts: [draft], uploadScope: '/work-items/item/evidence/upload', hasSource: true, onChange() {} };
  const html = render(components.ConstructionEvidenceCapture, props);
  assert.match(html, /data-component="evidence-uploader"/);
  assert.match(html, /Mở camera/);
  assert.match(html, /aria-label="Tên minh chứng 1"/);
  assert.match(html, /Tiến độ|tiến độ/);
  assert.doesNotMatch(html, /report-photo-grid|Tệp và ảnh hiện trường/);
});

test('each resource has one editable control per value across responsive layouts', () => {
  const crew = render(components.CrewEditor, { rows: [{ crew_name: 'Tổ <A>', worker_count: '8', work_hours: '7.5', note: 'Ca chiều' }], onChange() {}, reveal: 0 });
  assert.equal((crew.match(/<input\b/g) ?? []).length, 4);
  assert.match(crew, /value="Tổ &lt;A&gt;"/);
  assert.match(crew, /value="7.5"/);
  assert.match(crew, /data-label="Số người \*"/);
  assert.match(crew, /<details[^>]*open=""/);
  const materials = render(components.MaterialEditor, { rows: [{ material_name: 'Cát', quantity: '0', unit: 'm³', note: '' }], onChange() {}, reveal: 0 });
  assert.equal((materials.match(/<input\b/g) ?? []).length, 4);
  assert.match(materials, /value="0"/);
  assert.match(materials, /data-label="ĐVT \*"/);
});

test('empty resources collapse while incomplete rows remain open with an error', () => {
  const props = { rows: [{ crew_name: '', worker_count: '', work_hours: '', note: '' }], onChange() {}, reveal: 0 };
  assert.doesNotMatch(render(components.CrewEditor, props), /<details[^>]*open=/);
  const invalid = render(components.CrewEditor, { ...props, error: 'Thiếu số người', reveal: 1 });
  assert.match(invalid, /<details[^>]*open=""/);
  assert.match(invalid, /role="alert"[^>]*>Thiếu số người/);
});

test('report prioritizes photos, preserves required fields, and keeps draft/send permissions separate', () => {
  const client = new QueryClient();
  client.setQueryData(['project-operations', 'project', 'full'], { work_items: [] });
  client.setQueryData(['work-report-recipients', 'project'], []);
  const props = { open: true, projectId: 'project', projectName: 'Công trình <A>', onClose() {}, onSubmit: async () => {} };
  const html = render(components.ConstructionReportFormDialog, { ...props, canSubmitWorkReport: true }, client);
  assert.match(html, /Công trình &lt;A&gt;/);
  assert.ok(html.indexOf('Ảnh hiện trường') < html.indexOf('Công việc trong ngày'));
  for (const label of ['Ngày báo cáo', 'Thời tiết', 'Tiến độ (%)', 'Hạng mục thi công']) assert.ok(html.includes(label));
  assert.match(html, /Chưa có hạng mục Thi công khả dụng/);
  assert.match(html, /Báo cáo chỉ có thể lưu nháp/);
  assert.match(html, /name="delivery_mode" value="draft"/);
  assert.match(html, /name="delivery_mode" value="submit" disabled=""/);
  const withoutPermission = render(components.ConstructionReportFormDialog, props);
  assert.doesNotMatch(withoutPermission, /value="submit"/);
  assert.match(withoutPermission, /report-actions-single/);
});

test('report camera exposes loading and system-camera recovery while the shutter is unavailable', () => {
  const props = { open: true, onClose() {}, onUse: () => true, onFallback() {} };
  const report = render(components.CameraCaptureModal, { ...props, selectedCount: 3 });
  assert.match(report, /Đã chọn: 3 ảnh/);
  assert.match(report, /Đang mở camera/);
  assert.match(report, /Camera hệ thống/);
  assert.match(report, /dialog-mobile-fullscreen/);
  const regular = render(components.CameraCaptureModal, props);
  assert.match(regular, /dialog-mobile-fullscreen/);
});

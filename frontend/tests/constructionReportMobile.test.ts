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
    contents: `export { ConstructionReportFormDialog, CrewEditor, MaterialEditor, MaterialArrivalEditor, NextPlanEditor, BlockerEditor } from './modules/operations/ConstructionReportFormDialog';
      export { constructionReportEditModel } from './modules/operations/constructionReportViewModel';
      export { ConstructionActionDialogs } from './modules/operations/ConstructionActionDialogs';
      export { ConstructionEvidenceCapture, CameraCaptureModal } from './modules/_shared/EvidenceCapture';
      export { ToastProvider } from './components/ToastProvider';
      export { Dialog } from './components/Dialog';`,
    resolveDir: fileURLToPath(new URL('../src/', import.meta.url)),
  },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///non-browser-test"' },
  plugins: [{ name: 'test-private-editors', setup(builder) {
    builder.onLoad({ filter: /ConstructionReportFormDialog\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { CrewEditor, MaterialEditor, MaterialArrivalEditor, NextPlanEditor, BlockerEditor };', loader: 'tsx' }));
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
  assert.equal((crew.match(/<input\b/g) ?? []).length, 6);
  assert.match(crew, /aria-label="Đại diện tổ đội dòng 1"/);
  assert.match(crew, /aria-label="Liên hệ tổ đội dòng 1"/);
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

test('native crew contact facts keep canonical row identity and arrivals remain separate from usage', () => {
  const saved = components.constructionReportEditModel({ structured_json: { crew_items: [{ row_id: 'crew-1', crew_name: 'Tổ A', worker_count: 3, representative: 'Nguyễn A', contact: '0900' }], material_arrival_items: [{ row_id: 'arrival-1', material_name: 'Cát', quantity: null }], material_usage_items: [{ material_name: 'Thép', quantity: 2, unit: 'kg' }] } });
  assert.equal(saved.crewItems[0].row_id, 'crew-1');
  assert.equal(saved.crewItems[0].representative, 'Nguyễn A');
  assert.equal(saved.crewItems[0].contact, '0900');
  assert.equal(saved.materialArrivalItems[0].row_id, 'arrival-1');
  assert.equal(saved.materialArrivalItems[0].quantity, '');
  assert.equal(saved.materialItems[0].material_name, 'Thép');
  const html = render(components.MaterialArrivalEditor, { rows: saved.materialArrivalItems, onChange() {}, reveal: 0 });
  assert.match(html, /Vật tư nhập trường/);
  assert.match(html, /aria-label="Quy cách nhập trường dòng 1"/);
  assert.match(html, /aria-label="Xuất xứ nhập trường dòng 1"/);
});

test('native action capture keeps field severity distinct from priority and reuses shared photo capture', () => {
  const html = render(components.ConstructionActionDialogs, { state: { type: 'create' }, projectId: 'project', workspace: { work_items: [], participants: [] }, onClose() {}, onSaved() {} });
  assert.match(html, /Chi tiết hiện trường/);
  assert.match(html, /Mã lỗi/);
  assert.match(html, /Vị trí/);
  assert.match(html, /Đơn vị phụ trách/);
  assert.match(html, /Liên hệ 24\/7/);
  assert.match(html, /Mức độ lỗi/);
  assert.match(html, /Mức ưu tiên/);
  assert.match(html, /value="critical"/);
  assert.match(html, /value="urgent"/);
  assert.match(html, /data-component="evidence-uploader"/);
  assert.doesNotMatch(html, /BM-GS|Bổ sung thông tin/);
});

test('arrival zero quantity without unit points inline validation to the unit control', () => {
  const row = { row_id: 'arrival-1', material_name: 'Cát', specification: '', origin: '', quantity: '0', unit: '', note: '' };
  const html = render(components.MaterialArrivalEditor, { rows: [row], onChange() {}, reveal: 1, error: 'Đơn vị vật tư nhập trường khi có số lượng' });
  assert.match(html, /aria-label="Đơn vị nhập trường dòng 1" aria-invalid="true" aria-describedby="arrival-unit-error-arrival-1"/);
  assert.match(html, /id="arrival-unit-error-arrival-1" role="alert"/);
  assert.match(html, /Nhập đơn vị khi có số lượng/);
  assert.doesNotMatch(render(components.MaterialArrivalEditor, { rows: [{ ...row, quantity: '' }], onChange() {}, reveal: 0 }), /aria-invalid="true"/);
});

test('native plan and blocker row details render optional facts and exact validation in existing editors', () => {
  const plan = { row_id: 'plan-a', title: 'Lắp cửa', instruction_code: 'CD01', location: 'Tầng 2', subcontractor: 'Tổ cửa', receiver: 'An', contact: '0900', issued_date: '2026-10-08', due_date: '', progress_percent: '0', state: 'Đã giao' };
  const html = render(components.NextPlanEditor, { rows: [plan], errors: { 'next_plan-a_progress_percent': 'Sai tiến độ' }, onChange() {} });
  assert.match(html, /Chi tiết kế hoạch/);
  assert.match(html, /value="CD01"/);
  assert.match(html, /aria-label="Tiến độ \(%\) · Sẽ làm 1" aria-invalid="true"/);
  assert.match(html, /value="0"/);
  const risk = { row_id: 'risk-a', id: 'risk-a', content: 'Thiếu cửa', risk_code: 'RR01', detected_at: '', cause: 'Chậm giao', impact: '', severity: 'high', proposed_action: 'Giao bổ sung', owner: 'Lan', target_date: '', status: '' };
  const riskHTML = render(components.BlockerEditor, { rows: [risk], drafts: [], errors: {}, onDetails() {}, onChange() {}, onRelation() {}, onRemove() {}, onAdd() {}, canCreateCheck: true, canCreateAction: true, disabled: false });
  assert.match(riskHTML, /Chi tiết vấn đề/);
  assert.match(riskHTML, /Chậm giao/);
  assert.match(riskHTML, /value="Lan"/);
  assert.match(riskHTML, /Tạo kiểm tra/);
  assert.match(riskHTML, /Giao xử lý/);
});

test('native plan editor removes by stable row identity while retaining facts on surviving duplicate titles', () => {
  const rows = [{ row_id: 'plan-a', title: 'Lắp cửa', receiver: 'An' }, { row_id: 'plan-b', title: 'Lắp cửa', receiver: 'Bình' }].map(row => ({ instruction_code: '', location: '', subcontractor: '', contact: '', issued_date: '', due_date: '', progress_percent: '', state: '', ...row }));
  let next: any[] = [];
  const tree = components.NextPlanEditor({ rows, errors: {}, onChange(value: any[]) { next = value; } });
  function find(node: any): any {
    if (!node || typeof node !== 'object') return;
    if (node.props?.['aria-label'] === 'Xóa Sẽ làm 1') return node;
    for (const child of [node.props?.children].flat(Infinity)) { const found = find(child); if (found) return found; }
  }
  find(tree).props.onClick();
  assert.deepEqual(next.map(row => row.row_id), ['plan-b']);
  assert.equal(next[0].receiver, 'Bình');
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

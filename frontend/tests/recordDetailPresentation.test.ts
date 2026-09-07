import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

// Exercise the real read-only components with fixed fixtures; no browser, HTTP or DB.
const bundle = await build({
  stdin: {
    contents: `export { BOQDetail } from './modules/boq/BOQPage';
      export { ProcurementDetail } from './modules/procurement/ProcurementPage';
      export { RecordDetailFields, RecordDetailDialog } from './components/RecordDetailDialog';
      export { TopicRecordSummary } from './modules/work-items/ActivityTopicDetails';
      export { RecordLink, RelatedRecords } from './modules/_shared/RecordLinks';`,
    resolveDir: fileURLToPath(new URL('../src/', import.meta.url)),
  },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}' },
  plugins: [{ name: 'read-only-auth', setup(builder) {
    builder.onResolve({ filter: /\/auth\/AuthContext$/ }, () => ({ path: 'auth', namespace: 'detail-test' }));
    builder.onLoad({ filter: /.*/, namespace: 'detail-test' }, () => ({ contents: 'export const useAuth = () => ({ user: { id: "reader" }, hasPermission: key => (globalThis.recordLinkTestPermissions || []).includes(key), hasAnyPermission: keys => keys.some(key => (globalThis.recordLinkTestPermissions || []).includes(key)) });', loader: 'js' }));
  } }],
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(createRequire(import.meta.url), module, module.exports);
const { BOQDetail, ProcurementDetail, RecordDetailFields, RecordDetailDialog } = module.exports;

test('stock source quantities format PostgreSQL decimals in Vietnamese without multiplying their apparent value', () => {
  const props = { topic: { scope: 'stock_movement', label: 'Vật tư' }, record: { quantity: '800.000', unit: 'm', material_name: 'Dây điện' } };
  assert.match(render(module.exports.TopicRecordSummary, props), />800 m</);
  assert.doesNotMatch(render(module.exports.TopicRecordSummary, props), /800\.000 m/);
  assert.match(render(module.exports.TopicRecordSummary, { ...props, record: { ...props.record, quantity: '1234.500' } }), /1\.234,5 m/);
});
const render = (component: any, props: object) => renderToStaticMarkup(createElement(component, props));
const renderWithPermissions = (component: any, props: object, permissions: string[]) => {
  (globalThis as any).recordLinkTestPermissions = permissions;
  try { return render(component, props); } finally { delete (globalThis as any).recordLinkTestPermissions; }
};

test('BOQ preview preserves numeric values and all tabs without enabling price actions', () => {
  const item = Object.freeze({ id: 'boq', work_item_name: 'Sơn nội thất', category: 'Hoàn thiện', quantity: '16.000', unit: 'thùng', total_amount: 12800000, status: 'approved', description: '<script>untrusted</script>', norm_price_locked: true, final_price_locked: false });
  const base = { item, tab: 'overview', onTabChange() {}, projectId: 'project', canViewProcurement: false };
  const html = render(BOQDetail, base);
  assert.match(html, /16 thùng/);
  assert.doesNotMatch(html, /16\.000 thùng/);
  assert.match(html, /12\.800\.000/);
  for (const label of ['Tổng quan', 'Đơn giá &amp; lịch sử', 'Báo giá', 'Mua sắm']) assert.ok(html.includes(label));
  assert.match(html, /&lt;script&gt;untrusted/);
  assert.doesNotMatch(html, /<script>/);
  assert.doesNotMatch(render(BOQDetail, { ...base, tab: 'prices' }), /Mở quản lý đơn giá/);
  assert.match(render(BOQDetail, { ...base, tab: 'prices', onOpenPrices() {} }), /Mở quản lý đơn giá/);
  assert.match(render(BOQDetail, { ...base, tab: 'quotes', item: { ...item, quoted_unit_price: 0 } }), /0\s*₫/);
});

test('procurement detail keeps requested, accepted, stocked and awaiting quantities distinct', () => {
  const line = Object.freeze({ id: 'line', material_name: 'Gạch lát', unit: 'm²', quantity_requested: 100, accepted_quantity: 60, stock_received_quantity: 20, awaiting_stock_quantity: 40, delivery_status: 'pending', price_waiver_reason: '<b>Căn cứ</b>' });
  const record = Object.freeze({ request_code: 'PR-01', title: 'Mua gạch', status: 'ordered', total_estimated_amount: 45000000, items: [line] });
  const html = render(ProcurementDetail, { record });
  for (const quantity of [100, 60, 20, 40]) assert.ok(html.includes(`${quantity} m²`));
  for (const label of ['Đạt nghiệm thu', 'Đã nhập kho', 'Chờ nhập kho', 'Chờ tiếp nhận']) assert.ok(html.includes(label));
  assert.doesNotMatch(html, />pending</);
  assert.match(html, /&lt;b&gt;Căn cứ&lt;\/b&gt;/);
  assert.doesNotMatch(html, /Gửi duyệt|Tạo phiếu nhập|onClick/);
  assert.match(render(ProcurementDetail, { record: { ...record, items: [] } }), /Yêu cầu chưa có dòng hàng/);
});

test('record fields retain false and zero values, apply hidden fields, and escape free text', () => {
  const html = render(RecordDetailFields, {
    record: { zero: 0, boolean: false, text: '<img src=x>', secret: 'hidden-value' },
    fields: [{ key: 'zero', label: 'Số lượng' }, { key: 'boolean', label: 'Đã xác nhận' }, { key: 'text', label: 'Nội dung', wide: true }, { key: 'secret', label: 'Ẩn', hidden: () => true }],
  });
  assert.match(html, />0</);
  assert.match(html, />Không</);
  assert.match(html, /&lt;img src=x&gt;/);
  assert.doesNotMatch(html, /hidden-value|<img/);
});

test('audit-style technical details stay available inside a closed disclosure', () => {
  const html = render(RecordDetailDialog, {
    open: true, title: 'Sự kiện', onClose() {}, record: { description: 'Đã sửa', id: 'audit-id', metadata_json: { before: 10, after: 20 } },
    fields: [{ key: 'description', label: 'Nội dung' }, { key: 'id', label: 'ID' }, { key: 'metadata_json', label: 'Dữ liệu' }],
    sections: [{ title: 'Nội dung', keys: ['description'] }, { title: 'Thông tin đối chiếu', keys: ['id', 'metadata_json'], collapsed: true }],
  });
  assert.match(html, /Đã sửa/);
  assert.match(html, /audit-id/);
  assert.match(html, /before/);
  assert.match(html, /after/);
  assert.match(html, /<details[^>]*>/);
  assert.doesNotMatch(html, /<details[^>]*\bopen/);
});

test('real source anchors preserve the original tab and disappear when access is unavailable', () => {
  const target = { kind: 'procurement_request', id: '11111111-1111-4111-8111-111111111111', projectId: '22222222-2222-4222-8222-222222222222' };
  const props = { target, children: 'Yêu cầu <gốc>' };
  const html = renderWithPermissions(module.exports.RecordLink, props, ['project.view', 'procurement.view']);
  assert.match(html, /href="\/projects\/22222222-2222-4222-8222-222222222222\/procurement\?record=11111111/);
  assert.match(html, /target="_blank"/);
  assert.match(html, /rel="noopener noreferrer"/);
  assert.match(html, /Yêu cầu &lt;gốc&gt;/);
  assert.doesNotMatch(render(module.exports.RecordLink, props), /<a/);
  assert.equal(render(module.exports.RecordLink, { ...props, primary: true }), '');
});

test('BOQ procurement and quotation tabs link exact source IDs rather than their list page', () => {
  const id = '11111111-1111-4111-8111-111111111111';
  const projectId = '22222222-2222-4222-8222-222222222222';
  const permissions = ['project.view', 'boq.view', 'procurement.view', 'catalog.view'];
  const base = { item: { work_item_name: 'Sơn', quantity: 16, unit: 'thùng', procurement_request_id: id, procurement_request_code: 'PR-01', selected_quotation_id: id, selected_quotation_code: 'BG-01' }, projectId, canViewProcurement: true, onTabChange() {} };
  assert.match(renderWithPermissions(BOQDetail, { ...base, tab: 'procurement' }, permissions), /record=11111111-1111-4111-8111-111111111111&amp;record_type=procurement_request/);
  assert.match(renderWithPermissions(BOQDetail, { ...base, tab: 'quotes' }, permissions), /record=11111111-1111-4111-8111-111111111111&amp;record_type=supplier_quotation/);
});

test('procurement links BOQ and parent quotation while keeping its PO as plain text', () => {
  const id = '11111111-1111-4111-8111-111111111111';
  const projectId = '22222222-2222-4222-8222-222222222222';
  const record = { id, project_id: projectId, order_reference: 'PO-01', items: [{ id, boq_item_id: id, quotation_id: id, quotation_code: 'BG-01', material_name: 'Gạch' }] };
  const html = renderWithPermissions(ProcurementDetail, { record }, ['project.view', 'boq.view', 'procurement.view']);
  assert.match(html, /record_type=boq_item/);
  assert.match(html, /record_type=supplier_quotation/);
  assert.doesNotMatch(html, /href="[^"]*PO-01/);
});

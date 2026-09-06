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
      export { RecordDetailFields, RecordDetailDialog } from './components/RecordDetailDialog';`,
    resolveDir: fileURLToPath(new URL('../src/', import.meta.url)),
  },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}' },
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(createRequire(import.meta.url), module, module.exports);
const { BOQDetail, ProcurementDetail, RecordDetailFields, RecordDetailDialog } = module.exports;
const render = (component: any, props: object) => renderToStaticMarkup(createElement(component, props));

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

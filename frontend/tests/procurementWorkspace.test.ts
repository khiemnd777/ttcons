import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: `export * from './modules/procurement/procurementWorkspace'; export { ProcurementApprovalPanel } from './modules/procurement/ProcurementDetail'; export { ProcurementSnapshot } from './modules/procurement/ProcurementSnapshot'; export { discussionWorkSurface } from './modules/work-items/discussionWorkDomain';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}' },
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(createRequire(import.meta.url), module, module.exports);
const { procurementActionFor, procurementHeader, procurementMutationWorkItemId, procurementTaskVerificationQueryOptions, validateProcurementHeader, ProcurementApprovalPanel, ProcurementSnapshot, discussionWorkSurface } = module.exports;

test('the next action requires both source state and its explicit server capability', () => {
  const capabilities = { edit: true, submit: true, approve: true, order: true };
  for (const [status, expected] of [['draft', 'submit'], ['changes_requested', 'submit'], ['waiting_approval', 'approve'], ['submitted', 'approve'], ['approved', 'order'], ['ordered', null], ['received', null], ['cancelled', null]]) {
    assert.equal(procurementActionFor({ status, capabilities }), expected);
    assert.equal(procurementActionFor({ status, capabilities }, true), null);
    assert.equal(procurementActionFor({ status }), null);
    assert.equal(procurementActionFor({ status, capabilities: {} }), null);
  }
  assert.equal(procurementActionFor({ status: 'approved', capabilities: { order: true, edit: false } }), 'order');
  assert.equal(procurementActionFor({ status: 'approved', can_act: true }), null);
});

test('header drafts carry only editable fields and ordering highlights required information', () => {
  const header = procurementHeader({ title: 'Tiles', supplier_partner_id: null, order_reference: null, expected_delivery_date: null, status: 'approved', total_estimated_amount: 100, approval_request_id: 'approval' });
  assert.deepEqual(header, { title: 'Tiles', supplier_partner_id: '', order_reference: '', expected_delivery_date: '' });
  assert.deepEqual(validateProcurementHeader(header), {});
  assert.deepEqual(Object.keys(validateProcurementHeader(header, 'order')).sort(), ['expected_delivery_date', 'order_reference', 'supplier_partner_id']);
  const ordered = { ...header, supplier_partner_id: 'supplier', order_reference: 'PO-1', expected_delivery_date: '2026-10-01' };
  assert.deepEqual(validateProcurementHeader(ordered, 'order'), {});
  assert.ok(validateProcurementHeader({ ...ordered, expected_delivery_date: '2026-02-31' }).expected_delivery_date);
  assert.ok(validateProcurementHeader({ ...ordered, order_reference: '  ' }, 'order').order_reference);
});

test('only supported procurement tasks open the editor and explicitly start matching source work', () => {
  const item = { id: 'work', execution_target: { workspace: 'procurement' } };
  for (const key of ['procurement.require_approved', 'procurement.generate_from_boq']) assert.equal(discussionWorkSurface({ ...item, domain_action_key: key }), 'procurement');
  for (const key of ['source.supplier_quotation_ready', 'source.supplier_selection_ready', 'warehouse.receive']) assert.equal(discussionWorkSurface({ ...item, domain_action_key: key }), 'external');
  assert.equal(procurementMutationWorkItemId({ id: 'work', domain_action_key: 'procurement.require_approved' }), 'work');
  assert.equal(procurementMutationWorkItemId({ id: 'work', domain_action_key: 'procurement.generate_from_boq' }), '');
  assert.equal(procurementMutationWorkItemId(), '');
});

test('procurement task verification always refetches a warm shared Work Item cache', () => {
  assert.deepEqual(procurementTaskVerificationQueryOptions, {
    retry: false,
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
  });
});

test('approval context names every eligible approver and explains the current actor capability', () => {
  const context = {
    permission: 'procurement.approve',
    candidate_count: 2,
    candidates: [
      { user_id: 'procurement', full_name: 'Ngô Lan Mua hàng', role_names: ['Mua hàng'] },
      { user_id: 'pm', full_name: 'Nguyễn Minh PM', role_names: ['PM'] },
    ],
  };
  const allowed = renderToStaticMarkup(createElement(ProcurementApprovalPanel, { context, canApprove: true }));
  for (const text of ['Người có thể duyệt', '2 người', 'Ngô Lan Mua hàng', 'Vai trò: Mua hàng', 'Nguyễn Minh PM', 'Bạn có quyền duyệt yêu cầu này.']) assert.ok(allowed.includes(text), text);
  assert.doesNotMatch(allowed, /Chờ người có quyền/);

  const denied = renderToStaticMarkup(createElement(ProcurementApprovalPanel, { context, canApprove: false }));
  assert.match(denied, /Bạn không có quyền duyệt yêu cầu này\./);

  const empty = renderToStaticMarkup(createElement(ProcurementApprovalPanel, { context: { ...context, candidate_count: 0, candidates: [] }, canApprove: false }));
  assert.match(empty, /Chưa có người đủ quyền duyệt yêu cầu này\./);
});

test('purchase snapshots render persisted prices and PO without current source actions or stock claims', () => {
  const snapshot = Object.freeze({ request_code: 'PR-01', status: 'approved', supplier_name: '<script>supplier</script>', order_reference: 'Old PO', expected_delivery_date: '2026-10-01', total_estimated_amount: 100000, items: [{ id: 'line', material_name: 'Old tile', quantity_requested: 2, unit: 'm2', estimated_unit_price: 50000 }] });
  const html = renderToStaticMarkup(createElement(ProcurementSnapshot, { snapshot }));
  for (const text of ['PR-01', 'Chờ đặt hàng', 'Old PO', 'Old tile', '100.000', '50.000', '01-10-2026']) assert.ok(html.includes(text), text);
  assert.match(html, /&lt;script&gt;supplier&lt;\/script&gt;/);
  assert.doesNotMatch(html, /Ghi nhận đặt hàng|Lưu thông tin|Đã nhập kho|<script>/);
});

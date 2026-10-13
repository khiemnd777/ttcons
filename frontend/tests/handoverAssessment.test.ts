import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { canAssessHandover, checklistAssessmentPayload, handoverAssessmentPayload, handoverDueDateIssue } from '../src/modules/handover/handoverAssessment.ts';

const require = createRequire(import.meta.url);
const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const { MemoryRouter } = require('react-router-dom');
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: "export {HandoverWorkspace} from './modules/handover/HandoverWorkspace'; export {HandoverAssessmentDialog} from './modules/handover/HandoverAssessmentDialog';" }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}' }, plugins: [{ name: 'toast', setup(b) { b.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast(){return {success(){},warn(){},error(){}}}', loader: 'js' })); } }] });
const module = { exports: {} as any };
// Use the existing controlled hook pattern to select the checklist tab without a browser.
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === 'react' ? { ...React, useState(initial: any) { return [initial === 'tasks' ? 'checklist' : typeof initial === 'function' ? initial() : initial, () => {}]; }, useEffect() {}, useMemo(fn: any) { return fn(); }, useRef() { return { current: null }; } } : require(name), module, module.exports);
const items = [
  { id: 'auto', item_code: 'final_acceptance', category: 'quality', title: 'Nghiệm thu cuối', required: true, status: 'verified', source_type: 'acceptance', physical_result: 'fail', requirement: 'Không thấm', defect: 'Thấm tường', due_date: '2026-10-12' },
  { id: 'linked', item_code: 'operation_manuals', category: 'other', title: 'Sổ vận hành', required: true, status: 'pending', document_id: 'doc' },
];
const detail = { id: 'ho', project_id: 'p', handover_code: 'HO', status: 'draft', client_representative: 'CĐT', company_representative: 'NT', handover_stage: 'Khu A', handover_conclusion: 'not_eligible', checklist_items: items, evidence_options: [], document_options: [] };
const noop = () => {};
function workspace(status = 'draft', canVerify = true) {
  return module.exports.HandoverWorkspace({ detail: { ...detail, status }, canVerify, canApprove: false, pending: false, onChecklist: noop, onViewSource: noop, onSubmit: noop, onCheck: noop, onComplete: noop, onAssessment: noop, onHeaderAssessment: noop, onAddItem: noop });
}
function html(tree: any) { return renderToStaticMarkup(React.createElement(MemoryRouter, null, tree)); }

test('observation payload explicitly excludes verification, source, evidence and document writes', () => {
  assert.deepEqual(checklistAssessmentPayload({ requirement: ' Yêu cầu ', physical_result: 'fail', defect: ' Lỗi ', due_date: '2026-10-12', status: 'verified', evidence_id: 'e', document_id: 'd', source_id: 's', required: true }), { requirement: 'Yêu cầu', physical_result: 'fail', defect: 'Lỗi', due_date: '2026-10-12' });
  assert.deepEqual(checklistAssessmentPayload({}), { requirement: '', physical_result: 'not_assessed', defect: '', due_date: '' });
  assert.deepEqual(handoverAssessmentPayload({ handover_stage: '', handover_conclusion: '', status: 'handed_over' }), { handover_stage: '', handover_conclusion: '' });
});

test('assessment permission gates allow every nonterminal dossier and reject terminal states', () => {
  for (const status of ['draft', 'submitted', 'ready', 'blocked']) assert.equal(canAssessHandover(status, true), true);
  for (const status of ['handed_over', 'cancelled', 'deleted']) assert.equal(canAssessHandover(status, true), false);
  assert.equal(canAssessHandover('draft', false), false);
});

test('due date allows clearing and leap day but rejects impossible calendar dates', () => {
  for (const value of ['', '2024-02-29', '2026-10-12']) assert.equal(handoverDueDateIssue(value), undefined);
  for (const value of ['2026-02-29', '2026-02-30', '2026-13-01', '2026-1-1', 'invalid']) assert.ok(handoverDueDateIssue(value));
});

test('automatic and linked rows render assessment plus existing source action on desktop and mobile', () => {
  const output = html(workspace());
  assert.equal((output.match(/>Đánh giá<\/button>/g) ?? []).length, 4);
  assert.equal((output.match(/Xem nguồn<\/button>/g) ?? []).length, 4);
  for (const label of ['Cập nhật đánh giá', 'Thêm hạng mục bàn giao', 'Khu A', 'Chưa đủ điều kiện bàn giao', 'Kiểm tra thực tế:', 'Không đạt', 'Thấm tường', 'Không thấm', 'Hạn xử lý:']) assert.ok(output.includes(label), label);
  assert.ok(output.indexOf('Giai đoạn bàn giao:') < output.indexOf('Thêm hạng mục bàn giao'));
});

test('locked and permissionless workspace keeps observations readable and hides all assessment writes', () => {
  for (const [status, permitted] of [['handed_over', true], ['cancelled', true], ['deleted', true], ['draft', false]] as const) {
    const output = html(workspace(status, permitted));
    assert.doesNotMatch(output, /Cập nhật đánh giá|Thêm hạng mục bàn giao|>Đánh giá<\/button>/);
    assert.match(output, /Thấm tường/);
    assert.match(output, /Xem nguồn/);
  }
});

test('assessment editor initializes saved date and restricts fields to physical observations', () => {
  const tree = module.exports.HandoverAssessmentDialog({ action: { type: 'assessment', item: { ...items[0], due_date: '2026-10-12T00:00:00Z' } }, detail, pending: false, onClose: noop, onSave: noop });
  const form = tree.props.children[1];
  assert.deepEqual(form.props.fields.map((field: any) => field.name), ['requirement', 'physical_result', 'defect', 'due_date']);
  assert.equal(form.props.fields.find((field: any) => field.name === 'due_date').defaultValue, '2026-10-12');
  const header = module.exports.HandoverAssessmentDialog({ action: { type: 'header-assessment' }, detail: { ...detail, handover_conclusion: '' }, pending: false, onClose: noop, onSave: noop });
  assert.equal(header.props.children[1].props.fields[1].placeholder, 'Chưa kết luận');
  assert.equal(header.props.children[1].props.fields[1].defaultValue, '');
  assert.equal(header.props.children[1].props.fields[0].validate(`  ${'😀'.repeat(160)}  `), undefined);
  assert.ok(header.props.children[1].props.fields[0].validate('😀'.repeat(161)));
  const add = module.exports.HandoverAssessmentDialog({ action: { type: 'add-item' }, detail, pending: false, onClose: noop, onSave: noop });
  assert.equal(add.props.children[1].props.fields[0].name, 'title');
  assert.equal(add.props.children[1].props.fields[0].required, true);
  assert.ok(add.props.children[1].props.fields[0].validate('x'.repeat(301)));
});

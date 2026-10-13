import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { emptyNextPlan, emptyBlockerCapture } from '../src/modules/operations/constructionDailyCapture.ts';

const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: "export {ConstructionReportFormDialog} from './modules/operations/ConstructionReportFormDialog';export {ConstructionReportDetailDialog} from './modules/operations/ConstructionReportDetailDialog';" }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///non-browser-test"' }, plugins: [{ name: 'non-browser-form-context', setup(builder) {
  builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast(){return {success(){},warn(){},error(){}}}', loader: 'js' }));
  builder.onLoad({ filter: /useWorkReportRecipients\.ts$/ }, () => ({ contents: 'export function useWorkReportRecipients(){return {data:[],isLoading:false}}', loader: 'js' }));
  builder.onLoad({ filter: /formValidationNavigation\.ts$/ }, () => ({ contents: 'export function useValidationNavigation(){return options=>options?.prepare?.()}', loader: 'js' }));
  builder.onLoad({ filter: /EvidenceGallery\.tsx$/ }, () => ({ contents: 'export function useEvidencePreviewUrls(){return {urls:{},loadingIds:new Set(),errorIds:new Set()}};export function EvidenceGallery(){return null}', loader: 'js' }));
  builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export function useAuth(){return {hasPermission:()=>true}}', loader: 'js' }));
} }] });
function find(node: any, predicate: (node: any) => boolean): any {
  if (!node || typeof node !== 'object') return;
  if (predicate(node)) return node;
  for (const child of react.Children.toArray(node.props?.children)) { const found = find(child, predicate); if (found) return found; }
}
function form(nextPlans: any[], blockers: any[], mode: 'create' | 'edit' = 'create') {
  const values: any[] = [];
  const fixed: Record<number, any> = { 3: 'work-item', 4: ['Đã làm'], 6: nextPlans, 7: blockers, 17: [{ id: 'pending-photo', evidenceType: 'photo' }] };
  let cursor = 0;
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === 'react' ? { ...react, useState(initial: any) { const index = cursor++; values[index] = index in fixed ? fixed[index] : typeof initial === 'function' ? initial() : initial; return [values[index], (value: any) => { values[index] = typeof value === 'function' ? value(values[index]) : value; }]; }, useEffect() {}, useMemo(fn: any) { return fn(); }, useCallback(fn: any) { return fn; }, useRef() { return { current: null }; } } : name === '@tanstack/react-query' ? { ...require(name), useQuery() { return { data: { work_items: [], items: [] }, isSuccess: true, isLoading: false, isError: false }; } } : require(name), module, module.exports);
  const calls: any[] = [];
  const tree = module.exports.ConstructionReportFormDialog({ open: true, projectId: 'project', mode, onClose() {}, async onSubmit(...args: any[]) { calls.push(args); } });
  return { values, calls, submit: find(tree, node => node.type === 'form').props.onSubmit };
}
test('invalid native row details block parent save/upload and expose exact inline errors', async () => {
  const plan = { ...emptyNextPlan('Lắp cửa'), progress_percent: '101' };
  const risk = { ...emptyBlockerCapture('Thiếu cửa'), detected_at: '2026-02-30', id: '' };
  risk.id = risk.row_id;
  const fixture = form([plan], [risk]);
  await fixture.submit({ preventDefault() {}, nativeEvent: {} });
  assert.equal(fixture.calls.length, 0);
  assert.ok(fixture.values[19][`next_${plan.row_id}_progress_percent`]);
  assert.ok(fixture.values[19][`blocker_${risk.row_id}_detected_at`]);
  assert.equal(fixture.values[20], 'next');
});
test('create and edit submit existing canonical rows once with explicit clears and zero before parent upload', async () => {
  for (const mode of ['create', 'edit'] as const) {
    const plan = { ...emptyNextPlan('Lắp cửa'), receiver: '', progress_percent: '0' };
    const risk = { ...emptyBlockerCapture('Thiếu cửa'), owner: '', cause: 'Chậm giao' };
    const fixture = form([plan], [{ ...risk, id: risk.row_id }], mode);
    await fixture.submit({ preventDefault() {}, nativeEvent: {} });
    assert.equal(fixture.calls.length, 1);
    const [payload, uploads] = fixture.calls[0];
    assert.equal(payload.next_items[0].row_id, plan.row_id);
    assert.equal(payload.next_items[0].progress_percent, 0);
    assert.equal(payload.next_items[0].receiver, '');
    assert.equal(payload.blockers[0].row_id, risk.row_id);
    assert.equal(payload.blockers[0].owner, '');
    assert.equal(payload.blockers[0].cause, 'Chậm giao');
    assert.equal(uploads[0].id, 'pending-photo');
  }
});

function currentDetail(record: Record<string, any>) {
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === 'react' ? { ...react, useState(initial: any) { return [typeof initial === 'function' ? initial() : initial, () => {}]; }, useEffect() {}, useMemo(fn: any) { return fn(); }, useCallback(fn: any) { return fn; }, useRef() { return { current: null }; } } : name === '@tanstack/react-query' ? { ...require(name), useQueryClient() { return {}; }, useQuery() { return { data: { work_items: [], items: [] }, isSuccess: true }; } } : name === 'react-router-dom' ? { ...require(name), useLocation() { return { pathname: '/projects/project/operations' }; }, useNavigate() { return () => {}; } } : require(name), module, module.exports);
  return module.exports.ConstructionReportDetailDialog({ open: true, projectId: 'project', record, onClose() {} });
}
test('current daily native exports use exact saved report identity and hide absent source rows', () => {
  const record = { id: 'daily-exact', structured_json: { next_plan_items: [{ title: 'Lắp cửa', receiver: 'An', progress_percent: 0 }], blockers: [{ content: 'Thiếu cửa', owner: 'Lan' }] } };
  const tree = currentDetail(record);
  assert.equal(find(tree, node => node.props?.label === 'Xuất báo cáo rủi ro').props.path, '/daily-reports/daily-exact/risks/export.xlsx');
  assert.equal(find(tree, node => node.props?.label === 'Xuất báo cáo hiện trường').props.path, '/daily-reports/daily-exact/next-plan/export.xlsx');
  const empty = currentDetail({ id: 'daily-empty' });
  assert.equal(find(empty, node => node.props?.label === 'Xuất báo cáo rủi ro'), undefined);
  assert.equal(find(empty, node => node.props?.label === 'Xuất báo cáo hiện trường'), undefined);
});

test('current workflow progress uses captured authority and never displays canonical fallback zero or another value', () => {
  const { renderToStaticMarkup } = require('react-dom/server');
  for (const captured of [undefined, null, '', ' ', { value: null }, 'N/A', '0x10', true, false, -1, 101, { value: 101 }]) {
    const html = renderToStaticMarkup(currentDetail({ id: 'daily', workflow_form_instance_id: 'form', progress_percent: 88, structured_json: { progress_percent: captured } }));
    assert.ok(html.includes('Chưa ghi tiến độ'));
    assert.ok(!html.includes('role="progressbar"'));
    assert.ok(!html.includes('88%'));
  }
  for (const captured of [0, '0', 35, '35', { value: 35 }]) {
    const expected = typeof captured === 'object' ? 35 : captured;
    const html = renderToStaticMarkup(currentDetail({ id: 'daily', workflow_form_instance_id: 'form', progress_percent: 88, structured_json: { progress_percent: captured } }));
    assert.ok(html.includes(`aria-valuenow="${expected}"`));
    assert.ok(html.includes(`${expected}%`));
    assert.ok(!html.includes('88%'));
  }
  const manual = renderToStaticMarkup(currentDetail({ id: 'manual', progress_percent: 0, structured_json: { progress_percent: 88 } }));
  assert.ok(manual.includes('aria-valuenow="0"'));
  const manualClamped = renderToStaticMarkup(currentDetail({ id: 'manual', progress_percent: 101 }));
  assert.ok(manualClamped.includes('aria-valuenow="100"'));
});

test('current native next-plan progress omits invalid legacy values and preserves recorded zero', () => {
  const { renderToStaticMarkup } = require('react-dom/server');
  for (const captured of [null, '', 'N/A', true, false, -1, 101]) {
    const html = renderToStaticMarkup(currentDetail({ id: 'daily', progress_percent: 0, structured_json: { next_plan_items: [{ title: 'Lắp cửa', progress_percent: captured }] } }));
    assert.ok(!html.includes('Tiến độ: '));
    assert.ok(!html.includes('N/A%'));
    assert.ok(!html.includes('false%'));
  }
  for (const captured of [0, '0', 40, '40']) {
    const html = renderToStaticMarkup(currentDetail({ id: 'daily', progress_percent: 0, structured_json: { next_plan_items: [{ title: 'Lắp cửa', progress_percent: captured }] } }));
    assert.ok(html.includes(`Tiến độ: ${captured}%`));
  }
});

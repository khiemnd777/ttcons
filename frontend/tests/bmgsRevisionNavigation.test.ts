import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
const require = createRequire(import.meta.url);
const react = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const bundle = await build({stdin: {resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: "export {WorkReportReviewDialog} from './modules/work-reports/WorkReportReviewDialog';"}, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: {'.css': 'empty'}, define: {'import.meta.env': '{}'}, plugins: [{name: 'contexts', setup(b) {
  b.onLoad({filter: /ToastProvider\.tsx$/}, () => ({contents: 'export function useToast(){return {success(){},warn(){},error(){}}}', loader: 'js'}));
  b.onLoad({filter: /EvidenceGallery\.tsx$/}, () => ({contents: 'export function EvidenceGallery(){return null}', loader: 'js'}));
}}]});
function find(n: any, predicate: (n: any) => boolean): any {
  if (!n || typeof n !== 'object') return;
  if (predicate(n)) return n;
  for (const child of react.Children.toArray(n.props?.children)) {const found = find(child, predicate); if (found) return found;}
}
const m = {exports: {} as any};
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === 'react' ? {...react, useState(initial: any) {return [typeof initial === 'function' ? initial() : initial, () => {}];}, useEffect() {}, useMemo(fn: any) {return fn();}, useCallback(fn: any) {return fn;}, useRef() {return {current: null};}} : name === 'react-router-dom' ? {...require(name), useNavigate: () => () => {}} : require(name), m, m.exports);
const evidence = {id: 'exact-historical-evidence', name: 'Ảnh cũ'};
const revisions = [{revision_no: 1, snapshot_json: {structured_json: {done_items: [{title: 'Nội dung cũ'}]}}, evidence_records: [evidence]}, {revision_no: 2, snapshot_json: {structured_json: {done_items: [{title: 'Nội dung mới'}]}}, evidence_records: [{id: 'current-only-evidence'}]}];
const detail = {source_id: 'source', work_report_id: 'report', project_id: 'project', current_revision_no: 2, status: 'submitted', capabilities: {can_acknowledge: true, can_request_changes: true, can_discuss: true}, revisions, read_receipts: [], events: []};
const render = (initialRevisionNo: number | null | undefined, extra: any = {}) => m.exports.WorkReportReviewDialog({open: true, detail, loading: false, error: null, initialRevisionNo, userID: 'actor', canReview: true, canManage: true, canOpenSource: true, canExport: true, onRefresh: async () => {}, onClose() {}, ...extra});
test('requested historical revision selects exact snapshot and immutable evidence byte endpoint', () => {
  const tree = render(1);
  const summary = find(tree, n => n.type?.name === 'ReportSummary');
  assert.equal(summary.props.revision.revision_no, 1);
  const content = find(tree, n => n.type?.name === 'ReportContent');
  assert.deepEqual(content.props.evidence, [evidence]);
  assert.equal(content.props.filePath(evidence), '/work-reports/report/revisions/1/evidence/exact-historical-evidence/file');
  const panel = find(tree, n => n.type?.name === 'ReviewActionPanel');
  if (panel) assert.equal(panel.props.mayReview, false);
});
test('invalid or absent requested revision never shows current content as substitute', () => {
  for (const value of [null, 3]) {
    const tree = render(value);
    assert.equal(find(tree, n => n.type?.name === 'ReportContent'), undefined);
    assert.ok(find(tree, n => n.props?.role === 'alert'));
    assert.equal(tree.props.footer, undefined);
  }
});
test('ordinary report opens current revision; loading keeps historical content hidden', () => {
  assert.equal(find(render(undefined), n => n.type?.name === 'ReportSummary').props.revision.revision_no, 2);
  assert.equal(find(render(1, {loading: true}), n => n.type?.name === 'ReportContent'), undefined);
});
test('fetch failure retains retry and never exposes cached revision content', () => {
  const tree = render(3, {error: new Error('transport')});
  assert.equal(find(tree, n => n.type?.name === 'ReportContent'), undefined);
  assert.ok(find(tree, n => n.type === 'p' && n.props.children === 'transport'));
  assert.ok(find(tree, n => n.type === 'button' && typeof n.props.onClick === 'function'));
});

test('exact historical content renders saved native risk, plan, crew, arrival and assessment facts', () => {
  const historical = { revision_no: 1, snapshot_json: { structured_json: {
    next_plan_items: [{ row_id: 'plan-old', title: 'Lắp cửa cũ', receiver: 'Người nhận cũ', instruction_code: 'CD-CU', progress_percent: 0, issued_date: '2026-10-08' }],
    blockers: [{ row_id: 'risk-old', content: 'Rủi ro cũ', cause: 'Nguyên nhân cũ', owner: 'Chị Lan', severity: 'high', proposed_action: 'Biện pháp cũ' }],
    crew_items: [{ crew_name: 'Tổ cũ', representative: 'Đại diện cũ', contact: '0900' }],
    material_arrival_items: [{ material_name: 'Cát cũ', quantity: 0, unit: 'm³', origin: 'Nguồn cũ' }],
    supervision_assessments: [{ code: 'safety', assessment: 'fail', note: 'Thiếu lan can cũ' }],
  } }, evidence_records: [] };
  const tree = render(1, { detail: { ...detail, revisions: [historical, revisions[1]] } });
  const content = find(tree, n => n.type?.name === 'ReportContent');
  const html = renderToStaticMarkup(content);
  assert.equal(find(tree, n => n.type?.name === 'NativeExcelExportButton'), undefined);
  for (const fact of ['CD-CU', 'Người nhận cũ', 'Tiến độ: 0%', 'Nguyên nhân cũ', 'Chị Lan', 'Biện pháp cũ', 'Đại diện cũ', '0900', 'Cát cũ', 'Nguồn cũ', 'Số lượng: 0', 'Thiếu lan can cũ', 'Không đạt']) assert.ok(html.includes(fact), fact);
  assert.ok(!html.includes('Nội dung mới'));
});

test('historical workflow progress uses exact captured revision value without current or canonical fallback', () => {
  for (const captured of [undefined, null, '', { value: null }, 'N/A', true, false, -1, 101, { value: 101 }, 0, '0', 42, '42', { value: 42 }]) {
    const historical = { revision_no: 1, snapshot_json: { workflow_form_instance_id: 'form-old', progress_percent: 88, structured_json: { progress_percent: captured } }, evidence_records: [] };
    const tree = render(1, { detail: { ...detail, revisions: [historical, { ...revisions[1], snapshot_json: { workflow_form_instance_id: 'form-new', progress_percent: 99, structured_json: { progress_percent: 99 } } }] } });
    const summary = find(tree, node => node.type?.name === 'ReportSummary');
    const html = renderToStaticMarkup(summary);
    const recorded = typeof captured === 'object' && captured !== null ? captured.value : captured;
    if (recorded == null || recorded === '' || typeof recorded === 'boolean' || !Number.isFinite(Number(recorded)) || Number(recorded) < 0 || Number(recorded) > 100) {
      assert.ok(html.includes('Chưa ghi tiến độ'));
      assert.ok(!html.includes('role="progressbar"'));
    } else {
      const expected = typeof captured === 'object' ? captured.value : captured;
      assert.ok(html.includes(`aria-valuenow="${expected}"`));
    }
    assert.ok(!html.includes('88%'));
    assert.ok(!html.includes('99%'));
  }
});

test('historical native next-plan progress omits invalid legacy percentages without live fallback', () => {
  for (const captured of [null, '', 'N/A', true, false, -1, 101, 0, '0', 42]) {
    const historical = { revision_no: 1, snapshot_json: { structured_json: { next_plan_items: [{ title: 'Kế hoạch cũ', progress_percent: captured }] } }, evidence_records: [] };
    const tree = render(1, { detail: { ...detail, revisions: [historical, { ...revisions[1], snapshot_json: { structured_json: { next_plan_items: [{ title: 'Kế hoạch mới', progress_percent: 99 }] } } }] } });
    const html = renderToStaticMarkup(find(tree, node => node.type?.name === 'ReportContent'));
    if (captured === 0 || captured === '0' || captured === 42) assert.ok(html.includes(`Tiến độ: ${captured}%`));
    else assert.ok(!html.includes('Tiến độ: '));
    assert.ok(!html.includes('99%'));
    assert.ok(!html.includes('N/A%'));
    assert.ok(!html.includes('false%'));
  }
});

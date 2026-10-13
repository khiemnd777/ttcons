import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';

const require = createRequire(import.meta.url);
const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: "export { DailyTimeline } from './modules/operations/OperationsPage';" },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///non-browser-test"' },
  plugins: [{ name: 'timeline-test-entrypoint', setup(builder) {
    // Expose the actual private component in the test bundle only.
    builder.onLoad({ filter: /\/OperationsPage\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { DailyTimeline };', loader: 'tsx' }));
  } }],
});
const module = { exports: {} as any };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
const defaults = { loading: false, error: null, createEnabled: true, onRetry() {}, onCreate() {}, onView() {} };
const timeline = (rows: Record<string, any>[], extra: Record<string, any> = {}) => module.exports.DailyTimeline({ ...defaults, rows, ...extra });
function find(node: any, predicate: (node: any) => boolean): any {
  if (!node || typeof node !== 'object') return;
  if (predicate(node)) return node;
  for (const child of React.Children.toArray(node.props?.children)) {
    const found = find(child, predicate);
    if (found) return found;
  }
}
function progress(row: Record<string, any>) {
  const tree = timeline([row]);
  const label = find(tree, node => node.type === 'div' && React.Children.toArray(node.props.children).some((child: any) => child.props?.children === 'Tiến độ báo cáo'));
  assert.ok(label, 'the journal card retains its progress label');
  return renderToStaticMarkup(React.Children.toArray(label.props.children)[1]);
}

test('journal card shows missing workflow progress without falling back to the synchronized header', () => {
  for (const captured of [undefined, null, '', ' ', { value: null }, 'N/A', '0x10', true, false, -1, 101, { value: 101 }]) {
    for (const header of [0, 88]) {
      const row = { id: 'workflow', workflow_form_instance_id: 'form', progress_percent: header, structured_json: { progress_percent: captured } };
      const before = structuredClone(row);
      const html = progress(row);
      assert.ok(html.includes('Chưa ghi tiến độ'), `capture=${JSON.stringify(captured)}, header=${header}`);
      assert.ok(!html.includes('%'));
      assert.deepEqual(row, before, 'rendering must not change the saved report');
    }
  }
});

test('journal card preserves captured zero, valid percentages and legacy value wrappers', () => {
  for (const [captured, expected] of [[0, 0], ['0', 0], [{ value: 0 }, 0], [35, 35], ['35', 35], [' 35 ', 35], [{ value: '35' }, 35], [42.5, 42.5], [100, 100]] as const) {
    const row = { id: 'workflow', workflow_form_instance_id: 'form', progress_percent: 88, structured_json: { progress_percent: captured } };
    const before = structuredClone(row);
    assert.ok(progress(row).includes(`>${expected}%</p>`), JSON.stringify(captured));
    assert.deepEqual(row, before);
  }
});

test('manual journal reports retain canonical header authority including an actual saved zero', () => {
  for (const [header, expected] of [[0, 0], ['0', 0], [65, 65], [101, 100]] as const) {
    const row = { id: 'manual', progress_percent: header, structured_json: { progress_percent: 88 } };
    const before = structuredClone(row);
    assert.ok(progress(row).includes(`>${expected}%</p>`));
    assert.deepEqual(row, before);
  }
});

test('loading, error retry and empty create states remain available', () => {
  assert.ok(renderToStaticMarkup(timeline([], { loading: true })).includes('Đang tải nhật ký thi công...'));
  let retries = 0;
  const errorTree = timeline([], { error: new Error('transport'), onRetry() { retries++; } });
  assert.ok(renderToStaticMarkup(errorTree).includes('Không tải được nhật ký thi công.'));
  find(errorTree, node => node.type === 'button').props.onClick();
  assert.equal(retries, 1);
  const empty = timeline([], { createEnabled: false });
  assert.ok(renderToStaticMarkup(empty).includes('Chưa có báo cáo thi công'));
  assert.equal(find(empty, node => node.props?.title === 'Chưa có báo cáo thi công').props.action.props.disabled, true);
});

test('view action retains the exact saved report and linked check/action badges', () => {
  const row = { id: 'exact-report', progress_percent: 0 };
  let viewed: any;
  const tree = timeline([row], {
    onView(record: any) { viewed = record; },
    workspace: { checks: [{ source_daily_report_id: row.id }], actions: [{ source_daily_report_id: row.id, status: 'done' }] },
  });
  const html = renderToStaticMarkup(tree);
  for (const label of ['1 kiểm tra', '1 cần xử lý', 'Đã khắc phục']) assert.ok(html.includes(label), label);
  find(tree, node => node.props?.['aria-label'] === 'Xem báo cáo').props.onClick();
  assert.equal(viewed, row);
});
